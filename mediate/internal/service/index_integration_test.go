//go:build integration

package service_test

import (
	"context"
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

	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
)

// indexed is one message's row in the index, with the state the index reads select by.
type indexed struct {
	id, thread, from, subject, sent string
	labels                          []string
	read, starred, attachments      bool
	flags                           []string
	scan, storedClass               string
	// domain is the domain stored for the sender, the part of its address after the last @ lowered
	// when it is empty.
	domain string
}

// put stores m in the index with its sender's statistics, in one statement, as a workload stores a
// message and rebuilds its sender's statistics in one transaction, so the statistics hold exactly the
// domains the index stores.
func put(t *testing.T, conn *pgx.Conn, account string, m indexed) {
	t.Helper()
	domain := m.domain
	if domain == "" {
		domain = strings.ToLower(m.from[strings.LastIndexByte(m.from, '@')+1:])
	}
	class, scan, thread := m.storedClass, m.scan, m.thread
	if class == "" {
		class = "normal"
	}
	if scan == "" {
		scan = "scanned"
	}
	if thread == "" {
		thread = m.id
	}
	var types []string
	if m.attachments {
		types = []string{"pdf"}
	}
	flags := fmt.Sprintf(`{"read": %v, "starred": %v}`, m.read, m.starred)
	must(t, conn, `WITH stored AS (
		INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, subject, sent_at, labels,
		flags, has_attachments, attachment_types, sender_class, content_flags, rule_ids, scan_state)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, '{}', $14) RETURNING account_id, from_domain)
		INSERT INTO senders (account_id, domain, message_count) SELECT account_id, from_domain, 1 FROM stored
		ON CONFLICT (account_id, domain) DO UPDATE SET message_count = senders.message_count + 1`,
		account, m.id, thread, m.from, domain, m.subject, m.sent, orEmpty(m.labels), flags, m.attachments, orEmpty(types),
		class, orEmpty(m.flags), scan)
}

