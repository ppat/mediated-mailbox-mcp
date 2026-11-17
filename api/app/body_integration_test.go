//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/process/connections"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/fixture"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// stored is one message as the test writes it to the index and to the provider fake. The index row
// carries what the gate decides from, and the fake the body, snippet and filenames only the provider
// holds (ADR-0016).
type stored struct {
	id, from, scan string
	flags, rules   []string
	text, html     string
	snippet        string
	names          []string
	// absent leaves the message out of the fake, as one the provider no longer holds.
	absent bool
}

// providerTimeout is the provider timeout the harness's API server runs with, short so a provider that
// does not answer fails the request quickly.
const providerTimeout = 300 * time.Millisecond

// leaseWait is the deadline the API server puts on a body request's waits for its leases, counted
// from the request's arrival (ADR-0130). The test of it waits for it in full, so it proves the
// deadline the API server ships with.
const leaseWait = 45 * time.Second

// The links and the code the fixtures below carry.
const (
	loginLink = "https://accounts.example/reset?token=Zq8XvB3nT1kLm4Pw9sYd2Rf6" // gitleaks:allow, a synthetic value
	codeName  = "Your verification code 482913.txt"
)

// corpus is the test account's mail. Every message's snippet, filenames and body carry marker text,
// so a search of a response or an audit row finds any of them that reached it.
func corpus() []stored {
	news := fixture.Newsletter()
	bank := fixture.Bank()
	return []stored{
		{
			id: "m-news", from: news.FromAddress, scan: "scanned",
			html: "<p>" + marker.Body("newsbody") + `</p><p>Read <a href="https://evil.example/steal">https://bank.example/login</a></p>` +
				`<img src="https://t.example/pixel.gif"><script>alert(1)</script>`,
			text: marker.Body("newsbody"), snippet: marker.Body("newssnippet") + " <img src=https://t.example/s.gif>",
			names: []string{"minutes_" + marker.Field("newsfile") + ".pdf"},
		},
		{
			id: "m-plain", from: news.FromAddress, scan: "scanned",
			text: marker.Body("plainbody") + "\n<img src=https://t.example/p.gif>\n![x](https://t.example/q.gif)\n" +
				"[Your bank](https://evil.example/login)\n```\n# not a heading\n",
			snippet: marker.Body("plainsnippet"),
		},
		{
			id: "m-bank", from: bank.FromAddress, scan: "scanned",
			text: bank.Body, snippet: marker.Body("banksnippet"), names: bank.Names(),
		},
		{
			id: "m-pending", from: news.FromAddress, scan: "pending",
			text: marker.Body("pendingbody"), snippet: marker.Body("pendingsnippet"), names: []string{marker.Field("pendingfile") + ".pdf"},
		},
		{
			id: "m-flagged", from: news.FromAddress, scan: "scanned", flags: []string{"login_link"}, rules: []string{"link.query_token"},
			text: marker.Body("flaggedbody") + " " + loginLink, snippet: marker.Body("flaggedsnippet"),
		},
		{
			id: "m-link", from: news.FromAddress, scan: "skipped_gate",
			text: fixture.LoginLink().Body, snippet: marker.Body("linksnippet"),
		},
		{
			id: "m-snippetlink", from: news.FromAddress, scan: "skipped_gate",
			text: marker.Body("snippetlinkbody"), snippet: "Reset your password: " + loginLink,
		},
		{
			id: "m-namecode", from: news.FromAddress, scan: "skipped_gate",
			text: marker.Body("namecodebody"), snippet: marker.Body("namecodesnippet"), names: []string{"report.pdf", codeName},
		},
		{
			id: "m-skipped", from: news.FromAddress, scan: "skipped_gate",
			text: marker.Body("skippedbody"), snippet: marker.Body("skippedsnippet"),
		},
		{
			id: "m-scannedname", from: news.FromAddress, scan: "scanned",
			text: marker.Body("scannednamebody"), snippet: marker.Body("scannednamesnippet"), names: []string{codeName},
		},
		{
			id: "m-oldrestricted", from: news.FromAddress, scan: "skipped_restricted",
			text: marker.Body("oldrestrictedbody"), snippet: marker.Body("oldrestrictedsnippet"),
		},
		{id: "m-other", from: "someone@other.example", scan: "scanned", text: marker.Body("otherbody")},
		{id: "m-gone", from: news.FromAddress, scan: "scanned", absent: true},
	}
}

// harness is one account served by the API server, started through its spec as its main.go starts
// it, over the provider fake, with the bank's domain restricted by a base rule.
type harness struct {
	conn     *pgx.Conn
	keys     keyPair
	account  string
	revision string
	args     []string
	srv      *server
	fake     *fake.Fake

	mu sync.Mutex
	// fakes are the mailboxes of the accounts the provider holds, the harness's account's among them.
	fakes map[string]*fake.Fake
	// calls counts the provider calls every mailbox the connections built received.
	calls int
	// tokens are the credentials of the tokens each mailbox was built over, in order, made the tokens
	// themselves, and clients the clients they were made with.
	tokens  []string
	made    []*fake.Token
	clients []mail.OAuthClient
	// refuse is the credential whose calls the provider refuses, and outage fails every call.
	refuse string
	outage bool
	// hang holds each call this long before it reaches the fake, as a provider that does not answer.
	hang time.Duration
	// rotate maps a credential to the one a refresh of it rotates to, which a token holding it holds
	// from its first call on, as a provider that rotates the credential at a refresh does.
	rotate map[string]string
	// attempt is recorded on each token at each call when set, as the token's latest attempt at the
	// provider's authentication endpoint.
	attempt *mail.AuthAttempt
	// atEnd is called when a unit of work's end reads its token's latest attempt, between its read of
	// the token's credential and its write-back.
	atEnd func()
}

// options are how a test's harness differs from the default one.
type options struct {
	// interval is the API server's reload interval, an hour unless set, so no reload runs after the
	// first unless the test asks for them.
	interval time.Duration
	// before changes the database before the API server starts, so its first reload reads the change.
	before func(t *testing.T, h *harness)
}

