//go:build integration

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/accountload"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/fixture"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
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

// providerTimeout is the provider timeout the harness's mediator runs with, short so a provider that
// does not answer fails the request quickly.
const providerTimeout = 300 * time.Millisecond

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
			text: bank.Body, snippet: marker.Body("banksnippet"), names: bank.Attachments,
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

// harness is one account served by the mediator as the composition root builds it, over the provider
// fake, with the bank's domain restricted by a base rule.
type harness struct {
	conn    *pgx.Conn
	pool    *pgxpool.Pool
	ring    *open.Keyring
	public  seal.PublicKey
	account string
	served  *serving
	reg     service.Registry
	metrics *prometheus.Registry
	log     *logBuffer
	fake    *fake.Fake
	// providers opens the body operation's provider sessions.
	providers *providers

	mu sync.Mutex
	// calls counts the provider calls each port the connector built received.
	calls int
	// tokens are the refresh tokens of the sources the connector built ports over, in order, and
	// sources the sources themselves.
	tokens  []string
	sources []*gmail.TokenSource
	// refuse is the refresh token whose calls the provider refuses, and outage fails every call.
	refuse string
	outage bool
	// hang holds each call this long before it reaches the fake, as a provider that does not answer.
	hang time.Duration
}

// newHarness serves one connected account holding corpus.
func newHarness(t *testing.T) *harness {
	t.Helper()
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	h := &harness{conn: conn, pool: mediator(t), ring: ring, public: public, account: newAccounts(t, conn)[0], metrics: prometheus.NewRegistry(), log: &logBuffer{}}
	connect(t, conn, public, h.account)
	client(t, conn, public, household, "client-id", "client-secret")
	through(t, conn, h.account, household)
	must(t, conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES (NULL, 'base.bank', 'restricted', ARRAY['bank.example'], 'operator', 'test')`)
	var held []fake.Message
	for i, m := range corpus() {
		domain := m.from[strings.IndexByte(m.from, '@')+1:]
		must(t, conn, `INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments,
			sender_class, scan_state, content_flags, rule_ids) VALUES ($1, $2, $2, $3, $4, now(), $5, 'normal', $6, $7, $8)`,
			h.account, m.id, m.from, domain, len(m.names) > 0, m.scan, orEmpty(m.flags), orEmpty(m.rules))
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
	f, err := fake.New(fake.Config{Account: h.account}, held...)
	if err != nil {
		t.Fatal(err)
	}
	h.fake = f
	h.served, err = newServing(t.Context(), h.pool, ring, h.metrics, debugLogger(h.log))
	if err != nil {
		t.Fatal(err)
	}
	bodies, opener, err := newBodies(h.served, h.connect, scanner(t), providerTimeout, h.metrics)
	if err != nil {
		t.Fatal(err)
	}
	h.providers = opener
	h.reg, err = service.NewRegistry(nil, service.Operations(sources(h.pool, h.served, bodies))...)
	if err != nil {
		t.Fatal(err)
	}
	h.served.registry = h.reg
	if err := h.served.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	return h
}

// orEmpty returns s, or an empty list for nil, so a text array column is written as '{}'.
func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// household is the name of the Gmail client the harness's account connects through. It is not the
// provider's name, so nothing finds it by the provider.
const household = "household"

// client writes a Gmail client under its name, its secret sealed to public.
func client(t *testing.T, conn *pgx.Conn, public seal.PublicKey, name, id, secret string) {
	t.Helper()
	sealed, err := public.Seal([]byte(secret), seal.ClientSecret(name))
	if err != nil {
		t.Fatal(err)
	}
	must(t, conn, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ($1, $2, $3, $4)", name, gmailProvider, id, sealed)
}

// through makes an account connect through the named client, as account setup writes it.
func through(t *testing.T, conn *pgx.Conn, account, client string) {
	t.Helper()
	must(t, conn, "UPDATE accounts SET oauth_client = $2 WHERE account_id = $1", account, client)
}

// connect is the harness's connector. It builds a port over the provider fake whose every call is
// counted, refused as a credential refusal while the source holds the refused token, and failed on
// the provider's side during an outage.
func (h *harness) connect(_ string, source *gmail.TokenSource) (mail.Port[context.Context], error) {
	token := source.RefreshToken()
	h.mu.Lock()
	h.tokens = append(h.tokens, token)
	h.sources = append(h.sources, source)
	h.mu.Unlock()
	return fake.Throttle(h.fake, func(fake.Call) error {
		h.mu.Lock()
		h.calls++
		hang := h.hang
		h.mu.Unlock()
		time.Sleep(hang)
		h.mu.Lock()
		defer h.mu.Unlock()
		switch {
		case h.outage:
			return fmt.Errorf("the fake's outage: %w", mail.ErrProvider)
		case h.refuse != "" && token == h.refuse:
			return fmt.Errorf("the fake refuses %s: %w", token, mail.ErrAuthentication)
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

// request asks for a message's body through the registry and returns the result's text with the
// result.
func (h *harness) request(t *testing.T, id string) (body, string) {
	t.Helper()
	out, err := h.reg.Call(t.Context(), "get_message_body", json.RawMessage(`{"account_id":"`+h.account+`","message_id":"`+id+`"}`))
	if err != nil {
		t.Fatalf("get_message_body %s: %v", id, err)
	}
	var b body
	if err := json.Unmarshal(out, &b); err != nil {
		t.Fatal(err)
	}
	return b, string(out)
}

// auditRow is one audit row as the test reads it back.
type auditRow struct {
	Actor, Action, Message, Sensitivity string
	Rules                               []string
}

// audit returns the account's audit rows, oldest first.
func (h *harness) audit(t *testing.T) []auditRow {
	t.Helper()
	rows, err := h.conn.Query(t.Context(), `SELECT actor, action, message_id, sensitivity::text, rule_ids FROM audit_log
		WHERE account_id = $1 ORDER BY id`, h.account)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (auditRow, error) {
		var a auditRow
		err := r.Scan(&a.Actor, &a.Action, &a.Message, &a.Sensitivity, &a.Rules)
		return a, err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// counter returns the value of a counter series with the given labels, or 0 when it has none.
func counter(t *testing.T, metrics prometheus.Gatherer, name string, labels map[string]string) float64 {
	t.Helper()
	families, err := metrics.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			got := map[string]string{}
			for _, l := range m.GetLabel() {
				got[l.GetName()] = l.GetValue()
			}
			if cmp.Equal(labels, got) {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

// The gate decides every request from the index under the policy in force, and a denial never reaches
// the provider (ADR-0002). D3's parts of VERIFICATIONS' rows for requesting a restricted body with the
// provider never contacted, and for an ex-restricted, still-unscanned message whose body would pass
// the serve-time pattern check. Each denial carries the gate's reason, the pending one told apart from
// a restricted sender, and no snippet, filename or body text of the message reaches the response or
// the audit row (ADR-0001). Every denial writes one DENY_BODY row naming the rules behind it, and is
// counted as a gate denial (ADR-0036).
func TestTheGateDeniesFromTheIndexWithoutReachingTheProvider(t *testing.T) {
	h := newHarness(t)
	cases := []struct {
		id, reason string
		rules      []string
	}{
		{"m-bank", "restricted sender", []string{"base.bank"}},
		{"m-pending", "pending content scan", []string{}},
		{"m-flagged", "content flagged", []string{"link.query_token"}},
		{"m-oldrestricted", "skipped as restricted", []string{}},
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
			if c.reason == "pending content scan" && !strings.Contains(*got.Note, "backlog") {
				t.Errorf("the pending denial's note does not read as backlog: %s", *got.Note)
			}
			// A sender left unscanned as restricted may have been delisted since, so the note must not
			// say the policy lists it (ADR-0037).
			if c.reason == "skipped as restricted" && strings.Contains(*got.Note, "is on the sensitive-sender policy") {
				t.Errorf("the note for a message skipped as restricted says the policy lists its sender: %s", *got.Note)
			}
		})
	}
	if n := h.providerCalls(); n != 0 {
		t.Errorf("the provider received %d calls for bodies the gate denied", n)
	}
	rows := h.audit(t)
	if len(rows) != len(cases) {
		t.Fatalf("%d audit rows for %d denials: %+v", len(rows), len(cases), rows)
	}
	for i, c := range cases {
		r := rows[i]
		if r.Action != "DENY_BODY" || r.Message != c.id || r.Actor != "client" || !slices.Equal(r.Rules, c.rules) {
			t.Errorf("the audit row of %s is %+v", c.id, r)
		}
		if strings.Contains(r.Sensitivity, marker.BodyPrefix) {
			t.Errorf("the audit row of %s carries message text: %s", c.id, r.Sensitivity)
		}
	}
	if want := `{"scan_state": "scanned", "sender_class": "restricted", "class_rule_id": "base.bank", "content_flags": []}`; rows[0].Sensitivity != want {
		t.Errorf("the restricted sender's audit row records %s, want %s", rows[0].Sensitivity, want)
	}
	for _, c := range cases {
		if got := counter(t, h.metrics, bodiesDeniedName, map[string]string{"account": h.account, "stage": "gate", "reason": c.reason}); got != 1 {
			t.Errorf("the gate denials for %q count %v, want 1", c.reason, got)
		}
	}
	if got := counter(t, h.metrics, bodiesServedName, map[string]string{"account": h.account}); got != 0 {
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
	rows := h.audit(t)
	if len(rows) != 2 || rows[0].Action != "READ_BODY" || rows[1].Action != "READ_BODY" || rows[0].Message != "m-news" {
		t.Fatalf("the audit rows are %+v", rows)
	}
	if got := counter(t, h.metrics, bodiesServedName, map[string]string{"account": h.account}); got != 2 {
		t.Errorf("two serves counted %v", got)
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
		if got.Released || got.Reason != "serve-time pattern check matched" || got.Body != nil || got.Snippet != nil || got.AttachmentNames != nil {
			t.Errorf("%s answered %s", id, out)
		}
		if strings.Contains(out, marker.BodyPrefix) || strings.Contains(out, "482913") || strings.Contains(out, "Zq8XvB3n") {
			t.Errorf("%s's denial carries its text: %s", id, out)
		}
	}
	if got, out := h.request(t, "m-skipped"); !got.Released || got.Snippet == nil {
		t.Errorf("a clean gate-skipped message answered %s", out)
	}
	rows := h.audit(t)
	if len(rows) != 5 {
		t.Fatalf("the audit rows are %+v", rows)
	}
	for _, r := range rows[:4] {
		if r.Action != "DENY_BODY" || len(r.Rules) == 0 {
			t.Errorf("a serve-time denial's audit row is %+v", r)
		}
	}
	if rows[4].Action != "READ_BODY" {
		t.Errorf("the clean message's audit row is %+v", rows[4])
	}
	labels := map[string]string{"account": h.account, "stage": "serve", "reason": "serve-time pattern check matched"}
	if got := counter(t, h.metrics, bodiesDeniedName, labels); got != 4 {
		t.Errorf("the serve-time denials count %v, want 4", got)
	}
	labels["stage"] = "gate"
	if got := counter(t, h.metrics, bodiesDeniedName, labels); got != 0 {
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
		sender_class, scan_state) VALUES ($1, 'm-pixel', 'm-pixel', 'a@newsletter.example', 'newsletter.example', now(), false, 'normal', 'scanned')`, h.account)
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