// restrict lists domain in the account's policy, as a rule restricting its senders.
func restrict(t *testing.T, conn *pgx.Conn, account, domain string) {
	t.Helper()
	must(t, conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES ($1, $2, 'restricted', ARRAY[$3], 'operator', 'test')`, account, account+"."+domain, domain)
}

// counts is a group or a summary as a test reads it.
type counts struct {
	Key             *string `json:"key"`
	Messages        int64   `json:"messages"`
	Threads         int64   `json:"threads"`
	Unread          int64   `json:"unread"`
	WithAttachments int64   `json:"with_attachments"`
	OldestAt        *string `json:"oldest_at"`
	NewestAt        *string `json:"newest_at"`
}

type counted struct {
	Summary    counts   `json:"summary"`
	GroupBy    *string  `json:"group_by"`
	Groups     []counts `json:"groups"`
	NextCursor *string  `json:"next_cursor"`
}

// count runs count_messages on account with query and group_by.
func count(t *testing.T, reg service.Registry, account, query, groupBy string) counted {
	t.Helper()
	args := `{"account_id":"` + account + `"`
	if query != "" {
		args += `,"query":` + query
	}
	if groupBy != "" {
		args += `,"group_by":"` + groupBy + `"`
	}
	var out counted
	if err := json.Unmarshal(call(t, reg, "count_messages", args+`}`), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// allGroups pages through every group count_messages returns for query and group_by.
func allGroups(t *testing.T, reg service.Registry, account, query, groupBy string) []counts {
	t.Helper()
	var groups []counts
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > 20 {
			t.Fatalf("count_messages by %s did not end", groupBy)
		}
		args := `{"account_id":"` + account + `","query":` + query + `,"group_by":"` + groupBy + `"` + cursor + `}`
		var out counted
		if err := json.Unmarshal(call(t, reg, "count_messages", args), &out); err != nil {
			t.Fatal(err)
		}
		groups = append(groups, out.Groups...)
		if out.NextCursor == nil {
			return groups
		}
		cursor = `,"cursor":"` + *out.NextCursor + `"`
	}
}

// searched pages through every message search_messages returns for query, sort and order, and
// returns them in order.
func searched(t *testing.T, reg service.Registry, account, query, sortOrder string) []servedMessage {
	t.Helper()
	var all []servedMessage
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > 20 {
			t.Fatal("search_messages did not end")
		}
		out := read(t, reg, "search_messages", `{"account_id":"`+account+`","query":`+query+sortOrder+cursor+`}`)
		all = append(all, out.Messages...)
		if out.NextCursor == nil {
			return all
		}
		cursor = `,"cursor":"` + *out.NextCursor + `"`
	}
}

func ids(ms []servedMessage) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.MessageID)
	}
	return out
}

func keyOf(g counts) string {
	if g.Key == nil {
		return "<none>"
	}
	return *g.Key
}

// byKey returns each group's message count under its key.
func byKey(groups []counts) map[string]int64 {
	out := map[string]int64{}
	for _, g := range groups {
		out[keyOf(g)] = g.Messages
	}
	return out
}

// A message whose body is denied stays in every count, group and search result (G1). A restricted
// sender's message, a flagged one and one the scanner has not reached are each selected, counted and
// grouped exactly as an ordinary message is, and served with their body unavailable.
func TestEveryMessageStaysInEveryIndexRead(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	restrict(t, conn, account, "bank.example")
	bank, news, shop := marker.Field("alerts")+"@bank.example", marker.Field("news")+"@news.example", marker.Field("orders")+"@shop.example"
	corpus := []indexed{
		{id: "m-restricted", from: bank, subject: marker.Field("overdraft"), sent: "2025-03-10T09:00:00Z", labels: []string{"Finance"}, attachments: true, scan: "skipped_restricted", storedClass: "restricted"},
		{id: "m-flagged", from: news, subject: marker.Field("signin"), sent: "2025-03-11T09:00:00Z", labels: []string{"INBOX"}, read: true, flags: []string{"login_link"}},
		{id: "m-pending", from: news, subject: marker.Field("weekly"), sent: "2025-04-01T09:00:00Z", scan: "pending"},
		{id: "m-open", from: shop, subject: marker.Field("receipt"), sent: "2025-04-02T09:00:00Z", labels: []string{"INBOX"}, read: true, starred: true},
	}
	for _, m := range corpus {
		put(t, conn, account, m)
	}
	reg := reads(t, account)

	all := searched(t, reg, account, `{}`, "")
	if diff := cmp.Diff([]string{"m-open", "m-pending", "m-flagged", "m-restricted"}, ids(all), compare.Options); diff != "" {
		t.Errorf("search with an empty query (-want +got):\n%s", diff)
	}
	available := map[string]bool{}
	for _, m := range all {
		available[m.MessageID] = m.BodyAvailable
	}
	if diff := cmp.Diff(map[string]bool{"m-open": true, "m-pending": false, "m-flagged": false, "m-restricted": false}, available, compare.Options); diff != "" {
		t.Errorf("body_available (-want +got):\n%s", diff)
	}
	for query, want := range map[string]string{
		`{"subject":"` + marker.Field("overdraft") + `"}`: "m-restricted",
		`{"from":"` + strings.ToUpper(bank) + `"}`:        "m-restricted",
		`{"labels":["Finance"]}`:                          "m-restricted",
		`{"sender_class":"restricted"}`:                   "m-restricted",
		`{"has_attachments":true}`:                        "m-restricted",
		`{"subject":"` + marker.Field("signin") + `"}`:    "m-flagged",
		`{"subject":"` + marker.Field("weekly") + `"}`:    "m-pending",
		`{"labels_within":[]}`:                            "m-pending",
	} {
		if got := ids(searched(t, reg, account, query, "")); !slices.Equal(got, []string{want}) {
			t.Errorf("search %s found %v, want %s", query, got, want)
		}
		if got := count(t, reg, account, query, "").Summary.Messages; got != 1 {
			t.Errorf("count %s counted %d, want 1", query, got)
		}
	}

	summary := count(t, reg, account, `{}`, "").Summary
	if diff := cmp.Diff(counts{Messages: 4, Threads: 4, Unread: 2, WithAttachments: 1, OldestAt: ptr("2025-03-10T09:00:00Z"), NewestAt: ptr("2025-04-02T09:00:00Z")}, summary, compare.Options); diff != "" {
		t.Errorf("the summary of the whole index (-want +got):\n%s", diff)
	}
	for groupBy, want := range map[string]map[string]int64{
		"sender":        {bank: 1, news: 2, shop: 1},
		"sender_domain": {"bank.example": 1, "news.example": 2, "shop.example": 1},
		"label":         {"<none>": 1, "Finance": 1, "INBOX": 2},
		"sender_class":  {"restricted": 1, "normal": 3},
		"month":         {"2025-03-01T00:00:00Z": 2, "2025-04-01T00:00:00Z": 2},
	} {
		if diff := cmp.Diff(want, byKey(allGroups(t, reg, account, `{}`, groupBy)), compare.Options); diff != "" {
			t.Errorf("groups by %s (-want +got):\n%s", groupBy, diff)
		}
	}
}

func ptr(s string) *string { return &s }

// Enumeration and counting reach the whole corpus, not a recent window (G1). Two hundred and fifty
// messages, one a month for over twenty years, are each served once by a search with no query, oldest
// first in ascending order, and each counted, the summary dating the oldest twenty years back.
func TestEnumerationReachesTheWholeCorpus(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	start := time.Date(2005, 1, 15, 12, 0, 0, 0, time.UTC)
	const n = 250
	for i := range n {
		put(t, conn, account, indexed{id: fmt.Sprintf("m-%03d", i), from: "someone@news.example", subject: "issue", sent: start.AddDate(0, i, 0).Format(time.RFC3339)})
	}
	reg := reads(t, account)

	for _, order := range []string{"", `,"order":"ascending"`} {
		got := ids(searched(t, reg, account, `{}`, order))
		if len(got) != n {
			t.Errorf("search%s served %d messages, want %d", order, len(got), n)
			continue
		}
		first, last := "m-249", "m-000"
		if order != "" {
			first, last = last, first
		}
		if got[0] != first || got[n-1] != last {
			t.Errorf("search%s ran from %s to %s, want %s to %s", order, got[0], got[n-1], first, last)
		}
	}
	summary := count(t, reg, account, `{}`, "").Summary
	if summary.Messages != n || summary.OldestAt == nil || *summary.OldestAt != "2005-01-15T12:00:00Z" {
		t.Errorf("the summary counts %d messages from %v, want %d from 2005-01-15T12:00:00Z", summary.Messages, summary.OldestAt, n)
	}
	if months := allGroups(t, reg, account, `{}`, "month"); len(months) != n {
		t.Errorf("the messages fall in %d months, want %d", len(months), n)
	}
}

// senderRow is one sender's statistics as a test reads them.
type senderRow struct {
	Domain            string           `json:"domain"`
	SenderClass       string           `json:"sender_class"`
	Messages          int64            `json:"messages"`
	FirstSeen         *string          `json:"first_seen"`
	LastSeen          *string          `json:"last_seen"`
	ListIDShare       *float64         `json:"list_id_share"`
	LabelDistribution map[string]int64 `json:"label_distribution"`
	DisplayNames      []string         `json:"display_names"`
	LocalPartSample   []string         `json:"local_part_sample"`
}

func senderPage(t *testing.T, reg service.Registry, args string) ([]senderRow, *string) {
	t.Helper()
	var out struct {
		Senders    []senderRow `json:"senders"`
		NextCursor *string     `json:"next_cursor"`
	}
	if err := json.Unmarshal(call(t, reg, "list_senders", args), &out); err != nil {
		t.Fatal(err)
	}
	return out.Senders, out.NextCursor
}

// The sender class a query term, a grouping and the sender listing use is the Redaction Gate's under
// the policy in force, never the class the index stored (ADR-0002). A domain the policy lists after its
// messages and its statistics were stored as normal reads as restricted, and a domain stored as
// restricted that the policy does not list reads as normal. A grouping by class keeps to the class the
// query selects, and statistics left with no message in the index are classified by their domain. The
// sender listing serves the statistics and nothing of the stored class or the prior scan hits.
func TestTheIndexReadsClassifySendersUnderThePolicyInForce(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	listed, unlisted := "alerts@bank.example", "legacy@old.example"
	put(t, conn, account, indexed{id: "m-1", from: listed, subject: "s", sent: "2025-03-10T09:00:00Z", labels: []string{"Finance"}})
	put(t, conn, account, indexed{id: "m-2", from: "Statements@Bank.example", subject: "s", sent: "2025-03-11T09:00:00Z", labels: []string{"Finance", "INBOX"}, read: true})
	put(t, conn, account, indexed{id: "m-3", from: unlisted, subject: "s", sent: "2025-03-12T09:00:00Z", storedClass: "restricted", scan: "skipped_restricted"})
	must(t, conn, `INSERT INTO senders (account_id, domain, local_part_sample, display_names, message_count, first_seen, last_seen,
		has_list_id_ratio, label_distribution, scan_hit_count, sender_class) VALUES
		($1, 'bank.example', '{alerts,statements}', '{Bank}', 2, '2025-03-10T11:00:00+02:00', '2025-03-11T09:00:00Z', 0.5, '{"Finance": 2, "INBOX": 1}', 7, 'normal'),
		($1, 'old.example', '{legacy}', '{}', 1, '2025-03-12T09:00:00Z', '2025-03-12T09:00:00Z', NULL, '{}', 0, 'restricted'),
		($1, 'archive.bank.example', '{}', '{}', 1, '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z', NULL, '{}', 0, 'normal')
		ON CONFLICT (account_id, domain) DO UPDATE SET local_part_sample = excluded.local_part_sample,
		display_names = excluded.display_names, message_count = excluded.message_count, first_seen = excluded.first_seen,
		last_seen = excluded.last_seen, has_list_id_ratio = excluded.has_list_id_ratio,
		label_distribution = excluded.label_distribution, scan_hit_count = excluded.scan_hit_count,
		sender_class = excluded.sender_class`, account)
	restrict(t, conn, account, "bank.example")
	reg := reads(t, account)

	for class, want := range map[string][]string{"restricted": {"m-2", "m-1"}, "normal": {"m-3"}} {
		if got := ids(searched(t, reg, account, `{"sender_class":"`+class+`"}`, "")); !slices.Equal(got, want) {
			t.Errorf("search for sender_class %s found %v, want %v", class, got, want)
		}
	}
	if diff := cmp.Diff(map[string]int64{"restricted": 2, "normal": 1}, byKey(allGroups(t, reg, account, `{}`, "sender_class")), compare.Options); diff != "" {
		t.Errorf("groups by sender class (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(map[string]int64{"restricted": 1}, byKey(allGroups(t, reg, account, `{"labels":["INBOX"]}`, "sender_class")), compare.Options); diff != "" {
		t.Errorf("groups by sender class of the INBOX (-want +got):\n%s", diff)
	}
	for class, want := range map[string]map[string]int64{"restricted": {"restricted": 2}, "normal": {"normal": 1}} {
		if diff := cmp.Diff(want, byKey(allGroups(t, reg, account, `{"sender_class":"`+class+`"}`, "sender_class")), compare.Options); diff != "" {
			t.Errorf("groups by sender class of the %s senders (-want +got):\n%s", class, diff)
		}
	}
	share := 0.5
	got, next := senderPage(t, reg, `{"account_id":"`+account+`"}`)
	want := []senderRow{
		{
			Domain: "bank.example", SenderClass: "restricted", Messages: 2, FirstSeen: ptr("2025-03-10T09:00:00Z"), LastSeen: ptr("2025-03-11T09:00:00Z"),
			ListIDShare: &share, LabelDistribution: map[string]int64{"Finance": 2, "INBOX": 1}, DisplayNames: []string{"Bank"}, LocalPartSample: []string{"alerts", "statements"},
		},
		{
			// Statistics whose messages the index no longer holds are classified by their domain alone.
			Domain: "archive.bank.example", SenderClass: "restricted", Messages: 1, FirstSeen: ptr("2024-01-01T00:00:00Z"), LastSeen: ptr("2024-01-01T00:00:00Z"),
			LabelDistribution: map[string]int64{}, DisplayNames: []string{}, LocalPartSample: []string{},
		},
		{
			Domain: "old.example", SenderClass: "normal", Messages: 1, FirstSeen: ptr("2025-03-12T09:00:00Z"), LastSeen: ptr("2025-03-12T09:00:00Z"),
			LabelDistribution: map[string]int64{}, DisplayNames: []string{}, LocalPartSample: []string{"legacy"},
		},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" || next != nil {
		t.Errorf("list_senders (-want +got), next %v:\n%s", next, diff)
	}
	raw := string(call(t, reg, "list_senders", `{"account_id":"`+account+`"}`))
	for _, absent := range []string{"scan_hit", `"domain":"old.example","sender_class":"restricted"`} {
		if strings.Contains(raw, absent) {
			t.Errorf("list_senders carries %q: %s", absent, raw)
		}
	}
}

// Each term of the index query selects the messages it names and no others, labels by membership,
// exclusion and containment, the subject by its served form, which for a masked subject never holds
// the code masked out of it, and the flags by their stored state.
func TestEachQueryTermSelectsItsMessages(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	corpus := []indexed{
		{id: "m-a", from: "Ann@One.example", subject: "Quarterly 50%_report", sent: "2025-01-01T00:00:00Z", labels: []string{"INBOX", "Work"}, attachments: true},
		{id: "m-b", from: "bob@two.example", subject: "your code is ██████", sent: "2025-02-01T00:00:00Z", labels: []string{"INBOX"}, read: true},
		{id: "m-c", from: "ann@one.example", subject: "quarterly plans", sent: "2025-03-01T00:00:00Z", labels: []string{"Work"}, read: true, starred: true},
		{id: "m-d", from: "cy@three.example", subject: "unfiled", sent: "2025-04-01T00:00:00Z"},
	}
	for _, m := range corpus {
		put(t, conn, account, m)
	}
	reg := reads(t, account)
	for query, want := range map[string][]string{
		`{}`:                                      {"m-d", "m-c", "m-b", "m-a"},
		`{"after":"2025-02-01T00:00:00Z"}`:        {"m-d", "m-c", "m-b"},
		`{"before":"2025-02-01T00:00:00Z"}`:       {"m-a"},
		`{"from":"ANN@one.example"}`:              {"m-c", "m-a"},
		`{"from_domain":"ONE.example"}`:           {"m-c", "m-a"},
		`{"labels":["INBOX","Work"]}`:             {"m-a"},
		`{"labels":["Work"]}`:                     {"m-c", "m-a"},
		`{"excluded_labels":["Work"]}`:            {"m-d", "m-b"},
		`{"labels_within":[]}`:                    {"m-d"},
		`{"labels_within":["INBOX"]}`:             {"m-d", "m-b"},
		`{"subject":"QUARTERLY"}`:                 {"m-c", "m-a"},
		`{"subject":"50%_"}`:                      {"m-a"},
		`{"subject":"%"}`:                         {"m-a"},
		`{"subject":"_"}`:                         {"m-a"},
		`{"subject":"██"}`:                        {"m-b"},
		`{"subject":"419283"}`:                    nil,
		`{"unread":true}`:                         {"m-d", "m-a"},
		`{"unread":false}`:                        {"m-c", "m-b"},
		`{"starred":true}`:                        {"m-c"},
		`{"has_attachments":true}`:                {"m-a"},
		`{"has_attachments":false,"unread":true}`: {"m-d"},
	} {
		got := ids(searched(t, reg, account, query, ""))
		if len(want) == 0 {
			want = []string{}
		}
		if diff := cmp.Diff(want, got, compare.Options); diff != "" {
			t.Errorf("search %s (-want +got):\n%s", query, diff)
		}
	}
	for sortOrder, want := range map[string][]string{
		`,"sort":"sender","order":"ascending"`:  {"m-a", "m-c", "m-b", "m-d"},
		`,"sort":"sender"`:                      {"m-d", "m-b", "m-a", "m-c"},
		`,"sort":"subject","order":"ascending"`: {"m-a", "m-c", "m-d", "m-b"},
		`,"sort":"date","order":"ascending"`:    {"m-a", "m-b", "m-c", "m-d"},
	} {
		if diff := cmp.Diff(want, ids(searched(t, reg, account, `{}`, sortOrder)), compare.Options); diff != "" {
			t.Errorf("search sorted%s (-want +got):\n%s", sortOrder, diff)
		}
	}
	work := count(t, reg, account, `{"labels":["Work"]}`, "")
	if diff := cmp.Diff(counts{Messages: 2, Threads: 2, Unread: 1, WithAttachments: 1, OldestAt: ptr("2025-01-01T00:00:00Z"), NewestAt: ptr("2025-03-01T00:00:00Z")}, work.Summary, compare.Options); diff != "" || work.GroupBy != nil || len(work.Groups) != 0 || work.NextCursor != nil {
		t.Errorf("count of Work (-want +got), group_by %v, groups %v:\n%s", work.GroupBy, work.Groups, diff)
	}
	none := count(t, reg, account, `{"from":"nobody@nowhere.example"}`, "label")
	if diff := cmp.Diff(counted{GroupBy: ptr("label"), Groups: []counts{}}, none, compare.Options); diff != "" {
		t.Errorf("count of nothing (-want +got):\n%s", diff)
	}
	labels := allGroups(t, reg, account, `{}`, "label")
	var order []string
	for _, g := range labels {
		order = append(order, keyOf(g))
	}
	if diff := cmp.Diff([]string{"<none>", "INBOX", "Work"}, order, compare.Options); diff != "" {
		t.Errorf("the label groups' order (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(map[string]int64{"one.example": 2}, byKey(allGroups(t, reg, account, `{"subject":"quarterly"}`, "sender_domain")), compare.Options); diff != "" {
		t.Errorf("the sender domains of the quarterly messages (-want +got):\n%s", diff)
	}
}

// The domain term matches a sender domain given in any case against the domain the index stores, a
// dotted capital I among its letters, because the service gives the statement the domain in the form
// the index stores it in rather than leaving case to the database (ADR-0016). A rule written with the
// dotted capital I restricts the domain stored from it and not the domain spelled with a plain i, in
// the class term, in the sender listing and in the messages served (ADR-0108, ADR-0109).
func TestTheDomainAndClassTermsReadTheStoredForm(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	restrict(t, conn, account, "B\u0130NK.example")
	put(t, conn, account, indexed{id: "m-dotted", from: "alerts@B\u0130NK.example", domain: "bi\u0307nk.example", subject: "s", sent: "2025-03-10T09:00:00Z"})
	put(t, conn, account, indexed{id: "m-plain", from: "alerts@bink.example", subject: "s", sent: "2025-03-11T09:00:00Z"})
	reg := reads(t, account)
	for query, want := range map[string][]string{
		`{"from_domain":"B\u0130NK.EXAMPLE"}`:  {"m-dotted"},
		`{"from_domain":"bi\u0307nk.example"}`: {"m-dotted"},
		`{"from_domain":"BINK.EXAMPLE"}`:       {"m-plain"},
		`{"sender_class":"restricted"}`:        {"m-dotted"},
		`{"sender_class":"normal"}`:            {"m-plain"},
	} {
		served := searched(t, reg, account, query, "")
		if diff := cmp.Diff(want, ids(served), compare.Options); diff != "" {
			t.Errorf("search %s (-want +got):\n%s", query, diff)
		}
		for _, m := range served {
			if class := map[string]string{"m-dotted": "restricted", "m-plain": "normal"}[m.MessageID]; m.Sensitivity.SenderClass != class {
				t.Errorf("search %s served %s as %s, want %s", query, m.MessageID, m.Sensitivity.SenderClass, class)
			}
		}
	}
	listed, _ := senderPage(t, reg, `{"account_id":"`+account+`"}`)
	classes := map[string]string{}
	for _, row := range listed {
		classes[row.Domain] = row.SenderClass
	}
	if diff := cmp.Diff(map[string]string{"bi\u0307nk.example": "restricted", "bink.example": "normal"}, classes, compare.Options); diff != "" {
		t.Errorf("the sender listing's classes (-want +got):\n%s", diff)
	}
}

// An index query term named in another case than the schema declares, one the schema does not
// declare, a null term, an empty address, a sender class outside the two, and a sort, an order or a
// grouping outside its set are refused before anything is read (ADR-0087).
func TestAnIndexReadTakesOnlyTheTermsItDeclares(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	reg := reads(t, account)
	for _, c := range []struct{ op, args string }{
		{"search_messages", `"query":{"Unread":true}`},
		{"search_messages", `"query":{"unread":null}`},
		{"search_messages", `"query":{"body":"x"}`},
		{"search_messages", `"query":{"from":""}`},
		{"search_messages", `"query":{"sender_class":"secret"}`},
		{"search_messages", `"query":[]`},
		{"search_messages", `"query":null`},
		{"search_messages", `"sort":"size"`},
		{"search_messages", `"order":"up"`},
		{"count_messages", `"query":{"Sender_Class":"restricted"}`},
		{"count_messages", `"group_by":"subject"`},
		{"list_senders", `"domain":"bank.example"`},
	} {
		_, err := reg.Call(t.Context(), c.op, json.RawMessage(`{"account_id":"`+account+`",`+c.args+`}`))
		var refused *service.ArgumentError
		if !errors.As(err, &refused) {
			t.Errorf("%s with %s: %v, want it refused", c.op, c.args, err)
		}
	}
}

// Each index read pages through every row exactly once, rows tied on their sort value across a page
// boundary included, and a cursor continues only the read, account, query, sort, order and grouping
// it came from (ADR-0087).
func TestAnIndexReadPagesThroughEveryRowOnce(t *testing.T) {
	conn := superuser(t)
	account, other := newAccount(t, conn), newAccount(t, conn)
	base := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)
	const n = 231
	for i := range n {
		// Messages share a time, a sender and a subject in fours, so each sort ties across every page
		// boundary. Each message carries a label of its own, and every fourth one a sender of its own.
		put(t, conn, account, indexed{
			id: fmt.Sprintf("m-%03d", i), from: fmt.Sprintf("s%03d@d%03d.example", i/4, i/4), subject: fmt.Sprintf("subject %03d", i/4),
			sent: base.Add(time.Duration(i/4) * time.Minute).Format(time.RFC3339), labels: []string{fmt.Sprintf("L%03d", i)},
		})
	}
	// The sender listing pages more statistics than the messages' domains, its own rows apart from them.
	must(t, conn, `INSERT INTO senders (account_id, domain, message_count) SELECT $1, 'd' || lpad(i::text, 3, '0') || '.example', 4
		FROM generate_series(0, 230) AS i ON CONFLICT (account_id, domain) DO NOTHING`, account)
	reg := reads(t, account, other)

	for _, sortOrder := range []string{"", `,"order":"ascending"`, `,"sort":"sender"`, `,"sort":"sender","order":"ascending"`, `,"sort":"subject"`, `,"sort":"subject","order":"ascending"`} {
		got := ids(searched(t, reg, account, `{}`, sortOrder))
		if seen := uniq(got); len(got) != n || seen != n {
			t.Errorf("search%s served %d rows, %d distinct, want %d once each", sortOrder, len(got), seen, n)
		}
	}
	for _, groupBy := range []string{"label", "sender", "sender_domain"} {
		want := map[string]int{"label": n, "sender": (n + 3) / 4, "sender_domain": (n + 3) / 4}[groupBy]
		var keys []string
		for _, g := range allGroups(t, reg, account, `{}`, groupBy) {
			keys = append(keys, keyOf(g))
		}
		if seen := uniq(keys); len(keys) != want || seen != want {
			t.Errorf("groups by %s served %d groups, %d distinct, want %d once each", groupBy, len(keys), seen, want)
		}
	}
	var domains []string
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > 10 {
			t.Fatal("list_senders did not end")
		}
		page, next := senderPage(t, reg, `{"account_id":"`+account+`"`+cursor+`}`)
		for _, s := range page {
			domains = append(domains, s.Domain)
		}
		if next == nil {
			break
		}
		cursor = `,"cursor":"` + *next + `"`
	}
	if seen := uniq(domains); len(domains) != n || seen != n {
		t.Errorf("list_senders served %d senders, %d distinct, want %d once each", len(domains), seen, n)
	}

	searchCursor := *read(t, reg, "search_messages", `{"account_id":"`+account+`","query":{}}`).NextCursor
	countCursor := *count(t, reg, account, `{}`, "label").NextCursor
	_, sendersNext := senderPage(t, reg, `{"account_id":"`+account+`"}`)
	messagesCursor := *read(t, reg, "list_messages", `{"account_id":"`+account+`"}`).NextCursor
	for _, misuse := range []struct{ op, args string }{
		{"search_messages", `"account_id":"` + other + `","query":{},"cursor":"` + searchCursor + `"`},
		{"search_messages", `"account_id":"` + account + `","query":{"unread":true},"cursor":"` + searchCursor + `"`},
		{"search_messages", `"account_id":"` + account + `","query":{},"sort":"sender","cursor":"` + searchCursor + `"`},
		{"search_messages", `"account_id":"` + account + `","query":{},"order":"ascending","cursor":"` + searchCursor + `"`},
		{"search_messages", `"account_id":"` + account + `","query":{},"cursor":"` + countCursor + `"`},
		{"count_messages", `"account_id":"` + account + `","query":{},"group_by":"sender","cursor":"` + countCursor + `"`},
		{"count_messages", `"account_id":"` + account + `","query":{"unread":true},"group_by":"label","cursor":"` + countCursor + `"`},
		{"count_messages", `"account_id":"` + account + `","query":{},"cursor":"` + countCursor + `"`},
		{"count_messages", `"account_id":"` + other + `","query":{},"group_by":"label","cursor":"` + countCursor + `"`},
		{"count_messages", `"account_id":"` + account + `","query":{},"group_by":"label","cursor":"` + searchCursor + `"`},
		{"list_senders", `"account_id":"` + other + `","cursor":"` + *sendersNext + `"`},
		{"list_senders", `"account_id":"` + account + `","cursor":"` + searchCursor + `"`},
		{"list_messages", `"account_id":"` + account + `","cursor":"` + *sendersNext + `"`},
		{"list_senders", `"account_id":"` + account + `","cursor":"` + messagesCursor + `"`},
	} {
		_, err := reg.Call(t.Context(), misuse.op, json.RawMessage(`{`+misuse.args+`}`))
		var refused *service.ArgumentError
		if !errors.As(err, &refused) {
			t.Errorf("%s with %s: %v, want the cursor refused", misuse.op, misuse.args, err)
		}
	}
}

func uniq(s []string) int {
	seen := map[string]bool{}
	for _, v := range s {
		seen[v] = true
	}
	return len(seen)
}

// hooked is a database whose transactions run hook, once, right before the first statement whose
// text names before. With honour unset it opens every transaction at the default isolation level
// whatever options it is given, as a transaction helper that dropped the snapshot would.
type hooked struct {
	pool   *pgxpool.Pool
	before string
	hook   func()
	fired  *bool
	honour bool
}

func (h hooked) Begin(ctx context.Context) (pgx.Tx, error) {
	return h.BeginTx(ctx, pgx.TxOptions{})
}

func (h hooked) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	if !h.honour {
		opts = pgx.TxOptions{}
	}
	t, err := h.pool.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return hookedTx{Tx: t, h: h}, nil
}

type hookedTx struct {
	pgx.Tx
	h hooked
}

func (t hookedTx) run(sql string) {
	if !*t.h.fired && strings.Contains(sql, "-- name: "+t.h.before+" ") {
		*t.h.fired = true
		t.h.hook()
	}
}

func (t hookedTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	t.run(sql)
	return t.Tx.Query(ctx, sql, args...)
}

func (t hookedTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	t.run(sql)
	return t.Tx.QueryRow(ctx, sql, args...)
}

// An index read's statements read one state of the index, though a workload commits between them
// (ADR-0109). With the read's snapshot honoured, a message and its statistics from a listed domain the
// index held no message for, committed after the classification read the domains and before the
// statement that selects, are in none of the call's statements, and a message committed between a
// count's summary and its groups is in neither, so the summary equals its groups, and a message from a
// normal domain the index held no message for, committed before a search's statement, is never
// selected as restricted. With the snapshot dropped, the class term still fails closed. The new domain, which the
// call never classified, is never selected or counted as normal (ADR-0108).
func TestAnIndexReadSeesOneStateOfTheIndex(t *testing.T) {
	for _, honour := range []bool{true, false} {
		t.Run(fmt.Sprintf("snapshot honoured %v", honour), func(t *testing.T) {
			conn := superuser(t)
			account := newAccount(t, conn)
			restrict(t, conn, account, "bank.example")
			put(t, conn, account, indexed{id: "m-1", from: "a@news.example", subject: "s", sent: "2025-03-10T09:00:00Z"})
			other := superuser(t)
			pool := mediator(t)
			n := 0
			// overFrom commits a message from a new address at domain, with its sender's statistics, right
			// before the statement before.
			overFrom := func(before, domain string) service.Registry {
				fired := false
				return readsOver(t, pool, hooked{pool: pool, before: before, fired: &fired, honour: honour, hook: func() {
					n++
					put(t, other, account, indexed{id: fmt.Sprintf("m-new-%d", n), from: fmt.Sprintf("new%d@%s", n, domain), subject: "s", sent: "2025-03-11T09:00:00Z"})
				}}, account)
			}
			over := func(before string) service.Registry { return overFrom(before, "bank.example") }
			for _, m := range searched(t, over("SearchPage"), account, `{"sender_class":"normal"}`, "") {
				if m.Sensitivity.SenderClass != "normal" {
					t.Errorf("a search for normal senders served %s, whose sender is %s", m.MessageID, m.Sensitivity.SenderClass)
				}
			}
			if got := byKey(count(t, over("SearchSummary"), account, `{}`, "sender_class").Groups)["normal"]; got != 1 {
				t.Errorf("the normal senders' group counts %d messages, want the one normal message", got)
			}
			if !honour {
				return
			}
			// A message from a normal domain the call did not classify, committed before the search's
			// statement, is outside its snapshot, so a search for restricted senders never selects it.
			for _, m := range searched(t, overFrom("SearchPage", "fresh.example"), account, `{"sender_class":"restricted"}`, "") {
				if m.Sensitivity.SenderClass != "restricted" {
					t.Errorf("a search for restricted senders served %s, whose sender is %s", m.MessageID, m.Sensitivity.SenderClass)
				}
			}
			out := count(t, over("CountBySender"), account, `{}`, "sender")
			var sum int64
			for _, g := range out.Groups {
				sum += g.Messages
			}
			if sum != out.Summary.Messages {
				t.Errorf("a count's groups hold %d messages and its summary %d", sum, out.Summary.Messages)
			}
		})
	}
}