// newHarness serves one connected account holding corpus.
func newHarness(t *testing.T, opts ...options) *harness {
	t.Helper()
	o := options{interval: time.Hour}
	for _, given := range opts {
		if given.interval != 0 {
			o.interval = given.interval
		}
		if given.before != nil {
			o.before = given.before
		}
	}
	conn := superuser(t)
	reset(t, conn)
	h := &harness{conn: conn, keys: newKeyPair(t), account: newAccounts(t, conn)[0], fakes: map[string]*fake.Fake{}, rotate: map[string]string{}}
	connect(t, conn, h.keys, h.account)
	client(t, conn, h.keys, household, "client-id", "client-secret")
	through(t, conn, h.account, household)
	must(t, conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES (NULL, 'base.bank', 'restricted', ARRAY['bank.example'], 'operator', 'test')`)
	h.fake = h.hold(t, h.account)
	if o.before != nil {
		o.before(t, h)
	}
	h.args = apiFlags(t, h.keys, o.interval, providerTimeout)
	h.revision = scannerRevision(t, h.args)
	h.start(t)
	return h
}

// hold writes corpus to the account's index and to a provider fake of its own, and returns the fake.
func (h *harness) hold(t *testing.T, account string) *fake.Fake {
	t.Helper()
	var held []fake.Message
	for i, m := range corpus() {
		domain := m.from[strings.IndexByte(m.from, '@')+1:]
		must(t, h.conn, `INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments,
			scan_state, content_flags, rule_ids) VALUES ($1, $2, $2, $3, $4, now(), $5, $6, $7, $8)`,
			account, m.id, m.from, domain, len(m.names) > 0, m.scan, orEmpty(m.flags), orEmpty(m.rules))
		if m.absent {
			continue
		}
		held = append(held, fake.Message{
			Metadata: mail.MessageMetadata{
				ID: m.id, ThreadID: m.id, From: mail.Address{Email: m.from}, Date: mail.UnixMilli(1_700_000_000_000 + int64(i)),
				Snippet: m.snippet, AttachmentNames: m.names, HasAttachments: len(m.names) > 0,
			},
			Body: mail.MessageBody{Text: m.text, HTML: m.html},
		})
	}
	f, err := fake.New(fake.Config{Account: account}, held...)
	if err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	h.fakes[account] = f
	h.mu.Unlock()
	return f
}

// start starts the API server over the harness's adapters and waits until it is ready, its Session's
// first reload done.
func (h *harness) start(t *testing.T) {
	t.Helper()
	h.srv = start(t, h.args, connections.WithAdapters(fake.Adapters(h.mailbox, gmail)))
	h.srv.awaitReady(t, 15*time.Second)
}

// restart stops the API server and starts another over the same database, as a restart of its
// process does, so what the second one reads is what the first left stored.
func (h *harness) restart(t *testing.T) {
	t.Helper()
	h.srv.stop()
	h.start(t)
}

// orEmpty returns s, or an empty list for nil, so a text array column is written as '{}'.
func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// mailbox is the harness's mailbox for an account over a token the connections made. Every call it
// receives is counted, held for the harness's hang, failed on the provider's side during an outage,
// and refused as a credential refusal while the token holds the refused credential. A token whose
// credential the harness rotates holds the rotated one from its first call on.
func (h *harness) mailbox(account string, token *fake.Token) (*fake.Throttled, error) {
	h.mu.Lock()
	h.tokens = append(h.tokens, string(token.Credential()))
	h.made = append(h.made, token)
	h.clients = append(h.clients, token.Client())
	f := h.fakes[account]
	if h.atEnd != nil {
		token.AtAttemptRead(h.atEnd)
	}
	h.mu.Unlock()
	if f == nil {
		return nil, fmt.Errorf("the provider holds no mailbox for %s", account)
	}
	return fake.Throttle(f, func(fake.Call) error {
		h.mu.Lock()
		h.calls++
		hang := h.hang
		h.mu.Unlock()
		time.Sleep(hang)
		h.mu.Lock()
		defer h.mu.Unlock()
		if rotated, ok := h.rotate[string(token.Credential())]; ok {
			token.Rotate([]byte(rotated))
		}
		if h.attempt != nil {
			token.Attempted(*h.attempt)
		}
		switch {
		case h.outage:
			return fmt.Errorf("the fake's outage: %w", mail.ErrProvider)
		case h.refuse != "" && string(token.Credential()) == h.refuse:
			return fmt.Errorf("the fake refuses %s: %w", token.Credential(), mail.ErrAuthentication)
		}
		return nil
	}, time.Now), nil
}

// providerCalls returns how many calls the provider has received.
func (h *harness) providerCalls() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.calls
}

// built returns the credentials of the tokens each mailbox was built over, in order.
func (h *harness) built() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.tokens)
}

// body is the body operation's result.
type body struct {
	AccountID       string   `json:"account_id"`
	MessageID       string   `json:"message_id"`
	Released        bool     `json:"released"`
	Reason          string   `json:"reason"`
	Note            *string  `json:"note"`
	Body            *string  `json:"body"`
	Snippet         *string  `json:"snippet"`
	AttachmentNames []string `json:"attachment_names"`
}

// bodyPath is the API root's path of a message's body.
func (h *harness) bodyPath(account, id string) string {
	return "/api/accounts/" + account + "/messages/" + id + "/body"
}

// request asks the API root for a message's body and returns the result with its text, failing the
// test when the request fails.
func (h *harness) request(t *testing.T, id string) (body, string) {
	t.Helper()
	status, out := h.srv.api(t, h.bodyPath(h.account, id))
	if status != http.StatusOK {
		t.Fatalf("get_message_body %s answered %d %s", id, status, out)
	}
	var b body
	if err := json.Unmarshal([]byte(out), &b); err != nil {
		t.Fatal(err)
	}
	return b, out
}

// failed asks the API root for a message's body of the account and returns the origin of its failure,
// or "" when the request did not fail.
func (h *harness) failed(t *testing.T, account, id string) string {
	t.Helper()
	status, out := h.srv.api(t, h.bodyPath(account, id))
	if status == http.StatusOK {
		return ""
	}
	var content struct {
		Error struct {
			Origin string `json:"origin"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &content); err != nil {
		t.Fatalf("a failed request answered %d %s", status, out)
	}
	return content.Error.Origin
}

// auditRow is one audit row as the test reads it back, with every column a body decision's row
// records (ADR-0016). A NULL column reads as nil.
type auditRow struct {
	Actor, Action, Message     string
	Stage, Reason, SenderClass *string
	ClassRule, ClassScope      *string
	ContentFlags, ContentRules []string
	ServeRules                 []string
	ScanState                  *string
	ScannerVersion             *int32
	ScannerRevision            *string
}