// A body the conversion refuses is denied at serve, audited and counted (ADR-0036).
func TestABodyTheConversionRefusesIsDenied(t *testing.T) {
	h := newHarness(t)
	huge := "<p>" + strings.Repeat("a", 600<<10) + "</p>"
	if err := h.fake.Deliver(fake.Message{
		Metadata: mail.MessageMetadata{ID: "m-huge", ThreadID: "m-huge", From: mail.Address{Email: "a@newsletter.example"}},
		Body:     mail.MessageBody{HTML: huge, Text: "x"},
	}); err != nil {
		t.Fatal(err)
	}
	must(t, h.conn, `INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments,
		sender_class, scan_state) VALUES ($1, 'm-huge', 'm-huge', 'a@newsletter.example', 'newsletter.example', now(), false, 'normal', 'scanned')`, h.account)
	got, _ := h.request(t, "m-huge")
	if got.Released || got.Reason != "the conversion refused the body" || got.Body != nil {
		t.Errorf("a body the conversion refuses answered %+v", got)
	}
	if rows := h.audit(t); len(rows) != 1 || rows[0].Action != "DENY_BODY" {
		t.Errorf("the audit rows are %+v", rows)
	}
	if got := counter(t, h.metrics, bodiesDeniedName, map[string]string{"account": h.account, "stage": "serve", "reason": "the conversion refused the body"}); got != 1 {
		t.Errorf("the refusal was counted %v times", got)
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
	if got, _ := h.request(t, "m-news"); got.Released || got.Reason != "restricted sender" {
		t.Errorf("the next call after the domain was listed answered %+v", got)
	}
	if h.providerCalls() != calls {
		t.Error("the denial reached the provider")
	}

	const policyRead = "SELECT (account_id, rule_id, class, domain_suffix) ON policy_rules"
	revoke(t, h.conn, policyRead)
	must(t, h.conn, "DELETE FROM policy_rules WHERE rule_id = 'base.news'")
	if got, _ := h.request(t, "m-news"); got.Released || got.Reason != "restricted sender" {
		t.Errorf("with the load failing, the request answered %+v, want the active policy's denial", got)
	}
	// A sender the active policy does not list is still released, so a failed load decides under
	// the active policy rather than denying every sender.
	if got, _ := h.request(t, "m-other"); !got.Released {
		t.Errorf("with the load failing, a sender the active policy does not list answered %+v, want it released", got)
	}
	if got := reloadFailed(t, h.metrics); !slices.Equal(got, []float64{1}) {
		t.Errorf("after a failed load the reload-failure series reads %v", got)
	}
	grantBack(t, h.conn, policyRead)
	if got, _ := h.request(t, "m-news"); !got.Released {
		t.Errorf("once the load succeeds again and the rule is gone, the request answered %+v", got)
	}
	if got := reloadFailed(t, h.metrics); !slices.Equal(got, []float64{0}) {
		t.Errorf("after a load that succeeded the reload-failure series reads %v", got)
	}
}

// D3's part of VERIFICATIONS' row for editing the policy while the accounts stay the same. A
// scheduled reload loads the policy every time, so a metadata read shows an edit without a body
// request (ADR-0099).
func TestEveryReloadLoadsThePolicy(t *testing.T) {
	h := newHarness(t)
	read := func() string {
		out, err := h.reg.Call(t.Context(), "get_message", json.RawMessage(`{"account_id":"`+h.account+`","message_id":"m-news"}`))
		if err != nil {
			t.Fatal(err)
		}
		var m struct {
			Sensitivity struct {
				SenderClass string `json:"sender_class"`
			} `json:"sensitivity"`
		}
		if err := json.Unmarshal(out, &m); err != nil {
			t.Fatal(err)
		}
		return m.Sensitivity.SenderClass
	}
	must(t, h.conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES (NULL, 'base.news', 'restricted', ARRAY['newsletter.example'], 'operator', 'test')`)
	if got := read(); got != "normal" {
		t.Fatalf("before a reload the metadata reads %s", got)
	}
	if err := h.served.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := read(); got != "restricted" {
		t.Errorf("after a reload with the same accounts the metadata reads %s, want restricted", got)
	}
}

// An audit write that fails releases nothing, and the request fails as the mediator's (ADR-0002,
// ADR-0101). D3's part of VERIFICATIONS' row for the audit of every serve and denial.
func TestNoBodyIsReleasedWithoutItsAuditRow(t *testing.T) {
	h := newHarness(t)
	revoke(t, h.conn, "INSERT ON audit_log")
	out, err := h.reg.Call(t.Context(), "get_message_body", json.RawMessage(`{"account_id":"`+h.account+`","message_id":"m-news"}`))
	if err == nil {
		t.Fatalf("a release whose audit write failed answered %s", out)
	}
	if origin, _ := service.Classify(err); origin != service.OriginMediator {
		t.Errorf("the failed audit write is the %s's failure", origin)
	}
	if got := counter(t, h.metrics, bodiesServedName, map[string]string{"account": h.account}); got != 0 {
		t.Errorf("a body whose audit write failed was counted as served %v times", got)
	}
}

// surfaceAnswer is what one root answered, its status and its content.
type surfaceAnswer struct {
	Status  int
	Content string
}

// both asks the API root and the MCP root for the same message's body, behind the bearer check as the
// composition root mounts them, and returns what each answered.
func (h *harness) both(t *testing.T, id string) (apiAnswer, mcpAnswer surfaceAnswer) {
	t.Helper()
	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte("s3cret"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := surface(h.reg, tokenFile, debugLogger(h.log))
	if err != nil {
		t.Fatal(err)
	}
	send := func(method, target, body string) (int, []byte) {
		req := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer s3cret")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		out, err := io.ReadAll(rec.Result().Body)
		if err != nil {
			t.Fatal(err)
		}
		return rec.Code, out
	}
	status, out := send(http.MethodGet, "/api/accounts/"+h.account+"/messages/"+id+"/body", "")
	apiAnswer = surfaceAnswer{status, string(out)}
	call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_message_body","arguments":{"account_id":"` + h.account + `","message_id":"` + id + `"}}}`
	_, out = send(http.MethodPost, "/mcp", call)
	var r struct {
		Result struct {
			IsError           bool            `json:"isError"`
			StructuredContent json.RawMessage `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("the MCP root answered %s: %v", out, err)
	}
	status = http.StatusOK
	if r.Result.IsError {
		status = 0
	}
	return apiAnswer, surfaceAnswer{status, string(r.Result.StructuredContent)}
}

// D3's part of VERIFICATIONS' rows for inducing one failure of each origin and for the two roots'
// shapes. A message the account does not hold is the client's failure, an audit write that fails the
// mediator's, and a provider outage and a message the provider no longer holds the provider's. Each
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
		if mcp.Status != 0 || mcp.Content != api.Content {
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
	check("an audit write that fails", "m-news", want{http.StatusInternalServerError, "mediator"})
	grantBack(t, h.conn, "INSERT ON audit_log")

	api, mcp := h.both(t, "m-bank")
	if api.Status != http.StatusOK || mcp.Status != http.StatusOK || api.Content != mcp.Content || !strings.Contains(api.Content, `"released":false`) {
		t.Errorf("a denial answered %+v on the API root and %+v on the MCP root", api, mcp)
	}
}

// D3's part of VERIFICATIONS' row for supplying a credential in the environment or a mounted file with
// the account's state row holding none. An account whose state row holds no sealed credential is not
// connected, whatever the environment holds, so its released message fails as the mediator's and
// never reaches a provider (ADR-0080).
func TestAnAccountWithoutAStoredCredentialIsNotConnected(t *testing.T) {
	h := newHarness(t)
	t.Setenv("MEDIATED_MAILBOX_REFRESH_TOKEN", "from-the-environment")
	t.Setenv("GMAIL_REFRESH_TOKEN", "from-the-environment")
	must(t, h.conn, "UPDATE account_state SET credential = NULL WHERE account_id = $1", h.account)
	if err := h.served.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	_, err := h.reg.Call(t.Context(), "get_message_body", json.RawMessage(`{"account_id":"`+h.account+`","message_id":"m-news"}`))
	if origin, _ := service.Classify(err); err == nil || origin != service.OriginMediator {
		t.Errorf("a body of an account with no stored credential answered %v", err)
	}
	if h.providerCalls() != 0 || len(h.tokens) != 0 {
		t.Errorf("the provider was reached with the tokens %v", h.tokens)
	}
}

// D3's part of VERIFICATIONS' rows for building a token source with the client's identifier and
// secret swapped and for an account's own client. The credentials a body request's source is built
// from are the Gmail client the account connects through, its identifier and its secret each in its
// own place, and the account's own stored refresh token. A second account connects through a second
// client of the one provider, so each account's credentials carry its own client and never the
// other's (ADR-0085, ADR-0106).
func TestTheCredentialsAreTheClientAndTheAccountsToken(t *testing.T) {
	h := newHarness(t)
	other := strings.TrimSuffix(h.account, "-a") + "-b"
	connect(t, h.conn, h.public, other)
	client(t, h.conn, h.public, "employer", "employer-id", "employer-secret")
	through(t, h.conn, other, "employer")
	if err := h.served.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	for account, want := range map[string]gmail.Credentials{
		h.account: {ClientID: "client-id", ClientSecret: "client-secret", RefreshToken: h.account + "-token"},
		other:     {ClientID: "employer-id", ClientSecret: "employer-secret", RefreshToken: other + "-token"},
	} {
		got, err := credentials(h.served.loader.Snapshot(), account)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, got, compare.Options); diff != "" {
			t.Errorf("credentials of %s (-want +got):\n%s", account, diff)
		}
	}
	if _, err := credentials(h.served.loader.Snapshot(), "acct-unlisted"); !errors.Is(err, errNotConnected) {
		t.Errorf("an account the snapshot does not list has credentials, %v", err)
	}
}

// F6's part of VERIFICATIONS' row for an account of a provider that authenticates through an OAuth
// client served without its client, for the mediator. The mediator tells the loader Gmail
// authenticates through a client, so a connected Gmail account that names no client, though its
// provider has one, is not connected and its body request never reaches a provider (ADR-0106,
// ADR-0090).
func TestAnAccountNamingNoClientIsNotConnected(t *testing.T) {
	h := newHarness(t)
	must(t, h.conn, "UPDATE accounts SET oauth_client = NULL WHERE account_id = $1", h.account)
	if err := h.served.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := credentials(h.served.loader.Snapshot(), h.account); !errors.Is(err, errNotConnected) {
		t.Errorf("an account naming no client has credentials, %v", err)
	}
	_, err := h.reg.Call(t.Context(), "get_message_body", json.RawMessage(`{"account_id":"`+h.account+`","message_id":"m-news"}`))
	if origin, _ := service.Classify(err); err == nil || origin != service.OriginMediator {
		t.Errorf("a body of an account naming no client answered %v", err)
	}
	if h.providerCalls() != 0 || len(h.tokens) != 0 {
		t.Errorf("the provider was reached with the tokens %v", h.tokens)
	}
}

// D3's part of VERIFICATIONS' row for reloading the account snapshot, for the mediator's body fetch.
// A credential the provider refuses is read again from its row before the refusal is reported, so a
// re-authorization reaches the next refused call. When the row holds another credential, the call is
// made again over a source holding it, which later requests keep, so the access token it is issued
// serves them rather than each building a source of its own. When the row holds the refused one,
// the refusal is reported as the provider's (ADR-0090, ADR-0101).
func TestARefusedCredentialIsReadAgainBeforeTheRefusalIsReported(t *testing.T) {
	h := newHarness(t)
	h.mu.Lock()
	h.refuse = h.account + "-token"
	h.mu.Unlock()
	_, err := h.reg.Call(t.Context(), "get_message_body", json.RawMessage(`{"account_id":"`+h.account+`","message_id":"m-news"}`))
	if origin, _ := service.Classify(err); origin != service.OriginProvider {
		t.Fatalf("a refusal with the stored credential unchanged answered %v", err)
	}

	sealed, err := h.public.Seal([]byte(h.account+"-renewed"), seal.AccountCredential(h.account))
	if err != nil {
		t.Fatal(err)
	}
	must(t, h.conn, "UPDATE account_state SET credential = $2 WHERE account_id = $1", h.account, sealed)
	if got, out := h.request(t, "m-news"); !got.Released {
		t.Fatalf("after the operator replaced the credential, the request answered %s", out)
	}
	if got, out := h.request(t, "m-plain"); !got.Released {
		t.Fatalf("the request after the re-authorization answered %s", out)
	}
	want := []string{h.account + "-token", h.account + "-token", h.account + "-renewed", h.account + "-renewed"}
	if diff := cmp.Diff(want, h.tokens, compare.Options); diff != "" {
		t.Errorf("the tokens the ports were built over (-want +got):\n%s", diff)
	}
	if len(h.sources) == 4 && h.sources[3] != h.sources[2] {
		t.Error("the request after the re-authorization built a source of its own rather than keeping the one the retry built")
	}
}

// F6's part of VERIFICATIONS' row for an account's own client, for the mediator's re-read. The account
// moves to another client while the mediator serves it, its new client and new credential written in
// one transaction (ADR-0106). The provider refuses the old credential, and the call made again is
// built from the new client and the new credential, which the account's later requests keep
// (ADR-0090).
func TestARereadAfterAMoveBuildsTheSourceFromTheNewClient(t *testing.T) {
	h := newHarness(t)
	if got, out := h.request(t, "m-plain"); !got.Released {
		t.Fatalf("the first request answered %s", out)
	}
	h.mu.Lock()
	h.refuse = h.account + "-token"
	h.mu.Unlock()
	client(t, h.conn, h.public, "employer", "employer-id", "employer-secret")
	sealed, err := h.public.Seal([]byte(h.account+"-moved"), seal.AccountCredential(h.account))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := h.conn.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{"UPDATE accounts SET oauth_client = 'employer' WHERE account_id = $1", "UPDATE account_state SET credential = $2 WHERE account_id = $1"} {
		args := []any{h.account}
		if strings.Contains(sql, "$2") {
			args = append(args, sealed)
		}
		if _, err := tx.Exec(t.Context(), sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}

	if got, out := h.request(t, "m-news"); !got.Released {
		t.Fatalf("after the account moved, the request answered %s", out)
	}

	h.providers.mu.Lock()
	built := h.providers.sources[h.account].built
	h.providers.mu.Unlock()
	want := gmail.Credentials{ClientID: "employer-id", ClientSecret: "employer-secret", RefreshToken: h.account + "-moved"}
	if diff := cmp.Diff(want, built, compare.Options); diff != "" {
		t.Errorf("the credentials the retried source was built from (-want +got):\n%s", diff)
	}
}

// F6's part of VERIFICATIONS' row for an account served without its client, for the mediator's
// re-read. The provider refuses the credential, and the account is then moved to a client whose
// secret does not open, or loses its credential. The re-read finds it not connected, so the refusal
// is reported after the one refused call and no source is built for a second (ADR-0106, ADR-0090).
func TestARereadThatFindsTheAccountNotConnectedReportsTheRefusal(t *testing.T) {
	cases := map[string]func(t *testing.T, h *harness){
		"moved to a client that does not open": func(t *testing.T, h *harness) {
			sealed, err := h.public.Seal([]byte("broken-secret"), seal.ClientSecret("another-row"))
			if err != nil {
				t.Fatal(err)
			}
			must(t, h.conn, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('broken', $1, 'broken-id', $2)", gmailProvider, sealed)
			credential, err := h.public.Seal([]byte(h.account+"-moved"), seal.AccountCredential(h.account))
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

			_, err := h.reg.Call(t.Context(), "get_message_body", json.RawMessage(`{"account_id":"`+h.account+`","message_id":"m-news"}`))

			if origin, _ := service.Classify(err); origin != service.OriginProvider {
				t.Errorf("the request answered %v, want the provider's refusal", err)
			}
			if calls := h.providerCalls(); calls != 1 {
				t.Errorf("the provider received %d calls, want the one refused", calls)
			}
		})
	}
}

// D3's parts of VERIFICATIONS' rows for failing a rotated credential's write-back and for reloading
// the account snapshot. A body request whose hand-over fails still answers, the failure is logged,
// and a reload keeps the rotated credential while the stored bytes are unchanged, so the next request
// uses it and its hand-over writes it once the write can land (ADR-0082, ADR-0090).
func TestARotationWhoseWriteBackFailedIsKept(t *testing.T) {
	h := newHarness(t)
	creds, err := credentials(h.served.loader.Snapshot(), h.account)
	if err != nil {
		t.Fatal(err)
	}
	rotated := creds
	rotated.RefreshToken = h.account + "-rotated"
	h.providers.mu.Lock()
	h.providers.sources[h.account] = heldSource{built: creds, source: gmail.NewTokenSource(unreachable(), rotated)}
	h.providers.mu.Unlock()
	const stateWrite = "UPDATE (credential) ON account_state"
	revoke(t, h.conn, stateWrite)
	if got, out := h.request(t, "m-news"); !got.Released {
		t.Fatalf("a request whose hand-over failed answered %s", out)
	}
	if !strings.Contains(h.log.String(), "was not written back") {
		t.Errorf("the failed write-back was not logged:\n%s", h.log.String())
	}
	if err := h.served.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got, _ := h.request(t, "m-plain"); !got.Released {
		t.Fatal("the request after the reload was not released")
	}
	if last := h.tokens[len(h.tokens)-1]; last != rotated.RefreshToken {
		t.Errorf("after the reload the request used %q, want the rotated credential", last)
	}
	grantBack(t, h.conn, stateWrite)
	if got, _ := h.request(t, "m-news"); !got.Released {
		t.Fatal("the request once the write can land was not released")
	}
	a, _ := loadedSnapshot(t, h).Account(h.account)
	if got := string(a.Credential()); got != rotated.RefreshToken {
		t.Errorf("after a restart the account holds %q, want the rotated token", got)
	}
}

// D3's parts of VERIFICATIONS' rows for forcing a rotation, for failing its write-back and for failing
// a unit of work after an authentication attempt. Each body request is the mediator's unit of work,
// and when it ends the source's current refresh token is handed over, so a process started afterwards
// reads a rotated one, and the source's latest authentication attempt is recorded, a request that
// failed included (ADR-0082, ADR-0097). Google's rotation and a refresh are out of reach without a
// stand-in for its endpoint (ADR-0043), so the account's held source is one holding the token a
// rotation would leave, with a failed attempt made through a proxy nothing listens on.
func TestEachBodyRequestHandsOverAndRecordsTheAttempt(t *testing.T) {
	h := newHarness(t)
	creds, err := credentials(h.served.loader.Snapshot(), h.account)
	if err != nil {
		t.Fatal(err)
	}
	rotated := creds
	rotated.RefreshToken = h.account + "-rotated"
	source := gmail.NewTokenSource(unreachable(), rotated)
	if _, err := source.AccessToken(t.Context()); err == nil {
		t.Fatal("a refresh through an unreachable proxy returned an access token")
	}
	h.providers.mu.Lock()
	h.providers.sources[h.account] = heldSource{built: creds, source: source}
	h.providers.mu.Unlock()
	h.mu.Lock()
	h.outage = true
	h.mu.Unlock()
	if _, err := h.reg.Call(t.Context(), "get_message_body", json.RawMessage(`{"account_id":"`+h.account+`","message_id":"m-news"}`)); err == nil {
		t.Fatal("the request during the outage succeeded")
	}
	restarted := loadedSnapshot(t, h)
	a, _ := restarted.Account(h.account)
	if got := string(a.Credential()); got != h.account+"-rotated" {
		t.Errorf("after a restart the account holds %q, want the rotated token", got)
	}
	var outcome *string
	if err := h.conn.QueryRow(t.Context(), "SELECT last_auth_outcome FROM account_state WHERE account_id = $1", h.account).Scan(&outcome); err != nil {
		t.Fatal(err)
	}
	if outcome == nil || *outcome != string(mail.AuthFailed) {
		t.Errorf("the recorded outcome is %v, want failed", outcome)
	}
}

// loadedSnapshot returns the account snapshot a process started now would load.
func loadedSnapshot(t *testing.T, h *harness) *accountload.Snapshot {
	t.Helper()
	l := accountload.New(h.pool, h.ring, slog.New(slog.DiscardHandler), []string{gmailProvider})
	if err := l.Load(t.Context()); err != nil {
		t.Fatal(err)
	}
	return l.Snapshot()
}

// unreachable returns a client whose requests fail before they leave the machine, sent through a
// proxy on a port nothing listens on, so a token source refreshing through it makes a real attempt
// that gets no answer, with no stand-in for Google's endpoint (ADR-0043).
func unreachable() *http.Client {
	return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: "127.0.0.1:9"})}}
}

// D3's part of VERIFICATIONS' row for leasing body fetches in the interactive class. A released
// message's two provider calls, its body and its metadata, each take a lease in the interactive class,
// sized by the call's cost, and a body the gate denies takes none (ADR-0025, ADR-0002).
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

// D3's part of VERIFICATIONS' row for a write-back that loses to a re-authorization, for a loader that
// adopted the operator's credential while a body request ran. The request opened its session with the
// account's old credential, the operator stored a new one, a reload adopted it, and the request then
// ended. Its hand-over names the adoption it started from, which the reload moved, so the loader
// discards it, and the operator's credential stays stored (ADR-0089, ADR-0090).
func TestAStaleHandOverLeavesTheReauthorization(t *testing.T) {
	h := newHarness(t)
	session, err := h.providers.open(t.Context(), h.account)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Body(t.Context(), "m-news"); err != nil {
		t.Fatal(err)
	}
	renewed, err := h.public.Seal([]byte(h.account+"-renewed"), seal.AccountCredential(h.account))
	if err != nil {
		t.Fatal(err)
	}
	must(t, h.conn, "UPDATE account_state SET credential = $2 WHERE account_id = $1", h.account, renewed)
	if err := h.served.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	session.Done(t.Context())
	a, _ := loadedSnapshot(t, h).Account(h.account)
	if got := string(a.Credential()); got != h.account+"-renewed" {
		t.Errorf("after the request ended the row holds %q, want the operator's re-authorization", got)
	}
}

// D3's part of VERIFICATIONS' row for a provider that does not answer. A provider call that has not
// answered within the provider timeout fails the request as the provider's, on both roots, rather
// than holding it until the client gives up or reading as the mediator's (ADR-0101).
func TestAProviderThatDoesNotAnswerIsTheProvidersFailure(t *testing.T) {
	h := newHarness(t)
	h.mu.Lock()
	h.hang = 2 * time.Second
	h.mu.Unlock()
	_, err := h.reg.Call(t.Context(), "get_message_body", json.RawMessage(`{"account_id":"`+h.account+`","message_id":"m-news"}`))
	if origin, message := service.Classify(err); err == nil || origin != service.OriginProvider {
		t.Errorf("a provider that does not answer answered %v, read as %s: %s", err, origin, message)
	}
}
