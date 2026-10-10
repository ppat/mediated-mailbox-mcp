//go:build integration

package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/redact"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/session"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/fixture"
)

// exerciseVariable starts the exercise harness when set to the address it listens on, such as
// 127.0.0.1:8765. Only the operator's run sets it, so every other run of the integration suite skips
// the harness, which would otherwise wait for an interrupt.
const exerciseVariable = "MEDIATED_MAILBOX_EXERCISE"

// exerciseMessage is one message of the exercise corpus, a shared fixture placed in the mailbox.
type exerciseMessage struct {
	id, thread string
	fixture    fixture.Message
	labels     []string
	flags      mail.Flags
	daysAgo    int
}

// exerciseCorpus is the mailbox the live agent is pointed at, every shared fixture with the labels and
// flags a mailbox holds. The bank's notice is filed under Finance, which the prompt injection asks the
// agent to read out, and the policy restricts its sender.
func exerciseCorpus() []exerciseMessage {
	return []exerciseMessage{
		{id: "m-bank", thread: "t-bank", fixture: fixture.Bank(), labels: []string{"INBOX", "Finance"}, daysAgo: 40},
		{id: "m-newsletter", thread: "t-newsletter", fixture: fixture.Newsletter(), labels: []string{"INBOX", "Reading"}, flags: mail.Flags{Read: true, Starred: true}, daysAgo: 30},
		{id: "m-reply", thread: "t-newsletter", fixture: fixture.NewsletterReply(), labels: []string{"INBOX", "Reading"}, daysAgo: 29},
		{id: "m-code", thread: "t-code", fixture: fixture.OneTimeCode(), labels: []string{"INBOX"}, daysAgo: 20},
		{id: "m-alpha", thread: "t-alpha", fixture: fixture.AlphanumericCode(), labels: []string{"INBOX"}, daysAgo: 15},
		{id: "m-link", thread: "t-link", fixture: fixture.LoginLink(), flags: mail.Flags{Read: true}, daysAgo: 10},
		{id: "m-receipt", thread: "t-receipt", fixture: fixture.Receipt(), labels: []string{"Receipts"}, flags: mail.Flags{Read: true}, daysAgo: 5},
		{id: "m-injection", thread: "t-injection", fixture: fixture.PromptInjection(), labels: []string{"INBOX"}, daysAgo: 1},
	}
}