// audit returns the account's audit rows, oldest first.
func (h *harness) audit(t *testing.T) []auditRow {
	t.Helper()
	rows, err := h.conn.Query(t.Context(), `SELECT actor, action, message_id, stage, reason, sender_class, class_rule_id,
		class_rule_scope, content_flags, content_rule_ids, serve_rule_ids, scan_state, scanner_version, scanner_revision
		FROM audit_log WHERE account_id = $1 ORDER BY id`, h.account)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (auditRow, error) {
		var a auditRow
		err := r.Scan(&a.Actor, &a.Action, &a.Message, &a.Stage, &a.Reason, &a.SenderClass, &a.ClassRule, &a.ClassScope,
			&a.ContentFlags, &a.ContentRules, &a.ServeRules, &a.ScanState, &a.ScannerVersion, &a.ScannerRevision)
		return a, err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// decided is the audit row a body decision writes when no policy rule set its sender class, no content
// flag is stored and the serve-time pattern check did not run. The methods below add each of those.
func decided(action, message, stage, reason, class, scan string) auditRow {
	return auditRow{
		Actor: "client", Action: action, Message: message, Stage: &stage, Reason: &reason, SenderClass: &class,
		ContentFlags: []string{}, ContentRules: []string{}, ScanState: &scan,
	}
}

// classRule records the policy rule that set the sender class, in its scope.
func (a auditRow) classRule(id, scope string) auditRow {
	a.ClassRule, a.ClassScope = &id, &scope
	return a
}

// flagged records the stored content flags with the content rules that set them.
func (a auditRow) flagged(flags, rules []string) auditRow {
	a.ContentFlags, a.ContentRules = flags, rules
	return a
}

// checked records the serve-time pattern check's run under the API server's scanner, whose revision
// is revision, with the rules it matched.
func (a auditRow) checked(revision string, rules []string) auditRow {
	version := int32(scan.Version)
	a.ServeRules, a.ScannerVersion, a.ScannerRevision = rules, &version, &revision
	return a
}

// The series of the body operation the tests read, as the API server names them.
const (
	bodiesServedName      = "mediated_mailbox_api_bodies_served_total"
	bodiesDeniedName      = "mediated_mailbox_api_bodies_denied_total"
	operationDurationName = "mediated_mailbox_api_operation_duration_seconds"
	processingName        = "mediated_mailbox_body_processing_duration_seconds"
	reloadFailedName      = "mediated_mailbox_policyload_reload_failed"
	accountRateName       = "mediated_mailbox_ratelimit_account_rate"
)

// counter returns the value of the API server's series name with the labels, beside its component
// label, or for a histogram how many it observed, or 0 when it serves none.
func (h *harness) counter(t *testing.T, name string, labels map[string]string) float64 {
	t.Helper()
	with := map[string]string{"component": "api"}
	for k, v := range labels {
		with[k] = v
	}
	return h.srv.metric(t, name, with)
}

// The gate decides every request from the index under the policy in force, and a denial never reaches
// the provider (ADR-0002). D3's parts of VERIFICATIONS' rows for requesting a restricted body with the
// provider never contacted, and for an ex-restricted, still-unscanned message whose body would pass
// the serve-time pattern check. Each denial carries the gate's reason, the pending one told apart from
// a restricted sender, and no snippet, filename or body text of the message reaches the response or
// the audit row (ADR-0001). Every denial writes one DENY_BODY row with every column its decision rests
// on, one case for each reason the gate gives and for each way a restricted sender's row differs, and
// is counted as a gate denial (ADR-0016, ADR-0036). A flag on a message not yet scanned, an invalid
// stored state, cannot be stored, since the schema's check refuses it (ADR-0016), so its reason is
// core/redact's tests'. A sender the classifier cannot read is restricted with no rule behind it, and a restricted sender's
// stored flag keeps its content rules apart from the policy rule.
func TestTheGateDeniesFromTheIndexWithoutReachingTheProvider(t *testing.T) {
	h := newHarness(t)
	for _, m := range []struct{ id, from, domain, scan, flag, rule string }{
		{"m-unreadable", "someone@com", "com", "scanned", "", ""},
		{"m-bankcode", "codes@bank.example", "bank.example", "scanned", "mfa_code", "mfa.trigger_window"},
	} {
		flags, rules := []string{}, []string{}
		if m.flag != "" {
			flags, rules = []string{m.flag}, []string{m.rule}
		}
		must(t, h.conn, `INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments,
			scan_state, content_flags, rule_ids) VALUES ($1, $2, $2, $3, $4, now(), false, $5, $6, $7)`,
			h.account, m.id, m.from, m.domain, m.scan, flags, rules)
	}
	cases := []struct {
		id, reason string
		row        auditRow
	}{
		{"m-bank", "restricted_sender", decided("DENY_BODY", "m-bank", "gate", "restricted_sender", "restricted", "scanned").classRule("base.bank", "base")},
		{"m-pending", "pending_scan", decided("DENY_BODY", "m-pending", "gate", "pending_scan", "normal", "pending")},
		{"m-flagged", "content_flagged", decided("DENY_BODY", "m-flagged", "gate", "content_flagged", "normal", "scanned").flagged([]string{"login_link"}, []string{"link.query_token"})},
		{"m-oldrestricted", "skipped_restricted", decided("DENY_BODY", "m-oldrestricted", "gate", "skipped_restricted", "normal", "skipped_restricted")},
		{"m-unreadable", "restricted_sender", decided("DENY_BODY", "m-unreadable", "gate", "restricted_sender", "restricted", "scanned")},
		{"m-bankcode", "restricted_sender", decided("DENY_BODY", "m-bankcode", "gate", "restricted_sender", "restricted", "scanned").classRule("base.bank", "base").flagged([]string{"mfa_code"}, []string{"mfa.trigger_window"})},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			got, out := h.request(t, c.id)
			want := body{AccountID: h.account, MessageID: c.id, Reason: c.reason}
			if diff := cmp.Diff(want, got, compare.Options, compareNoteSet()); diff != "" || got.Note == nil {
				t.Errorf("the denial (-want +got):\n%s", diff)
			}
			if strings.Contains(out, marker.BodyPrefix) || strings.Contains(out, ".pdf") {
				t.Errorf("the denial carries message text: %s", out)
			}
			if c.reason == "pending_scan" && !strings.Contains(*got.Note, "backlog") {
				t.Errorf("the pending denial's note does not read as backlog: %s", *got.Note)
			}
			// A sender left unscanned as restricted may have been delisted since, so the note must not
			// say the policy lists it (ADR-0037).
			if c.reason == "skipped_restricted" && strings.Contains(*got.Note, "is on the sensitive-sender policy") {
				t.Errorf("the note for a message skipped as restricted says the policy lists its sender: %s", *got.Note)
			}
		})
	}
	if n := h.providerCalls(); n != 0 {
		t.Errorf("the provider received %d calls for bodies the gate denied", n)
	}
	var want []auditRow
	for _, c := range cases {
		want = append(want, c.row)
	}
	if diff := cmp.Diff(want, h.audit(t), compare.Options); diff != "" {
		t.Errorf("the gate denials' audit rows (-want +got):\n%s", diff)
	}
	counts := map[string]float64{}
	for _, c := range cases {
		counts[c.reason]++
	}
	for reason, want := range counts {
		if got := h.counter(t, bodiesDeniedName, map[string]string{"account": h.account, "stage": "gate", "reason": reason}); got != want {
			t.Errorf("the gate denials for %q count %v, want %v", reason, got, want)
		}
	}
	if got := h.counter(t, bodiesServedName, map[string]string{"account": h.account}); got != 0 {
		t.Errorf("the denials counted %v serves", got)
	}
}

// compareNoteSet leaves the note out of a comparison, since a denial's wording is the service layer's
// and the tests check what it must say.
func compareNoteSet() cmp.Option {
	return cmp.FilterPath(func(p cmp.Path) bool { return p.Last().String() == ".Note" }, cmp.Ignore())
}

// A released message's body is served as clean Markdown inside the delimiters with its snippet and
// filenames, each written to its audit row and counted as a serve (ADR-0002, ADR-0036). D3's part of
// VERIFICATIONS' row for serving raw HTML with a disagreeing link and a remote image. The released
// HTML body holds no markup and no image, and the link keeps its label beside its target. A body with
// no HTML part, the snippet and each filename are each one fenced code block showing the text
// exactly (ADR-0100), a form whose rendering the conversion library's own property test proves.
func TestAReleasedBodyIsCleanMarkdown(t *testing.T) {
	h := newHarness(t)
	news, _ := h.request(t, "m-news")
	if !news.Released || news.Reason != "released" || news.Body == nil || news.Note != nil {
		t.Fatalf("the newsletter was not released: %+v", news)
	}
	enclosed := between(t, *news.Body)
	if !strings.Contains(enclosed, marker.Body("newsbody")) || !strings.Contains(enclosed, "[https://bank.example/login](https://evil.example/steal)") {
		t.Errorf("the released body lost its text or its link's disagreement:\n%s", enclosed)
	}
	for _, banned := range []string{"<", "![", "pixel.gif", "alert(1)"} {
		if strings.Contains(*news.Body, banned) {
			t.Errorf("the released body holds %q:\n%s", banned, *news.Body)
		}
	}
	m := corpus()[0]
	if news.Snippet == nil {
		t.Fatal("the newsletter's snippet was not released")
	}
	literal(t, *news.Snippet, m.snippet, 3)
	if len(news.AttachmentNames) != 1 {
		t.Fatalf("the filenames released are %v", news.AttachmentNames)
	}
	literal(t, news.AttachmentNames[0], m.names[0], 3)

	plain, _ := h.request(t, "m-plain")
	if !plain.Released || plain.Body == nil {
		t.Fatalf("the plain-text message was not released: %+v", plain)
	}
	literal(t, between(t, *plain.Body), corpus()[1].text, 4)
	if plain.AttachmentNames == nil || len(plain.AttachmentNames) != 0 {
		t.Errorf("a message without attachments released the filenames %v", plain.AttachmentNames)
	}
	if n := h.providerCalls(); n != 4 {
		t.Errorf("two releases made %d provider calls, want a body and a metadata call each", n)
	}
	// Each release is the serve stage's, and the serve-time pattern check ran over its filenames,
	// even none, under the harness's scanner and matched nothing.
	want := []auditRow{
		decided("READ_BODY", "m-news", "serve", "released", "normal", "scanned").checked(h.revision, []string{}),
		decided("READ_BODY", "m-plain", "serve", "released", "normal", "scanned").checked(h.revision, []string{}),
	}
	if diff := cmp.Diff(want, h.audit(t), compare.Options); diff != "" {
		t.Errorf("the releases' audit rows (-want +got):\n%s", diff)
	}
	if got := h.counter(t, bodiesServedName, map[string]string{"account": h.account}); got != 2 {
		t.Errorf("two serves counted %v", got)
	}
	// Both requests are timed as the body operation, and so is each body's rendering, the HTML body's
	// conversion and the plain-text body's literal form (ADR-0125, ADR-0131).
	if got := h.counter(t, operationDurationName, map[string]string{"operation": "get_message_body"}); got != 2 {
		t.Errorf("%v body requests timed, want 2", got)
	}
	if got := h.counter(t, processingName, map[string]string{"step": "convert"}); got != 2 {
		t.Errorf("%v renderings timed, want one for each body", got)
	}
}

