//go:build integration

package service_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/fixture"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// mediator returns a pool connecting as the mediator's role, under which row-level security and the
// grants apply as they do in production.
func mediator(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["options"] = "-c role=mediated_mailbox_mediate"
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// superuser returns a connection that bypasses row-level security, to write the rows a test stages.
func superuser(t *testing.T) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return conn
}

func must(t *testing.T, conn *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// newAccount creates an account named for the test.
func newAccount(t *testing.T, conn *pgx.Conn) string {
	t.Helper()
	account := "acct-" + strings.ToLower(rand.Text()[:10])
	must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", account)
	return account
}

// row is one message's row in the index.
type row struct {
	id, thread, from, name, subject string
	sentAt                          string
	labels, types, flags            []string
	scan                            string
	storedClass                     string
}

func insert(t *testing.T, conn *pgx.Conn, account string, r row) {
	t.Helper()
	domain := r.from[strings.LastIndexByte(r.from, '@')+1:]
	class := r.storedClass
	if class == "" {
		class = "normal"
	}
	must(t, conn, `INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, from_name, subject,
		sent_at, labels, has_attachments, attachment_types, sender_class, content_flags, rule_ids, scan_state)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, '{}', $14)`,
		account, r.id, r.thread, r.from, domain, r.name, r.subject, r.sentAt, orEmpty(r.labels), len(r.types) > 0,
		orEmpty(r.types), class, orEmpty(r.flags), r.scan)
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// reads returns the registry of the operations the mediator serves, reading as the mediator's role,
// serving accounts, and deciding under the policy the tables hold for them.
func reads(t *testing.T, accounts ...string) service.Registry {
	t.Helper()
	pool := mediator(t)
	return readsOver(t, pool, pool, accounts...)
}

// readsOver returns the registry reads returns, with its operations reading through db and the policy
// loaded through pool.
func readsOver(t *testing.T, pool *pgxpool.Pool, db service.Database, accounts ...string) service.Registry {
	t.Helper()
	policies, err := policyload.New(pool, accounts, prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if err := policies.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	var listed []service.Account
	for _, a := range accounts {
		listed = append(listed, service.Account{ID: a, Provider: "gmail"})
	}
	reg, err := service.NewRegistry(accounts, service.Operations(service.Sources{
		DB:       db,
		Accounts: func() []service.Account { return listed },
		Policy:   func(account string) policy.Composed { return policies.Snapshot().For(account) },
		Lookups:  classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne},
		Now:      func() time.Time { return time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC) },
	})...)
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

// call runs an operation and returns its result.
func call(t *testing.T, reg service.Registry, name, args string) json.RawMessage {
	t.Helper()
	out, err := reg.Call(t.Context(), name, json.RawMessage(args))
	if err != nil {
		t.Fatalf("%s %s: %v", name, args, err)
	}
	return out
}

// servedMessage is what a test reads of a served message.
type servedMessage struct {
	MessageID   string   `json:"message_id"`
	Date        string   `json:"date"`
	Labels      []string `json:"labels"`
	Sensitivity struct {
		SenderClass  string   `json:"sender_class"`
		ContentFlags []string `json:"content_flags"`
		ScanState    string   `json:"scan_state"`
	} `json:"sensitivity"`
	BodyAvailable bool `json:"body_available"`
}

// state returns what the message says of its sensitivity and whether its body is available.
func (m servedMessage) state() string {
	s := m.Sensitivity
	return fmt.Sprintf("%s %v %s body=%v", s.SenderClass, s.ContentFlags, s.ScanState, m.BodyAvailable)
}

// result is what a test reads of any operation's result.
type result struct {
	servedMessage
	Accounts []service.Account `json:"accounts"`
	Labels   []string          `json:"labels"`
	Messages []servedMessage   `json:"messages"`
	Threads  []struct {
		ThreadID string `json:"thread_id"`
		LatestAt string `json:"latest_at"`
	} `json:"threads"`
	Events []struct {
		MessageID string `json:"message_id"`
		MaskedAt  string `json:"masked_at"`
	} `json:"events"`
	NextCursor *string `json:"next_cursor"`
}

// read runs an operation and decodes its result.
func read(t *testing.T, reg service.Registry, name, args string) result {
	t.Helper()
	var r result
	if err := json.Unmarshal(call(t, reg, name, args), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

// Every message a read serves follows the redaction matrix under the policy in force (ADR-0001,
// ADR-0002). A sender the policy lists after its message was stored as normal is restricted and its
// body unavailable, and a flagged message and a message the scanner has not reached have no body. A
// stored flag or scan state the schema does not name, which the schema's checks refuse to store, is
// TestAnUnnamedStoredStateReadsAsTheMostRestrictive's. No served message carries a snippet, an
// attachment filename or body text, since the index holds none (ADR-0016).
func TestEveryServedMessageFollowsTheRedactionMatrix(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	bank, news, code, link := fixture.Bank(), fixture.Newsletter(), fixture.OneTimeCode(), fixture.LoginLink()
	must(t, conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES ($1, $2, 'restricted', ARRAY['bank.example'], 'operator', 'test')`, account, account+".bank")
	rows := []row{
		{id: "m-bank", thread: "t-bank", from: bank.FromAddress, name: bank.FromName, subject: bank.Subject, sentAt: "2026-07-20T09:12:00Z", types: []string{"pdf"}, scan: "scanned", storedClass: "normal"},
		{id: "m-news", thread: "t-news", from: news.FromAddress, name: news.FromName, subject: news.Subject, sentAt: "2026-07-21T18:04:00Z", labels: []string{"INBOX"}, scan: "scanned"},
		{id: "m-code", thread: "t-code", from: code.FromAddress, name: code.FromName, subject: code.Subject, sentAt: "2026-07-21T11:40:00Z", flags: []string{"mfa_code"}, scan: "scanned"},
		{id: "m-link", thread: "t-link", from: link.FromAddress, name: link.FromName, subject: link.Subject, sentAt: "2026-07-21T11:41:00Z", flags: []string{"login_link"}, scan: "scanned"},
		{id: "m-pending", thread: "t-news", from: news.FromAddress, name: news.FromName, subject: news.Subject, sentAt: "2026-07-21T19:00:00Z", scan: "pending"},
	}
	for _, r := range rows {
		insert(t, conn, account, r)
	}
	reg := reads(t, account)

	want := map[string]string{
		"m-bank":    "restricted [] scanned body=false",
		"m-news":    "normal [] scanned body=true",
		"m-code":    "normal [mfa_code] scanned body=false",
		"m-link":    "normal [login_link] scanned body=false",
		"m-pending": "normal [] pending body=false",
	}
	got := map[string]string{}
	for id := range want {
		raw := call(t, reg, "get_message", `{"account_id":"`+account+`","message_id":"`+id+`"}`)
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		got[id] = read(t, reg, "get_message", `{"account_id":"`+account+`","message_id":"`+id+`"}`).state()
		for _, field := range []string{"snippet", "attachment_names", "attachments", "body"} {
			if _, present := m[field]; present {
				t.Errorf("%s carries %s", id, field)
			}
		}
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the gate's decisions (-want +got):\n%s", diff)
	}

	for _, op := range []string{`"list_messages"`, `"get_thread","thread_id":"t-news"`} {
		name, extra, _ := strings.Cut(op, ",")
		args := `{"account_id":"` + account + `"`
		if extra != "" {
			args += "," + extra
		}
		args += "}"
		out := call(t, reg, strings.Trim(name, `"`), args)
		if strings.Contains(string(out), marker.BodyPrefix) || strings.Contains(string(out), "snippet") {
			t.Errorf("%s serves body text or a snippet: %s", name, out)
		}
		var r result
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatal(err)
		}
		for _, m := range r.Messages {
			if w, ok := want[m.MessageID]; ok && m.state() != w {
				t.Errorf("%s serves %s as %s, want %s", name, m.MessageID, m.state(), w)
			}
		}
	}
}

// Every timestamp a read serves is ISO 8601 UTC with the Z suffix, whatever offset the provider gave
// it when it was stored and whatever the process's local zone (ADR-0033). The test runs with a local
// zone two hours east of UTC, which the driver reads timestamps into.
func TestEveryTimestampServedIsUTC(t *testing.T) {
	local := time.Local
	time.Local = time.FixedZone("east", 2*60*60)
	t.Cleanup(func() { time.Local = local })
	conn := superuser(t)
	account := newAccount(t, conn)
	news := fixture.Newsletter()
	insert(t, conn, account, row{id: "m-1", thread: "t-1", from: news.FromAddress, subject: news.Subject, sentAt: "2026-07-21T20:04:00+02:00", scan: "scanned"})
	insert(t, conn, account, row{id: "m-2", thread: "t-1", from: news.FromAddress, subject: news.Subject, sentAt: "2026-07-21T08:30:00.25-05:30", scan: "scanned"})
	must(t, conn, `INSERT INTO masking_events (account_id, message_id, field, rule_id, tier, masked_at)
		VALUES ($1, 'm-1', 'subject', 'content.mfa.subject_numeric_6', 1, '2026-07-21T20:05:00+02:00')`, account)
	must(t, conn, `INSERT INTO senders (account_id, domain, message_count, first_seen, last_seen)
		VALUES ($1, 'newsletter.example', 2, '2026-07-21T08:30:00.25-05:30', '2026-07-21T20:04:00+02:00')`, account)
	reg := reads(t, account)

	var dates []string
	for _, m := range read(t, reg, "get_thread", `{"account_id":"`+account+`","thread_id":"t-1"}`).Messages {
		dates = append(dates, m.Date)
	}
	dates = append(dates, read(t, reg, "list_threads", `{"account_id":"`+account+`"}`).Threads[0].LatestAt)
	dates = append(dates, read(t, reg, "list_masking_events", `{"account_id":"`+account+`"}`).Events[0].MaskedAt)
	for _, m := range read(t, reg, "search_messages", `{"account_id":"`+account+`","order":"ascending"}`).Messages {
		dates = append(dates, m.Date)
	}
	summary := count(t, reg, account, `{}`, "").Summary
	month := count(t, reg, account, `{}`, "month").Groups[0]
	dates = append(dates, *summary.OldestAt, *summary.NewestAt, *month.Key, *month.OldestAt, *month.NewestAt)
	stats, _ := senderPage(t, reg, `{"account_id":"`+account+`"}`)
	dates = append(dates, *stats[0].FirstSeen, *stats[0].LastSeen)
	want := []string{
		"2026-07-21T14:00:00.25Z", "2026-07-21T18:04:00Z", "2026-07-21T18:04:00Z", "2026-07-21T18:05:00Z",
		"2026-07-21T14:00:00.25Z", "2026-07-21T18:04:00Z",
		"2026-07-21T14:00:00.25Z", "2026-07-21T18:04:00Z", "2026-07-01T00:00:00Z", "2026-07-21T14:00:00.25Z", "2026-07-21T18:04:00Z",
		"2026-07-21T14:00:00.25Z", "2026-07-21T18:04:00Z",
	}
	if diff := cmp.Diff(want, dates, compare.Options); diff != "" {
		t.Errorf("the timestamps served (-want +got):\n%s", diff)
	}
}

// A timestamp given with any offset but Z is refused with an error that says so, never converted, and
// one given in UTC is taken as it is (ADR-0033).
func TestANonUTCTimestampIsRefused(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	must(t, conn, `INSERT INTO masking_events (account_id, message_id, field, rule_id, tier, masked_at)
		VALUES ($1, 'm-1', 'subject', 'content.mfa.subject_numeric_6', 1, '2026-07-21T18:05:00Z')`, account)
	reg := reads(t, account)
	for _, since := range []string{"2026-07-21T20:00:00+02:00", "2026-07-21T18:00:00+00:00", "2026-07-21T18:00:00-00:00", "2026-07-21T18:00:00", "yesterday"} {
		_, err := reg.Call(t.Context(), "list_masking_events", json.RawMessage(`{"account_id":"`+account+`","since":"`+since+`"}`))
		var arg *service.ArgumentError
		if !errors.As(err, &arg) || !strings.Contains(string(service.Failure(err)), "since") {
			t.Errorf("since %q: %v, want a refusal naming since", since, err)
		}
	}
	for _, at := range []string{"2026-07-21T20:00:00+02:00", "2026-07-21T18:00:00+00:00", "2026-07-21T18:00:00", "yesterday"} {
		for _, op := range []string{"search_messages", "count_messages"} {
			for _, term := range []string{"after", "before"} {
				_, err := reg.Call(t.Context(), op, json.RawMessage(`{"account_id":"`+account+`","query":{"`+term+`":"`+at+`"}}`))
				var arg *service.ArgumentError
				if !errors.As(err, &arg) || !strings.Contains(string(service.Failure(err)), "query."+term) {
					t.Errorf("%s with %s %q: %v, want a refusal naming query.%s", op, term, at, err, term)
				}
			}
		}
	}
	insert(t, conn, account, row{id: "m-1", thread: "t-1", from: "a@news.example", subject: "s", sentAt: "2026-07-21T18:05:00Z", scan: "scanned"})
	for query, n := range map[string]int64{`{"after":"2026-07-21T18:05:00Z"}`: 1, `{"after":"2026-07-21T18:05:00.000001Z"}`: 0, `{"before":"2026-07-21T18:05:00.000001Z"}`: 1, `{"before":"2026-07-21T18:05:00Z"}`: 0} {
		if got := count(t, reg, account, query, "").Summary.Messages; got != n {
			t.Errorf("count %s counted %d, want %d", query, got, n)
		}
	}
	for since, n := range map[string]int{"2026-07-21T18:00:00Z": 1, "2026-07-21T18:05:00.000001Z": 0} {
		events := read(t, reg, "list_masking_events", `{"account_id":"`+account+`","since":"`+since+`"}`).Events
		if len(events) != n {
			t.Errorf("since %s returned %d events, want %d", since, len(events), n)
		}
	}
}

// A listing pages through every row exactly once, newest first, and a cursor continues only the
// listing, account and filter it came from.
func TestAListingPagesThroughEveryRowOnce(t *testing.T) {
	conn := superuser(t)
	account, other := newAccount(t, conn), newAccount(t, conn)
	news := fixture.Newsletter()
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	const n = 231
	for i := range n {
		// Message i sits in thread i/2, and the two messages of a thread share its time. Two threads
		// share each time after the first, so the threads' latest times tie in pairs, and a message's
		// time is shared by up to four messages. With 231 messages and 116 threads, the last row of each
		// listing's first page ties with the first row of its second, a message tying across the page
		// boundary among four and a thread in its pair. Masking events are one per message at minute
		// i/2, so pairs of them tie and the odd count puts a pair across the boundary too.
		thread := i / 2
		sent := base.Add(time.Duration((thread+1)/2) * time.Minute).Format(time.RFC3339)
		masked := base.Add(time.Duration(i/2) * time.Minute).Format(time.RFC3339)
		insert(t, conn, account, row{id: fmt.Sprintf("m-%03d", i), thread: fmt.Sprintf("t-%03d", thread), from: news.FromAddress, subject: news.Subject, sentAt: sent, labels: []string{fmt.Sprintf("L%d", i%4)}, scan: "scanned"})
		must(t, conn, `INSERT INTO masking_events (account_id, message_id, field, rule_id, tier, masked_at) VALUES ($1, $2, 'subject', 'r', 1, $3)`, account, fmt.Sprintf("m-%03d", i), masked)
	}
	reg := reads(t, account, other)

	// A cursor each listing returned, handed to another listing.
	otherListing := map[string]string{}
	for op, from := range map[string]string{"list_messages": "list_threads", "list_threads": "list_masking_events", "list_masking_events": "list_messages"} {
		first := read(t, reg, from, `{"account_id":"`+account+`"}`)
		if first.NextCursor == nil {
			t.Fatalf("%s returned one page", from)
		}
		otherListing[op] = *first.NextCursor
	}
	for _, c := range []struct{ op, extra string }{
		{"list_messages", ""},
		{"list_threads", ""},
		{"list_masking_events", `,"since":"2026-07-01T00:00:00Z"`},
	} {
		seen := map[string]int{}
		cursor := ""
		var last string
		for pages := 0; ; pages++ {
			if pages > 10 {
				t.Fatalf("%s did not end", c.op)
			}
			args := `{"account_id":"` + account + `"` + c.extra + cursor + `}`
			out := read(t, reg, c.op, args)
			var ids []string
			for _, m := range out.Messages {
				ids = append(ids, m.MessageID)
			}
			for _, th := range out.Threads {
				ids = append(ids, th.ThreadID)
			}
			for _, e := range out.Events {
				ids = append(ids, e.MessageID)
			}
			for _, id := range ids {
				seen[id]++
			}
			if out.NextCursor == nil {
				break
			}
			next := *out.NextCursor
			last = next
			cursor = `,"cursor":"` + next + `"`
		}
		wantRows := n
		if c.op == "list_threads" {
			wantRows = (n + 1) / 2
		}
		if len(seen) != wantRows {
			t.Errorf("%s served %d distinct rows, want %d", c.op, len(seen), wantRows)
		}
		for id, times := range seen {
			if times != 1 {
				t.Errorf("%s served %s %d times", c.op, id, times)
			}
		}
		for _, misuse := range []string{
			`{"account_id":"` + other + `"` + c.extra + `,"cursor":"` + last + `"}`,
			`{"account_id":"` + account + `","since":"2026-07-02T00:00:00Z","cursor":"` + last + `"}`,
			`{"account_id":"` + account + `"` + c.extra + `,"cursor":"not-a-cursor"}`,
			`{"account_id":"` + account + `"` + c.extra + `,"cursor":"` + otherListing[c.op] + `"}`,
		} {
			if c.op != "list_masking_events" && strings.Contains(misuse, "since") {
				continue
			}
			_, err := reg.Call(t.Context(), c.op, json.RawMessage(misuse))
			var arg *service.ArgumentError
			if !errors.As(err, &arg) {
				t.Errorf("%s with %s: %v, want the cursor refused", c.op, misuse, err)
			}
		}
	}

	labels := read(t, reg, "list_labels", `{"account_id":"`+account+`"}`).Labels
	if diff := cmp.Diff([]string{"L0", "L1", "L2", "L3"}, labels, compare.Options); diff != "" {
		t.Errorf("list_labels (-want +got):\n%s", diff)
	}
}

// Every identifier an operation takes comes from another operation's result (ADR-0035). An account
// from the accounts listing, a message from the message listing and a thread from the thread listing
// each reach the read that takes it, and the labels are the values the messages hold. The index query's
// label, address and domain terms take a label from the labels listing, an address from a served
// message and a domain from the sender listing, and each selects the message.
func TestEveryIdentifierAnOperationTakesIsDiscoverable(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	news := fixture.Newsletter()
	insert(t, conn, account, row{id: "m-1", thread: "t-1", from: news.FromAddress, subject: news.Subject, sentAt: "2026-07-21T18:04:00Z", labels: []string{"INBOX"}, scan: "scanned"})
	domain := news.FromAddress[strings.LastIndexByte(news.FromAddress, '@')+1:]
	must(t, conn, `INSERT INTO senders (account_id, domain, message_count) VALUES ($1, $2, 1)`, account, domain)
	reg := reads(t, account)

	var served struct {
		Messages []struct {
			From struct {
				Email string `json:"email"`
			} `json:"from"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(call(t, reg, "list_messages", `{"account_id":"`+account+`"}`), &served); err != nil {
		t.Fatal(err)
	}
	stats, _ := senderPage(t, reg, `{"account_id":"`+account+`"}`)
	label := read(t, reg, "list_labels", `{"account_id":"`+account+`"}`).Labels[0]
	for _, term := range []string{
		`"labels":["` + label + `"]`,
		`"from":"` + served.Messages[0].From.Email + `"`,
		`"from_domain":"` + stats[0].Domain + `"`,
	} {
		if got := ids(searched(t, reg, account, `{`+term+`}`, "")); !slices.Equal(got, []string{"m-1"}) {
			t.Errorf("search by a discovered %s found %v, want m-1", term, got)
		}
	}

	accounts := read(t, reg, "list_accounts", `{}`).Accounts
	if diff := cmp.Diff([]service.Account{{ID: account, Provider: "gmail"}}, accounts, compare.Options); diff != "" {
		t.Fatalf("list_accounts (-want +got):\n%s", diff)
	}
	id := accounts[0].ID
	msg := read(t, reg, "list_messages", `{"account_id":"`+id+`"}`).Messages[0]
	thr := read(t, reg, "list_threads", `{"account_id":"`+id+`"}`).Threads[0]
	got := read(t, reg, "get_message", `{"account_id":"`+id+`","message_id":"`+msg.MessageID+`"}`)
	if got.MessageID != "m-1" || len(got.Labels) != 1 || !slices.Equal(got.Labels, read(t, reg, "list_labels", `{"account_id":"`+id+`"}`).Labels) {
		t.Errorf("get_message by a listed identifier returned %+v", got)
	}
	if th := read(t, reg, "get_thread", `{"account_id":"`+id+`","thread_id":"`+thr.ThreadID+`"}`); len(th.Messages) != 1 {
		t.Errorf("get_thread by a listed identifier returned %+v", th)
	}
	for _, missing := range []string{`"get_message","message_id":"absent"`, `"get_thread","thread_id":"absent"`} {
		op, arg, _ := strings.Cut(missing, ",")
		_, err := reg.Call(t.Context(), strings.Trim(op, `"`), json.RawMessage(`{"account_id":"`+id+`",`+arg+`}`))
		var refused *service.ArgumentError
		if !errors.As(err, &refused) {
			t.Errorf("%s of an absent identifier: %v, want it refused", op, err)
		}
	}
}

// The system status reads only recorded operational state. With normal-sender fixtures whose subjects
// and senders carry marker text, and a policy rule, recorded, the whole response carries no marker
// text and no policy detail, only the backfill flags, the sync cursor and last tick, the backlog, the
// rate state and the last authentication (ADR-0034).
func TestTheSystemStatusCarriesOperationalStateOnly(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	must(t, conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES ($1, $2, 'restricted', ARRAY['mmfieldmarker-policy.example'], 'operator', 'mmfieldmarker-operator')`, account, account+".mmfieldmarker-rule")
	for i, f := range fixture.All() {
		insert(t, conn, account, row{id: fmt.Sprintf("m-%d", i), thread: "t", from: f.FromAddress, name: f.FromName, subject: f.Subject, sentAt: "2026-07-21T18:04:00Z", scan: []string{"pending", "scanned"}[i%2]})
	}
	must(t, conn, `INSERT INTO account_state (account_id, backfill_pass1_complete, sync_cursor, sync_cursor_at, last_auth_at, last_auth_outcome)
		VALUES ($1, true, 'mmfieldmarker-cursor', '2026-07-21T23:00:00+01:00', '2026-07-21T21:00:00Z', 'succeeded')`, account)
	must(t, conn, `INSERT INTO rate_state (account_id, current_rate, target_rate, hard_cap, backoff_until) VALUES ($1, 5, 8, 10, '2026-07-22T00:01:00Z')`, account)
	must(t, conn, `INSERT INTO job_runs (account_id, run_id, workload, pass, state, started_at, finished_at, last_error)
		VALUES ($1, 'r1', 'sync', 'tick', 'succeeded', '2026-07-21T21:55:00Z', '2026-07-21T21:56:00Z', 'mmfieldmarker-error'),
		       ($1, 'r2', 'sync', 'tick', 'failed', '2026-07-21T22:55:00Z', '2026-07-21T22:56:00Z', 'mmfieldmarker-error')`, account)
	reg := reads(t, account)

	out, err := reg.Call(t.Context(), "get_system_status", json.RawMessage(`{"account_id":"`+account+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{marker.FieldPrefix, marker.BodyPrefix, "bank.example", "restricted", "419283"} {
		if strings.Contains(string(out), leak) {
			t.Errorf("the status carries %q: %s", leak, out)
		}
	}
	var got any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"account_id":          account,
		"backfill":            map[string]any{"pass1_complete": true, "pass2_complete": false},
		"sync":                map[string]any{"cursor_written_at": "2026-07-21T22:00:00Z", "cursor_age_seconds": float64(7200), "last_succeeded_tick_at": "2026-07-21T21:56:00Z"},
		"scan_backlog":        float64(4),
		"rate":                map[string]any{"current_rate": float64(5), "target_rate": float64(8), "backoff_until": "2026-07-22T00:01:00Z", "in_backoff": true},
		"last_authentication": map[string]any{"at": "2026-07-21T21:00:00Z", "outcome": "succeeded"},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the status (-want +got):\n%s", diff)
	}
}

// An account with nothing recorded yet has a status whose recorded sections are null and whose backlog
// is zero, never an error.
func TestTheStatusOfAnAccountWithNothingRecorded(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	reg := reads(t, account)
	var got any
	if err := json.Unmarshal(call(t, reg, "get_system_status", `{"account_id":"`+account+`"}`), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"account_id": account, "backfill": nil, "sync": nil, "scan_backlog": float64(0), "rate": nil, "last_authentication": nil}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the status (-want +got):\n%s", diff)
	}
}
