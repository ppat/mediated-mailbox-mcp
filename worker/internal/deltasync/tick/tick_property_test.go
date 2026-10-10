//go:build integration

package tick_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick"
)

// The senders a generated history draws from. The first is listed by the policy, and the last has no
// domain the classifier can read.
var historySenders = []string{"alerts@bank.example", "orders@shop.example", "news@news.example", "no-domain"}

// change is one thing that happens at the provider after a tick took its cursor.
type change struct {
	// Kind is 0 for a delivery, 1 for a label and the read flag added to a message, 2 for a removal.
	Kind int
	// Target picks the message a label or a removal names among those the mailbox holds.
	Target int
	// Sender indexes historySenders, and Code puts a one-time code in a delivery's subject.
	Sender int
	Code   bool
}

// history is what the provider holds when the first tick takes its cursor, and what changes after.
type history struct {
	Initial int
	Changes []change
}

// drawHistory draws a history. Each change is drawn without seeing the ones before it, by rapid's
// collection generator, so rapid reduces a history by dropping changes (ADR-0069).
func drawHistory(t *rapid.T) history {
	c := rapid.Custom(func(t *rapid.T) change {
		return change{
			Kind:   rapid.IntRange(0, 2).Draw(t, "kind"),
			Target: rapid.IntRange(0, 7).Draw(t, "target"),
			Sender: rapid.IntRange(0, len(historySenders)-1).Draw(t, "sender"),
			Code:   rapid.Bool().Draw(t, "code"),
		}
	})
	return history{Initial: rapid.IntRange(0, 3).Draw(t, "initial"), Changes: rapid.SliceOfN(c, 0, 8).Draw(t, "changes")}
}

// play builds the provider's mailbox for the account and plays the history's changes after the first
// tick, returning the fake.
func play(t rapid.TB, account string, h history, first func(f *fake.Fake)) *fake.Fake {
	var initial []fake.Message
	for i := range h.Initial {
		initial = append(initial, message(fmt.Sprintf("i%d", i), historySenders[i%len(historySenders)], now.Add(-day), mail.Inbox))
	}
	f := mailbox(t, account, initial...)
	first(f)
	held := func() []string {
		page, err := f.EnumerateAll(context.Background(), "")
		if err != nil {
			t.Fatalf("listing the mailbox: %v", err)
		}
		var ids []string
		for {
			for _, m := range page.Items {
				ids = append(ids, m.ID)
			}
			if page.Next == "" {
				return ids
			}
			if page, err = f.EnumerateAll(context.Background(), page.Next); err != nil {
				t.Fatalf("listing the mailbox: %v", err)
			}
		}
	}
	for i, c := range h.Changes {
		ids := held()
		switch {
		case c.Kind == 0 || len(ids) == 0:
			m := message(fmt.Sprintf("d%d", i), historySenders[c.Sender], now)
			if c.Code {
				m.Metadata.Subject += " Your code is 419283"
			}
			deliver(t, f, m)
		case c.Kind == 1:
			label(t, f, ids[c.Target%len(ids)], "Filed")
		default:
			if err := f.Remove(ids[c.Target%len(ids)]); err != nil {
				t.Fatalf("removing: %v", err)
			}
		}
	}
	return f
}

// dump reads everything a tick writes to the index for the account, each row whole as JSON, the
// masking events counted per message and rule, and the senders' statistics, so two indexes compare
// equal only when every column of every row does.
func dump(t rapid.TB, conn *pgx.Conn, account string) map[string][]string {
	out := map[string][]string{}
	for name, sql := range map[string]string{
		"messages": `SELECT (to_jsonb(m) - 'account_id')::text FROM messages AS m WHERE m.account_id = $1 ORDER BY m.message_id`,
		"masks":    `SELECT message_id || ' ' || rule_id || ' ' || count(*) FROM masking_events WHERE account_id = $1 GROUP BY message_id, rule_id ORDER BY 1`,
		"senders":  `SELECT (to_jsonb(s) - 'account_id')::text FROM senders AS s WHERE s.account_id = $1 ORDER BY s.domain`,
	} {
		rows, err := conn.Query(context.Background(), sql, account)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		values, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		out[name] = values
	}
	return out
}

// VERIFICATIONS' row for applying the same changes twice. A rule ADR-0018 states, that a tick is
// idempotent. For a generated history of deliveries, label changes and removals after the cursor,
// one tick applies the changes. The cursor is then put back where it was and a second tick applies
// the same changes again, which leaves every row of the index, every masking event and every sender's
// statistics exactly as the first left them (ADR-0018, ADR-0055).
func TestApplyingTheSameChangesTwiceLeavesTheIndexAsOnce(t *testing.T) {
	conn := superuser(t)
	pool := syncPool(t)
	pol := listing(t, "bank.example")
	property.Check(t, drawHistory, func(t rapid.TB, h history) {
		account := newAccount(t, conn, false)
		var cursor string
		f := play(t, account, h, func(f *fake.Fake) {
			d := deps(t, pool, &direct{port: f}, pol, account)
			if _, err := tick.Run(context.Background(), d, account); err != nil {
				t.Fatalf("the first tick failed: %v", err)
			}
			var c *string
			if err := conn.QueryRow(context.Background(), "SELECT sync_cursor FROM account_state WHERE account_id = $1", account).Scan(&c); err != nil || c == nil {
				t.Fatalf("reading the cursor: %v", err)
			}
			cursor = *c
		})
		d := deps(t, pool, &direct{port: f}, pol, account)
		if _, err := tick.Run(context.Background(), d, account); err != nil {
			t.Fatalf("the tick failed: %v", err)
		}
		once := dump(t, conn, account)
		must(t, conn, "UPDATE account_state SET sync_cursor = $1 WHERE account_id = $2", cursor, account)
		if _, err := tick.Run(context.Background(), d, account); err != nil {
			t.Fatalf("the tick applying the changes again failed: %v", err)
		}
		if diff := cmp.Diff(once, dump(t, conn, account), compare.Options); diff != "" {
			t.Fatalf("%+v: applying the changes twice left the index unlike once (-once +twice):\n%s", h, diff)
		}
	})
}

// historyKind classifies a history by the rarest telling thing it holds, in the order listed.
func historyKind(h history) string {
	kinds := map[int]bool{}
	for _, c := range h.Changes {
		kinds[c.Kind] = true
	}
	switch {
	case len(h.Changes) == 0:
		return "no change"
	case kinds[2]:
		return "a removal"
	case kinds[1]:
		return "a label change"
	default:
		return "deliveries alone"
	}
}

// The generator report for the property above. Each kind is reached, stated as whether it is reached
// at all, not as an expected share.
func TestApplyingTheSameChangesTwiceLeavesTheIndexAsOnceMix(t *testing.T) {
	property.Report(t, drawHistory, historyKind, map[string]float64{
		"no change": 0.001, "a removal": 0.01, "a label change": 0.01, "deliveries alone": 0.01,
	})
}