// between returns the text a released body encloses in its delimiters.
func between(t *testing.T, released string) string {
	t.Helper()
	const opening, closing = "----- BEGIN UNTRUSTED CONTENT -----\n", "\n----- END UNTRUSTED CONTENT -----"
	start := strings.Index(released, opening)
	if start < 0 || !strings.HasSuffix(released, closing) {
		t.Fatalf("the released body is not wrapped:\n%s", released)
	}
	return strings.TrimSuffix(released[start+len(opening):], closing)
}

// literal checks that md is want inside a fence of fence backticks, the form ADR-0100 gives message
// text with no HTML form, written out here rather than read from the conversion library.
func literal(t *testing.T, md, want string, fence int) {
	t.Helper()
	if !strings.HasSuffix(want, "\n") {
		want += "\n"
	}
	marks := strings.Repeat("`", fence)
	if wrapped := marks + "\n" + want + marks; md != wrapped {
		t.Errorf("released %q, want %q", md, wrapped)
	}
}

// ADR-0002's serve-time pattern check, D3's part of VERIFICATIONS' rows for serving a gate-skipped
// body with a login link, a gate-skipped message whose snippet or filename holds a secret, and a
// scanned message whose filename holds one, since no scanner reads a filename. Each is
// denied at serve with nothing of its body, snippet or filenames in the response, writes one DENY_BODY
// row naming the pattern rules that matched, and is counted as a serve-time denial, never a gate one.
// A clean gate-skipped message is released.
func TestTheServeTimeCheckReadsEverythingReleased(t *testing.T) {
	h := newHarness(t)
	for _, id := range []string{"m-link", "m-snippetlink", "m-namecode", "m-scannedname"} {
		got, out := h.request(t, id)
		if got.Released || got.Reason != "serve_time_check_matched" || got.Body != nil || got.Snippet != nil || got.AttachmentNames != nil {
			t.Errorf("%s answered %s", id, out)
		}
		if strings.Contains(out, marker.BodyPrefix) || strings.Contains(out, "482913") || strings.Contains(out, "Zq8XvB3n") {
			t.Errorf("%s's denial carries its text: %s", id, out)
		}
	}
	if got, out := h.request(t, "m-skipped"); !got.Released || got.Snippet == nil {
		t.Errorf("a clean gate-skipped message answered %s", out)
	}
	// Each denial names the pattern rules that matched apart from the content rules, which none of
	// these messages stores.
	matched := func(id, scan string, rules ...string) auditRow {
		return decided("DENY_BODY", id, "serve", "serve_time_check_matched", "normal", scan).checked(h.revision, rules)
	}
	want := []auditRow{
		matched("m-link", "skipped_gate", "link.path_token", "link.query_token"),
		matched("m-snippetlink", "skipped_gate", "link.query_token"),
		matched("m-namecode", "skipped_gate", "mfa.trigger_window"),
		matched("m-scannedname", "scanned", "mfa.trigger_window"),
		decided("READ_BODY", "m-skipped", "serve", "released", "normal", "skipped_gate").checked(h.revision, []string{}),
	}
	if diff := cmp.Diff(want, h.audit(t), compare.Options); diff != "" {
		t.Errorf("the serve-time check's audit rows (-want +got):\n%s", diff)
	}
	labels := map[string]string{"account": h.account, "stage": "serve", "reason": "serve_time_check_matched"}
	if got := h.counter(t, bodiesDeniedName, labels); got != 4 {
		t.Errorf("the serve-time denials count %v, want 4", got)
	}
	labels["stage"] = "gate"
	if got := h.counter(t, bodiesDeniedName, labels); got != 0 {
		t.Errorf("serve-time denials were counted as gate denials %v times", got)
	}
}