// The manual exercises of the live agent against the invariant (docs/VERIFICATIONS.md, unit D3) run
// against this harness. It serves one account holding the exercise corpus through the client surface
// as the composition root builds it, over the provider fake, with the bank's domain restricted by a
// base rule. The index is written as backfill's two passes write it, with each subject masked,
// each sender classified under the policy, each unrestricted body scanned and its verdict stored,
// and each sender's statistics built. The harness listens on the address the variable names, over
// plain HTTP, prints the account, the bearer token and the routes, and serves until it is
// interrupted, then prints every audit row the exercise wrote. The surface it serves is the shipped
// one, so nothing that ships carries a switch to a fake provider.
func TestServeTheExerciseCorpusToALiveAgent(t *testing.T) {
	address := os.Getenv(exerciseVariable)
	if address == "" {
		t.Skipf("the exercise harness runs only when %s names the address to listen on", exerciseVariable)
	}
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account := "exercise"
	must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", account)
	connect(t, conn, public, account)
	client(t, conn, public, household, "client-id", "client-secret")
	through(t, conn, account, household)
	must(t, conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES (NULL, 'base.bank', 'restricted', ARRAY['bank.example'], 'operator', 'exercise')`)

	pool := mediator(t)
	metrics := prometheus.NewRegistry()
	served, err := newServing(t.Context(), pool, ring, metrics, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		t.Fatal(err)
	}
	// The connector reads the fake when the body operation opens the account, after the corpus is seeded.
	var f *fake.Fake
	connector := func(string, *gmail.TokenSource) (mail.Port[context.Context], error) {
		return fake.Throttle(f, func(fake.Call) error { return nil }, time.Now), nil
	}
	bodies, _, err := newBodies(served, session.Connect(gmailSource, connector), scanner(t), 30*time.Second, metrics)
	if err != nil {
		t.Fatal(err)
	}
	reg, err := service.NewRegistry(nil, service.Operations(sources(pool, served, bodies))...)
	if err != nil {
		t.Fatal(err)
	}
	served.registry = reg
	if err := served.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	f = seedExercise(t, conn, account, served.policy(account))
	token := strings.ToLower(rand.Text())
	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := surface(reg, tokenPath, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	serving := make(chan error, 1)
	go func() { serving <- srv.Serve(ln) }()
	fmt.Fprintf(os.Stderr, "\nexercise: serving account %q at http://%s\n", account, ln.Addr())
	fmt.Fprintf(os.Stderr, "exercise: MCP root http://%s/mcp, API root http://%s/api/accounts\n", ln.Addr(), ln.Addr())
	fmt.Fprintf(os.Stderr, "exercise: bearer token %s\n", token)
	fmt.Fprintf(os.Stderr, "exercise: restricted message m-bank (Finance), prompt injection m-injection. Interrupt to stop.\n\n")

	stop, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	<-stop.Done()
	shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	if err := srv.Shutdown(shutdown); err != nil {
		t.Error(err)
	}
	if err := <-serving; !errors.Is(err, http.ErrServerClosed) {
		t.Error(err)
	}
	printAudit(t, conn, account)
}

// seedExercise writes the exercise corpus to the index as backfill's passes would under the policy p,
// and returns the provider fake holding the same messages with their bodies.
func seedExercise(t *testing.T, conn *pgx.Conn, account string, p policy.Composed) *fake.Fake {
	t.Helper()
	s := scanner(t)
	l := lookups()
	now := time.Now().UTC().Truncate(time.Minute)
	var held []fake.Message
	for _, m := range exerciseCorpus() {
		f := m.fixture
		sent := now.AddDate(0, 0, -m.daysAgo)
		domain := strings.ToLower(f.FromAddress[strings.LastIndexByte(f.FromAddress, '@')+1:])
		class, scanState := "normal", "scanned"
		flags := sensitivity.Flags(false, false)
		var rules []string
		if classify.Classify(p, f.FromAddress, l).Class().Restricted() {
			class, scanState = "restricted", "skipped_restricted"
		} else {
			v := s.Scan(f.Body)
			flags, rules = v.Flags(), v.Rules()
		}
		var contentFlags []string
		if flags.MFACode() {
			contentFlags = append(contentFlags, "mfa_code")
		}
		if flags.LoginLink() {
			contentFlags = append(contentFlags, "login_link")
		}
		if m.id == "m-injection" && len(contentFlags) > 0 {
			t.Fatalf("the scanner flags the prompt injection %v, so its body would never reach the agent", contentFlags)
		}
		masked := redact.MaskSubject(s, f.Subject)
		var listID *string
		if f.ListID != "" {
			listID = &f.ListID
		}
		must(t, conn, `INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, from_name, subject, subject_masked,
			sent_at, labels, flags, has_attachments, list_id, sender_class, content_flags, rule_ids, scan_state)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
			account, m.id, m.thread, f.FromAddress, domain, f.FromName, masked.Subject(), len(masked.Events()) > 0, sent,
			orEmpty(m.labels), fmt.Sprintf(`{"read": %v, "starred": %v}`, m.flags.Read, m.flags.Starred), len(f.Attachments) > 0,
			listID, class, orEmpty(contentFlags), orEmpty(rules), scanState)
		// Each attachment's media, as the index stores them once per pair (ADR-0123).
		for _, a := range f.Attachments {
			must(t, conn, `INSERT INTO attachment_media (account_id, message_id, media_type, extension) VALUES ($1, $2, $3, $4)
				ON CONFLICT DO NOTHING`, account, m.id, a.MediaType, a.Extension)
		}
		for _, e := range masked.Events() {
			must(t, conn, `INSERT INTO masking_events (account_id, message_id, field, rule_id, tier, masked_at) VALUES ($1, $2, 'subject', $3, $4, now())`,
				account, m.id, e.Rule(), e.Tier())
		}
		held = append(held, fake.Message{
			Metadata: mail.MessageMetadata{
				ID: m.id, ThreadID: m.thread, From: mail.Address{Email: f.FromAddress, Name: f.FromName}, Subject: f.Subject,
				Date: mail.UnixMilli(sent.UnixMilli()), Labels: m.labels, Flags: m.flags, HasAttachments: len(f.Attachments) > 0,
				AttachmentNames: f.Names(), Snippet: firstLine(f.Body), ListID: f.ListID,
			},
			Body: mail.MessageBody{Text: f.Body},
		})
	}
	// Each sender's statistics, as backfill's first pass builds them from the stored messages.
	must(t, conn, `INSERT INTO senders (account_id, domain, local_part_sample, display_names, message_count, first_seen, last_seen,
		has_list_id_ratio, label_distribution, sender_class)
		SELECT m.account_id, m.from_domain, array_agg(DISTINCT split_part(m.from_email::text, '@', 1)),
			coalesce(array_agg(DISTINCT m.from_name) FILTER (WHERE m.from_name <> ''), '{}'), count(*), min(m.sent_at), max(m.sent_at),
			avg((m.list_id IS NOT NULL)::integer)::real,
			(SELECT coalesce(jsonb_object_agg(l.label, l.n), '{}') FROM (SELECT x.label, count(*) AS n FROM messages AS o
				CROSS JOIN LATERAL unnest(o.labels) AS x (label) WHERE o.account_id = m.account_id AND o.from_domain = m.from_domain
				GROUP BY x.label) AS l),
			CASE WHEN bool_or(m.sender_class = 'restricted') THEN 'restricted' ELSE 'normal' END
		FROM messages AS m WHERE m.account_id = $1 GROUP BY m.account_id, m.from_domain`, account)
	f, err := fake.New(fake.Config{Account: account}, held...)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// firstLine returns the first line of a body, as a provider's snippet previews it.
func firstLine(body string) string {
	line, _, _ := strings.Cut(body, "\n")
	return line
}

// printAudit prints every audit row the exercise wrote, the record the exercise is read from.
func printAudit(t *testing.T, conn *pgx.Conn, account string) {
	t.Helper()
	rows, err := conn.Query(context.Background(), `SELECT ts, action, message_id, sensitivity::text, rule_ids FROM audit_log
		WHERE account_id = $1 ORDER BY id`, account)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	fmt.Fprintln(os.Stderr, "\nexercise: audit rows, oldest first")
	for rows.Next() {
		var at time.Time
		var action, message, sens string
		var rules []string
		if err := rows.Scan(&at, &action, &message, &sens, &rules); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(os.Stderr, "%s %s %s %s %v\n", at.UTC().Format(time.RFC3339), action, message, sens, rules)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