// D3's part of VERIFICATIONS' row for serving raw HTML with a remote image, observed at the network
// layer. The message's images and its styled background point at a listener the test owns, which
// receives no request while the body is fetched, converted and served, and the released Markdown holds
// nothing that points at the listener, so no client rendering it can fetch from it (ADR-0036).
func TestAServedBodyFetchesNoImage(t *testing.T) {
	h := newHarness(t)
	var mu sync.Mutex
	var fetched []string
	listener := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		fetched = append(fetched, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(listener.Close)
	html := "<p>" + marker.Body("pixelbody") + `</p><img src="` + listener.URL + `/pixel.gif" width="1" height="1">` +
		`<picture><source srcset="` + listener.URL + `/hero.webp"><img src="` + listener.URL + `/hero.png"></picture>` +
		`<div style="background-image:url(` + listener.URL + `/bg.png)">styled</div>`
	if err := h.fake.Deliver(fake.Message{
		Metadata: mail.MessageMetadata{ID: "m-pixel", ThreadID: "m-pixel", From: mail.Address{Email: "a@newsletter.example"}},
		Body:     mail.MessageBody{HTML: html, Text: "x"},
	}); err != nil {
		t.Fatal(err)
	}
	must(t, h.conn, `INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments,
		scan_state) VALUES ($1, 'm-pixel', 'm-pixel', 'a@newsletter.example', 'newsletter.example', now(), false, 'scanned')`, h.account)
	got, out := h.request(t, "m-pixel")
	if !got.Released || got.Body == nil || !strings.Contains(*got.Body, marker.Body("pixelbody")) {
		t.Fatalf("the message was not released: %s", out)
	}
	if strings.Contains(out, listener.URL) || strings.Contains(out, "![") {
		t.Errorf("the released body points at the listener: %s", out)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(fetched) != 0 {
		t.Errorf("serving the body fetched %v", fetched)
	}
}

// A body the conversion refuses is denied at serve, audited and counted (ADR-0036). Its HTML part is
// within the bound and nests more elements than the parser accepts.
func TestABodyTheConversionRefusesIsDenied(t *testing.T) {
	h := newHarness(t)
	nested := strings.Repeat("<div>", 600) + marker.Body("nested") + strings.Repeat("</div>", 600)
	if err := h.fake.Deliver(fake.Message{
		Metadata: mail.MessageMetadata{ID: "m-nested", ThreadID: "m-nested", From: mail.Address{Email: "a@newsletter.example"}},
		Body:     mail.MessageBody{HTML: nested, Text: "x"},
	}); err != nil {
		t.Fatal(err)
	}
	must(t, h.conn, `INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments,
		scan_state) VALUES ($1, 'm-nested', 'm-nested', 'a@newsletter.example', 'newsletter.example', now(), false, 'scanned')`, h.account)
	got, _ := h.request(t, "m-nested")
	if got.Released || got.Reason != "conversion_refused" || got.Body != nil {
		t.Errorf("a body the conversion refuses answered %+v", got)
	}
	// The conversion refused the body before the serve-time pattern check could read it, so the row
	// names no check, which tells it apart from the check's own denial.
	want := []auditRow{decided("DENY_BODY", "m-nested", "serve", "conversion_refused", "normal", "scanned")}
	if diff := cmp.Diff(want, h.audit(t), compare.Options); diff != "" {
		t.Errorf("the refusal's audit row (-want +got):\n%s", diff)
	}
	if got := h.counter(t, bodiesDeniedName, map[string]string{"account": h.account, "stage": "serve", "reason": "conversion_refused"}); got != 1 {
		t.Errorf("the refusal was counted %v times", got)
	}
}

// rate returns the account's current rate as the rate-state series on the metrics endpoint reports
// it.
func (h *harness) rate(t *testing.T) float64 {
	t.Helper()
	for labels, v := range h.srv.series(t, accountRateName) {
		if strings.Contains(labels, `account="`+h.account+`"`) {
			return v
		}
	}
	t.Fatal("the rate-state series reports no rate for the account")
	return 0
}

// reloadFailed returns the values of the policies' reload-failure series on the metrics endpoint, which
// the reload-failure alarm reads (ADR-0041, ADR-0077).
func (h *harness) reloadFailed(t *testing.T) []float64 {
	t.Helper()
	var values []float64
	for _, v := range h.srv.series(t, reloadFailedName) {
		values = append(values, v)
	}
	return values
}

func TestABodyPastTheBoundIsDeniedAsRefused(t *testing.T) {
	h := newHarness(t)
	over := strings.Repeat("a", mail.MaxBodyBytes)
	for _, m := range []fake.Message{
		{Metadata: mail.MessageMetadata{ID: "m-huge", ThreadID: "m-huge", From: mail.Address{Email: "a@newsletter.example"}}, Body: mail.MessageBody{HTML: "<p>" + over + "</p>", Text: "x"}},
		{Metadata: mail.MessageMetadata{ID: "m-long", ThreadID: "m-long", From: mail.Address{Email: "a@newsletter.example"}}, Body: mail.MessageBody{Text: over + "a"}},
		{Metadata: mail.MessageMetadata{ID: "m-apart", ThreadID: "m-apart", From: mail.Address{Email: "a@newsletter.example"}}, Body: mail.MessageBody{HTML: "<p>" + marker.Body("apart") + "</p>"}, HeldApart: true},
	} {
		if err := h.fake.Deliver(m); err != nil {
			t.Fatal(err)
		}
		must(t, h.conn, `INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments,
			scan_state) VALUES ($1, $2, $2, 'a@newsletter.example', 'newsletter.example', now(), false, 'scanned')`, h.account, m.Metadata.ID)
	}
	start := h.providerCalls()
	if got, _ := h.request(t, "m-news"); !got.Released {
		t.Fatalf("the newsletter was not released: %+v", got)
	}
	released, before := h.providerCalls()-start, h.rate(t)
	for _, id := range []string{"m-huge", "m-long"} {
		if got, _ := h.request(t, id); got.Released || got.Reason != "body_refused" || got.Body != nil || got.Note == nil {
			t.Errorf("a body past the bound answered %+v", got)
		}
	}
	if after := h.rate(t); after != before {
		t.Errorf("the refusals moved the account's rate from %v to %v", before, after)
	}
	want := []auditRow{
		decided("READ_BODY", "m-news", "serve", "released", "normal", "scanned").checked(h.revision, []string{}),
		decided("DENY_BODY", "m-huge", "serve", "body_refused", "normal", "scanned"),
		decided("DENY_BODY", "m-long", "serve", "body_refused", "normal", "scanned"),
	}
	if diff := cmp.Diff(want, h.audit(t), compare.Options); diff != "" {
		t.Errorf("the refusals' audit rows (-want +got):\n%s", diff)
	}
	if got := h.counter(t, bodiesDeniedName, map[string]string{"account": h.account, "stage": "serve", "reason": "body_refused"}); got != 2 {
		t.Errorf("the refusals were counted %v times, want 2", got)
	}

	calls := h.providerCalls()
	if got, _ := h.request(t, "m-apart"); !got.Released || got.Body == nil || !strings.Contains(*got.Body, marker.Body("apart")) {
		t.Errorf("a part held apart within the bound answered %+v", got)
	}
	if n := h.providerCalls() - calls; n != released+1 {
		t.Errorf("the part held apart took %d provider calls, want one more than the %d of a release", n, released)
	}

	h.mu.Lock()
	h.outage = true
	h.mu.Unlock()
	if origin := h.failed(t, h.account, "m-news"); origin == "" {
		t.Fatal("a request during the outage succeeded")
	}
	if after := h.rate(t); after >= before {
		t.Errorf("a provider failure left the account's rate at %v from %v, so the series shows no cut", after, before)
	}
}

// D3's part of VERIFICATIONS' row for requesting a body from a just-deny-listed domain with no re-sync
// in between, and the row for a load that fails. A rule written after a body was served denies the
// next request with no reload in between, because each request has the policy loaded before it
// decides (ADR-0099). A load that fails leaves the policy active before it governing, and raises the
// reload-failure alarm (ADR-0041).
func TestANewlyListedDomainIsDeniedOnTheNextCall(t *testing.T) {
	h := newHarness(t)
	if got, _ := h.request(t, "m-news"); !got.Released {
		t.Fatalf("the newsletter was not released before its domain was listed: %+v", got)
	}
	must(t, h.conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES (NULL, 'base.news', 'restricted', ARRAY['newsletter.example'], 'operator', 'test')`)
	calls := h.providerCalls()
	if got, _ := h.request(t, "m-news"); got.Released || got.Reason != "restricted_sender" {
		t.Errorf("the next call after the domain was listed answered %+v", got)
	}
	if h.providerCalls() != calls {
		t.Error("the denial reached the provider")
	}

	const policyRead = "SELECT ON policy_rules"
	revoke(t, h.conn, policyRead)
	must(t, h.conn, "DELETE FROM policy_rules WHERE rule_id = 'base.news'")
	if got, _ := h.request(t, "m-news"); got.Released || got.Reason != "restricted_sender" {
		t.Errorf("with the load failing, the request answered %+v, want the active policy's denial", got)
	}
	// A sender the active policy does not list is still released, so a failed load decides under
	// the active policy rather than denying every sender.
	if got, _ := h.request(t, "m-other"); !got.Released {
		t.Errorf("with the load failing, a sender the active policy does not list answered %+v, want it released", got)
	}
	if got := h.reloadFailed(t); !slices.Equal(got, []float64{1}) {
		t.Errorf("after a failed load the reload-failure series reads %v", got)
	}
	grantBack(t, h.conn, policyRead)
	if got, _ := h.request(t, "m-news"); !got.Released {
		t.Errorf("once the load succeeds again and the rule is gone, the request answered %+v", got)
	}
	if got := h.reloadFailed(t); !slices.Equal(got, []float64{0}) {
		t.Errorf("after a load that succeeded the reload-failure series reads %v", got)
	}
}

// D3's part of VERIFICATIONS' row for editing the policy while the accounts stay the same. A
// scheduled reload loads the policy every time, so a metadata read shows an edit without a body
// request (ADR-0099).
// D3's part of VERIFICATIONS' row for editing the policy while the accounts stay the same. A
// scheduled reload loads the policy every time, so a metadata read shows an edit without a body
// request (ADR-0099).
func TestEveryReloadLoadsThePolicy(t *testing.T) {
	h := newHarness(t, options{interval: 100 * time.Millisecond})
	read := func() string {
		status, out := h.srv.api(t, "/api/accounts/"+h.account+"/messages/m-news")
		if status != http.StatusOK {
			t.Fatalf("get_message answered %d %s", status, out)
		}
		var m struct {
			Sensitivity struct {
				SenderClass string `json:"sender_class"`
			} `json:"sensitivity"`
		}
		if err := json.Unmarshal([]byte(out), &m); err != nil {
			t.Fatal(err)
		}
		return m.Sensitivity.SenderClass
	}
	if got := read(); got != "normal" {
		t.Fatalf("before the edit the metadata reads %s", got)
	}
	must(t, h.conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES (NULL, 'base.news', 'restricted', ARRAY['newsletter.example'], 'operator', 'test')`)
	h.srv.reloaded(t)
	if got := read(); got != "restricted" {
		t.Errorf("after a reload with the same accounts the metadata reads %s, want restricted", got)
	}
}

// An audit write that fails releases nothing, and the request fails as the API server's (ADR-0002,
// ADR-0101). D3's part of VERIFICATIONS' row for the audit of every serve and denial.
func TestNoBodyIsReleasedWithoutItsAuditRow(t *testing.T) {
	h := newHarness(t)
	revoke(t, h.conn, "INSERT ON audit_log")
	if status, out := h.srv.api(t, h.bodyPath(h.account, "m-news")); status == http.StatusOK || strings.Contains(out, marker.Body("newsbody")) {
		t.Fatalf("a release whose audit write failed answered %d %s", status, out)
	}
	if origin := h.failed(t, h.account, "m-news"); origin != "api" {
		t.Errorf("the failed audit write is the %q's failure", origin)
	}
	if got := h.counter(t, bodiesServedName, map[string]string{"account": h.account}); got != 0 {
		t.Errorf("a body whose audit write failed was counted as served %v times", got)
	}
}

// D3's row for the class rule's scope. The base policy and the account each hold a rule named
// base.bank, the base one restricting the bank and the account's the newsletter, so each denial's
// audit row names the identifier with the scope of the rule that restricted its sender (ADR-0016,
// ADR-0110). The request loads the policy before it decides, so the account's rule binds with no
// reload (ADR-0099).
func TestADenialNamesTheScopeOfTheRuleBehindIt(t *testing.T) {
	h := newHarness(t)
	must(t, h.conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES ($1, 'base.bank', 'restricted', ARRAY['newsletter.example'], 'operator', 'test')`, h.account)
	for _, id := range []string{"m-bank", "m-news"} {
		if got, out := h.request(t, id); got.Released || got.Reason != "restricted_sender" {
			t.Fatalf("%s answered %s", id, out)
		}
	}
	want := []auditRow{
		decided("DENY_BODY", "m-bank", "gate", "restricted_sender", "restricted", "scanned").classRule("base.bank", "base"),
		decided("DENY_BODY", "m-news", "gate", "restricted_sender", "restricted", "scanned").classRule("base.bank", "account"),
	}
	if diff := cmp.Diff(want, h.audit(t), compare.Options); diff != "" {
		t.Errorf("the denials' audit rows (-want +got):\n%s", diff)
	}
}

// D3's row for a reason the audit row's check does not know. A check that refuses the released token
// stands in for a token the API server writes and the check was never given. The release's audit write is
// refused, so the request fails as the API server's, its body is never returned and nothing is counted as
// served or written (ADR-0016, ADR-0101).
// D3's row for a reason the audit row's check does not know. A check that refuses the released token
// stands in for a token the API server writes and the check was never given. The release's audit write is
// refused, so the request fails as the API server's, its body is never returned and nothing is counted as
// served or written (ADR-0016, ADR-0101).
func TestAReasonTheAuditCheckDoesNotKnowReleasesNothing(t *testing.T) {
	h := newHarness(t)
	must(t, h.conn, "ALTER TABLE audit_log ADD CONSTRAINT audit_log_reason_unknown CHECK (reason <> 'released')")
	url := postgres.URL(t)
	t.Cleanup(func() {
		ctx := context.Background()
		c, err := pgx.Connect(ctx, url)
		if err != nil {
			t.Error(err)
			return
		}
		defer func() {
			if err := c.Close(ctx); err != nil {
				t.Error(err)
			}
		}()
		if _, err := c.Exec(ctx, "ALTER TABLE audit_log DROP CONSTRAINT audit_log_reason_unknown"); err != nil {
			t.Error(err)
		}
	})
	if status, out := h.srv.api(t, h.bodyPath(h.account, "m-news")); status == http.StatusOK || strings.Contains(out, marker.Body("newsbody")) {
		t.Fatalf("a release whose audit row the check refused answered %d %s", status, out)
	}
	if origin := h.failed(t, h.account, "m-news"); origin != "api" {
		t.Errorf("the refused audit write is the %q's failure", origin)
	}
	if got := h.counter(t, bodiesServedName, map[string]string{"account": h.account}); got != 0 {
		t.Errorf("a body whose audit row was refused was counted as served %v times", got)
	}
	if rows := h.audit(t); len(rows) != 0 {
		t.Errorf("the refused write left the audit rows %+v", rows)
	}
}

// surfaceAnswer is what one root answered, its status and its content.
type surfaceAnswer struct {
	Status  int
	Content string
}

// both asks the API root and the MCP root for the same message's body, behind the bearer check, and
// returns what each answered. The MCP root's status is 200 for a result and 0 for a tool error.
func (h *harness) both(t *testing.T, id string) (apiAnswer, mcpAnswer surfaceAnswer) {
	t.Helper()
	status, out := h.srv.api(t, h.bodyPath(h.account, id))
	apiAnswer = surfaceAnswer{status, out}
	_, isError, content := h.srv.mcp(t, "get_message_body", map[string]string{"account_id": h.account, "message_id": id})
	status = http.StatusOK
	if isError {
		status = 0
	}
	return apiAnswer, surfaceAnswer{status, content}
}

// D3's part of VERIFICATIONS' rows for inducing one failure of each origin and for the two roots'
// shapes. A message the account does not hold is the client's failure, an audit write that fails the
// API server's, and a provider outage and a message the provider no longer holds the provider's. Each
// names its origin, the API root with the status the origin gives, and the MCP root's tool error
// carries the same content. A denial is a successful result on both, with the same content
// (ADR-0101).
func TestEachOriginIsToldApartOnBothRoots(t *testing.T) {
	h := newHarness(t)
	type want struct {
		status int
		origin string
	}
	check := func(name, id string, w want) {
		t.Helper()
		api, mcp := h.both(t, id)
		var content struct {
			Error struct {
				Origin string `json:"origin"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(api.Content), &content); err != nil {
			t.Fatalf("%s: the API root answered %s", name, api.Content)
		}
		if api.Status != w.status || content.Error.Origin != w.origin {
			t.Errorf("%s: the API root answered %d %s, want %d from %s", name, api.Status, api.Content, w.status, w.origin)
		}
		if mcp.Status != 0 || mcp.Content != strings.TrimSpace(api.Content) {
			t.Errorf("%s: the MCP root answered %+v, want a tool error carrying %s", name, mcp, api.Content)
		}
		if strings.Contains(api.Content, "fake") || strings.Contains(api.Content, marker.BodyPrefix) {
			t.Errorf("%s: the failure carries the provider's text or the message's: %s", name, api.Content)
		}
	}
	check("a message the account does not hold", "m-unknown", want{http.StatusBadRequest, "client"})
	check("a message the provider no longer holds", "m-gone", want{http.StatusBadGateway, "provider"})
	h.mu.Lock()
	h.outage = true
	h.mu.Unlock()
	check("a provider outage", "m-news", want{http.StatusBadGateway, "provider"})
	h.mu.Lock()
	h.outage = false
	h.mu.Unlock()
	revoke(t, h.conn, "INSERT ON audit_log")
	check("an audit write that fails", "m-news", want{http.StatusInternalServerError, "api"})
	grantBack(t, h.conn, "INSERT ON audit_log")

	api, mcp := h.both(t, "m-bank")
	if api.Status != http.StatusOK || mcp.Status != http.StatusOK || strings.TrimSpace(api.Content) != mcp.Content || !strings.Contains(api.Content, `"released":false`) {
		t.Errorf("a denial answered %+v on the API root and %+v on the MCP root", api, mcp)
	}
}

// D3's part of VERIFICATIONS' row for supplying a credential in the environment or a mounted file with
// the account's state row holding none. An account whose state row holds no sealed credential is not
// connected, whatever the environment holds, so its released message fails as the API server's and
// never reaches a provider (ADR-0080).
func TestAnAccountWithoutAStoredCredentialIsNotConnected(t *testing.T) {
	t.Setenv("MEDIATED_MAILBOX_REFRESH_TOKEN", "from-the-environment")
	t.Setenv("GMAIL_REFRESH_TOKEN", "from-the-environment")
	h := newHarness(t, options{before: func(t *testing.T, h *harness) {
		must(t, h.conn, "UPDATE account_state SET credential = NULL WHERE account_id = $1", h.account)
	}})
	if origin := h.failed(t, h.account, "m-news"); origin != "api" {
		t.Errorf("a body of an account with no stored credential answered as the %q's", origin)
	}
	if h.providerCalls() != 0 || len(h.built()) != 0 {
		t.Errorf("the provider was reached with the tokens %v", h.built())
	}
}

// D3's part of VERIFICATIONS' rows for making a token with the client's identifier and secret swapped
// and for an account's own client. The token a body request calls with is made from the client the
// account connects through, its identifier and its secret each in its own place, and the account's own
// stored credential. A second account connects through a second client of the one provider, so each
// account's token carries its own client and never the other's (ADR-0085, ADR-0106).
func TestTheCredentialsAreTheClientAndTheAccountsToken(t *testing.T) {
	var other string
	h := newHarness(t, options{before: func(t *testing.T, h *harness) {
		other = strings.TrimSuffix(h.account, "-a") + "-b"
		connect(t, h.conn, h.keys, other)
		client(t, h.conn, h.keys, "employer", "employer-id", "employer-secret")
		through(t, h.conn, other, "employer")
		h.hold(t, other)
	}})
	if got, out := h.request(t, "m-news"); !got.Released {
		t.Fatalf("the first account's body answered %s", out)
	}
	if status, out := h.srv.api(t, h.bodyPath(other, "m-news")); status != http.StatusOK || !strings.Contains(out, `"released":true`) {
		t.Fatalf("the second account's body answered %d %s", status, out)
	}
	type made struct{ ClientID, ClientSecret, Credential string }
	var got []made
	h.mu.Lock()
	for i, c := range h.clients {
		got = append(got, made{c.ID, string(c.Secret), h.tokens[i]})
	}
	h.mu.Unlock()
	want := []made{
		{"client-id", "client-secret", h.account + "-token"},
		{"employer-id", "employer-secret", other + "-token"},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("what each account's token was made from, in the order called (-want +got):\n%s", diff)
	}
}

// F6's part of VERIFICATIONS' row for an account of a provider that authenticates through an OAuth
// client served without its client, for the API server. A connected account of a provider that
// authenticates through a client, whose row names no client, is not connected, and its body request
// never reaches a provider (ADR-0106, ADR-0090).
func TestAnAccountNamingNoClientIsNotConnected(t *testing.T) {
	h := newHarness(t, options{before: func(t *testing.T, h *harness) {
		must(t, h.conn, "UPDATE accounts SET oauth_client = NULL WHERE account_id = $1", h.account)
	}})
	if origin := h.failed(t, h.account, "m-news"); origin != "api" {
		t.Errorf("a body of an account naming no client answered as the %q's", origin)
	}
	if h.providerCalls() != 0 || len(h.built()) != 0 {
		t.Errorf("the provider was reached with the tokens %v", h.built())
	}
}

// D3's part of VERIFICATIONS' row for reloading the account snapshot, for the API server's body fetch.
// A credential the provider refuses is read again from its row before the refusal is reported, so a
// re-authorization reaches the next refused call. When the row holds another credential, the call is
// made again over a token holding it, which later requests keep, so the access token it is issued
// serves them rather than each making a token of its own. When the row holds the refused one, the
// refusal is reported as the provider's (ADR-0090, ADR-0101).
func TestARefusedCredentialIsReadAgainBeforeTheRefusalIsReported(t *testing.T) {
	h := newHarness(t)
	h.mu.Lock()
	h.refuse = h.account + "-token"
	h.mu.Unlock()
	if origin := h.failed(t, h.account, "m-news"); origin != "provider" {
		t.Fatalf("a refusal with the stored credential unchanged answered as the %q's", origin)
	}

	store(t, h.conn, h.keys, h.account, h.account+"-renewed")
	if got, out := h.request(t, "m-news"); !got.Released {
		t.Fatalf("after the operator replaced the credential, the request answered %s", out)
	}
	if got, out := h.request(t, "m-plain"); !got.Released {
		t.Fatalf("the request after the re-authorization answered %s", out)
	}
	want := []string{h.account + "-token", h.account + "-token", h.account + "-renewed", h.account + "-renewed"}
	if diff := cmp.Diff(want, h.built(), compare.Options); diff != "" {
		t.Errorf("the credentials the mailboxes were built over (-want +got):\n%s", diff)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.made) == 4 && h.made[3] != h.made[2] {
		t.Error("the request after the re-authorization made a token of its own rather than keeping the one the retry made")
	}
}

// F6's part of VERIFICATIONS' row for an account's own client, for the API server's re-read. The account
// moves to another client while the API server serves it, its new client and new credential written in
// one transaction (ADR-0106). The provider refuses the old credential, and the call made again is
// made with a token over the new client and the new credential (ADR-0090).
func TestARereadAfterAMoveBuildsTheTokenFromTheNewClient(t *testing.T) {
	h := newHarness(t)
	if got, out := h.request(t, "m-plain"); !got.Released {
		t.Fatalf("the first request answered %s", out)
	}
	h.mu.Lock()
	h.refuse = h.account + "-token"
	h.mu.Unlock()
	client(t, h.conn, h.keys, "employer", "employer-id", "employer-secret")
	sealed, err := h.keys.public.Seal([]byte(h.account+"-moved"), seal.AccountCredential(h.account))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := h.conn.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(t.Context(), "UPDATE accounts SET oauth_client = 'employer' WHERE account_id = $1", h.account); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(t.Context(), "UPDATE account_state SET credential = $2 WHERE account_id = $1", h.account, sealed); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}

	if got, out := h.request(t, "m-news"); !got.Released {
		t.Fatalf("after the account moved, the request answered %s", out)
	}
	h.mu.Lock()
	last, made := h.clients[len(h.clients)-1], h.tokens[len(h.tokens)-1]
	h.mu.Unlock()
	got := []string{last.ID, string(last.Secret), made}
	if diff := cmp.Diff([]string{"employer-id", "employer-secret", h.account + "-moved"}, got, compare.Options); diff != "" {
		t.Errorf("what the retried call's token was made from (-want +got):\n%s", diff)
	}
}

// F6's part of VERIFICATIONS' row for an account served without its client, for the API server's
// re-read. The provider refuses the credential, and the account is then moved to a client whose
// secret does not open, or loses its credential. The re-read finds it not connected, so the refusal
// is reported after the one refused call and no token is made for a second (ADR-0106, ADR-0090).
func TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal(t *testing.T) {
	cases := map[string]func(t *testing.T, h *harness){
		"moved to a client that does not open": func(t *testing.T, h *harness) {
			sealed, err := h.keys.public.Seal([]byte("broken-secret"), seal.ClientSecret("another-row"))
			if err != nil {
				t.Fatal(err)
			}
			must(t, h.conn, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('broken', $1, 'broken-id', $2)", gmail.Name, sealed)
			credential, err := h.keys.public.Seal([]byte(h.account+"-moved"), seal.AccountCredential(h.account))
			if err != nil {
				t.Fatal(err)
			}
			tx, err := h.conn.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(t.Context(), "UPDATE accounts SET oauth_client = 'broken' WHERE account_id = $1", h.account); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(t.Context(), "UPDATE account_state SET credential = $2 WHERE account_id = $1", h.account, credential); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(t.Context()); err != nil {
				t.Fatal(err)
			}
		},
		"its credential removed": func(t *testing.T, h *harness) {
			must(t, h.conn, "UPDATE account_state SET credential = NULL WHERE account_id = $1", h.account)
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.mu.Lock()
			h.refuse = h.account + "-token"
			h.mu.Unlock()
			change(t, h)
			if origin := h.failed(t, h.account, "m-news"); origin != "provider" {
				t.Errorf("the request answered as the %q's, want the provider's refusal", origin)
			}
			if calls := h.providerCalls(); calls != 1 {
				t.Errorf("the provider received %d calls, want the one refused", calls)
			}
		})
	}
}

// storedAfterRestart restarts the API server, as a restart of its process does, has it serve m-plain,
// and returns the credential of the token its connections made from what the account's row holds.
func (h *harness) storedAfterRestart(t *testing.T) string {
	t.Helper()
	h.mu.Lock()
	h.outage, h.refuse, h.rotate, h.attempt, h.atEnd = false, "", map[string]string{}, nil, nil
	h.mu.Unlock()
	h.restart(t)
	if got, out := h.request(t, "m-plain"); !got.Released {
		t.Fatalf("after the restart the request answered %s", out)
	}
	built := h.built()
	return built[len(built)-1]
}

// D3's parts of VERIFICATIONS' rows for failing a rotated credential's write-back and for reloading
// the account snapshot. A body request whose write-back fails still answers, the failure is logged,
// and a reload keeps the rotated credential while the stored bytes are unchanged, so the next request
// uses it and its end writes it back once the write can land (ADR-0082, ADR-0090).
func TestARotationWhoseWriteBackFailedIsKept(t *testing.T) {
	h := newHarness(t, options{interval: 100 * time.Millisecond})
	rotated := h.account + "-rotated"
	h.mu.Lock()
	h.rotate[h.account+"-token"] = rotated
	h.mu.Unlock()
	const stateWrite = "UPDATE (credential) ON account_state"
	revoke(t, h.conn, stateWrite)
	if got, out := h.request(t, "m-news"); !got.Released {
		t.Fatalf("a request whose write-back failed answered %s", out)
	}
	if !strings.Contains(h.srv.logs.String(), "was not written back") {
		t.Errorf("the failed write-back was not logged:\n%s", h.srv.logs.String())
	}
	h.srv.reloaded(t)
	if got, _ := h.request(t, "m-plain"); !got.Released {
		t.Fatal("the request after the reload was not released")
	}
	if built := h.built(); built[len(built)-1] != rotated {
		t.Errorf("after the reload the request called with %q, want the rotated credential", built[len(built)-1])
	}
	grantBack(t, h.conn, stateWrite)
	if got, _ := h.request(t, "m-news"); !got.Released {
		t.Fatal("the request once the write can land was not released")
	}
	if got := h.storedAfterRestart(t); got != rotated {
		t.Errorf("after a restart the account holds %q, want the rotated credential", got)
	}
}

// D3's parts of VERIFICATIONS' rows for forcing a rotation, for failing its write-back and for failing
// a unit of work after an authentication attempt. Each body request is the API server's unit of work,
// and when it ends the token's current credential is written back, so a process started afterwards
// reads a rotated one, and the token's latest authentication attempt is recorded, a request that
// failed included (ADR-0082, ADR-0097). The provider fake's token plays the provider's part, rotating
// the credential and recording a failed attempt at the call.
func TestEachBodyRequestWritesBackAndRecordsTheAttempt(t *testing.T) {
	h := newHarness(t)
	h.mu.Lock()
	h.rotate[h.account+"-token"] = h.account + "-rotated"
	h.attempt = &mail.AuthAttempt{At: mail.UnixMilli(time.Now().UnixMilli()), Outcome: mail.AuthFailed}
	h.outage = true
	h.mu.Unlock()
	if origin := h.failed(t, h.account, "m-news"); origin == "" {
		t.Fatal("the request during the outage succeeded")
	}
	var outcome *string
	if err := h.conn.QueryRow(t.Context(), "SELECT last_auth_outcome FROM account_state WHERE account_id = $1", h.account).Scan(&outcome); err != nil {
		t.Fatal(err)
	}
	if outcome == nil || *outcome != string(mail.AuthFailed) {
		t.Errorf("the recorded outcome is %v, want failed", outcome)
	}
	if got := h.storedAfterRestart(t); got != h.account+"-rotated" {
		t.Errorf("after a restart the account holds %q, want the rotated credential", got)
	}
}

// D3's part of VERIFICATIONS' row for leasing body fetches in the interactive class. A released
// message's two provider calls, its body and its metadata, each take a lease in the interactive class,
// sized by the call's cost, one unit each under the provider fake's rate profile, and a body the gate
// denies takes none (ADR-0025, ADR-0002).
func TestABodyFetchSpendsFromTheInteractiveReservation(t *testing.T) {
	h := newHarness(t)
	grants := func() []string {
		rows, err := h.conn.Query(t.Context(), "SELECT class || ' ' || tokens FROM rate_grants WHERE account_id = $1 ORDER BY grant_id", h.account)
		if err != nil {
			t.Fatal(err)
		}
		out, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	if got, _ := h.request(t, "m-bank"); got.Released {
		t.Fatal("the restricted body was released")
	}
	if got := grants(); len(got) != 0 {
		t.Errorf("a body the gate denied took the leases %v", got)
	}
	if got, _ := h.request(t, "m-news"); !got.Released {
		t.Fatal("the newsletter was not released")
	}
	if diff := cmp.Diff([]string{"interactive 1", "interactive 1"}, grants(), compare.Options); diff != "" {
		t.Errorf("the leases a released body took (-want +got):\n%s", diff)
	}
}

// D3's part of VERIFICATIONS' row for a write-back that loses to a re-authorization, for connections
// that adopted the operator's credential while a body request ran. The request began its unit of work
// with the account's credential, which the provider rotated, the operator stored a new one, and a
// reload adopted it, all before the request's end wrote back. Its write-back names the adoption it
// began from, which the reload moved, so it is refused, and the operator's credential stays stored
// (ADR-0089, ADR-0090).
func TestAStaleWriteBackLeavesTheReauthorization(t *testing.T) {
	h := newHarness(t, options{interval: 100 * time.Millisecond})
	var once sync.Once
	h.mu.Lock()
	h.rotate[h.account+"-token"] = h.account + "-rotated"
	h.atEnd = func() {
		once.Do(func() {
			store(t, h.conn, h.keys, h.account, h.account+"-renewed")
			h.srv.reloaded(t)
		})
	}
	h.mu.Unlock()
	if got, out := h.request(t, "m-news"); !got.Released {
		t.Fatalf("the request answered %s", out)
	}
	if got := h.storedAfterRestart(t); got != h.account+"-renewed" {
		t.Errorf("after the request ended the row holds %q, want the operator's re-authorization", got)
	}
}

// D3's part of VERIFICATIONS' row for a provider that does not answer. A provider call that has not
// answered within the provider timeout fails the request as the provider's, on both roots, rather
// than holding it until the client gives up or reading as the API server's (ADR-0101).
func TestAProviderThatDoesNotAnswerIsTheProvidersFailure(t *testing.T) {
	h := newHarness(t)
	h.mu.Lock()
	h.hang = 2 * time.Second
	h.mu.Unlock()
	if origin := h.failed(t, h.account, "m-news"); origin != "provider" {
		t.Errorf("a provider that does not answer answered as the %q's", origin)
	}
}

// A body request whose lease is never granted fails once the request's deadline for its leases
// passes, 45 seconds from the request's arrival, as the API server's failure, and reaches no
// provider. The account's rate controller sits in a backoff lasting an hour, so every ask is left
// waiting. The policy load before the gate decides is held back by a lock on the policy's rules for
// part of the deadline, so a deadline counted from any later point, such as the unit of work's
// beginning, ends that much later (ADR-0130, ADR-0099, ADR-0025).
func TestARequestWaitsForItsLeasesNoLongerThanItsDeadline(t *testing.T) {
	h := newHarness(t)
	must(t, h.conn, `INSERT INTO rate_state (account_id, current_rate, target_rate, hard_cap, backoff_until)
		VALUES ($1, 50, 50, 80, now() + interval '1 hour')
		ON CONFLICT (account_id) DO UPDATE SET backoff_until = now() + interval '1 hour'`, h.account)
	const held = 3 * time.Second
	lock, err := h.conn.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lock.Exec(t.Context(), "LOCK TABLE policy_rules IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	done := make(chan string, 1)
	start := time.Now()
	go func() { done <- h.failed(t, h.account, "m-news") }()
	time.Sleep(held)
	if err := lock.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	select {
	case origin := <-done:
		switch elapsed := time.Since(start); {
		case elapsed < leaseWait:
			t.Errorf("the request gave up after %s, before its deadline for its leases", elapsed)
		case elapsed >= leaseWait+held:
			t.Errorf("the request gave up after %s, later than its deadline for its leases counted from its arrival", elapsed)
		}
		if origin != "api" {
			t.Errorf("a request whose lease never came answered as the %q's", origin)
		}
	case <-time.After(leaseWait + 20*time.Second):
		t.Fatal("the request was still waiting for its lease 20 seconds past its deadline")
	}
	if n := h.providerCalls(); n != 0 {
		t.Errorf("a request whose lease never came made %d provider calls", n)
	}
}
