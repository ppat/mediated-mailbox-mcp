//go:build gmail_live

package gmail_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	netmail "net/mail"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/contract"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/livecontract"
)

// The contract suite's run against real Gmail, on an account set aside for testing whose other
// contents are unknown (ADR-0043). It runs only through go tool livecontract gmail, which the
// gmail-contract workflow runs with the test account's credentials in the environment. Run any
// other way it skips before reading a credential, and no test run without its tag compiles it. The
// adapter holds the modify grant alone (ADR-0011). Each case inserts its own marked messages, and
// the suite checks and reports only those. Nothing here deletes mail, and the cases that need a
// message removed run against the fake alone. When a case ends, failed or not, the harness takes the labels it inserted
// each of the case's messages with off that message and moves it to the trash, where Gmail purges it
// (ADR-0043). So the account holds about a month of runs, and a later run's scoped listing, which
// reads the trash too, passes over them. The suite stops starting cases ten minutes before the
// test's deadline, so the cases already run clean up. Only a run killed outright leaves messages
// behind with their labels.
//
// Before it adds anything, the run confirms the credential belongs to the account
// GMAIL_TEST_ACCOUNT names, and stops if it does not, so a grant made in a browser signed into
// another account never reaches that account's mail (ADR-0043).
func TestTheAdapterPassesTheContractAgainstGmail(t *testing.T) {
	livecontract.Require(t, "gmail")
	account := environment(t, "GMAIL_TEST_ACCOUNT")
	creds := gmail.Credentials{
		ClientID:     environment(t, "GMAIL_TEST_CLIENT_ID"),
		ClientSecret: environment(t, "GMAIL_TEST_CLIENT_SECRET"),
		RefreshToken: environment(t, "GMAIL_TEST_REFRESH_TOKEN"),
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	tokens := gmail.NewTokenSource(client, creds, filepath.Join(t.TempDir(), "refresh-token"),
		slog.New(slog.NewTextHandler(os.Stderr, nil)))
	metrics, err := gmail.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatalf("NewMetrics: %v", err)
	}
	pace := &pacer{}
	pace.wait(t.Context(), unitsGetProfile)
	access, err := tokens.AccessToken(t.Context())
	if err != nil {
		t.Fatalf("obtaining an access token: %v", err)
	}
	granted, err := gmail.Address(t.Context(), client, access)
	if err != nil {
		t.Fatalf("reading the account the credential belongs to: %v", err)
	}
	if err := gmail.RequireAccount(granted, account); err != nil {
		t.Fatalf("%v, the one GMAIL_TEST_ACCOUNT names, so the run adds nothing", err)
	}
	g := &provider{client: client, tokens: tokens, pace: pace, labels: map[string]string{}}
	run := runText(t)
	t.Logf("the messages this run adds carry marks beginning mmfieldmarker-contract%s", run)
	contract.Run(t, contract.Config{
		Harness: func(t *testing.T, account string) contract.Implementation {
			adapter, err := gmail.New(gmail.Config{Account: account, Client: client, Tokens: tokens, Metrics: metrics})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			threads := map[string]thread{}
			messageIDs := map[string]string{}
			return contract.Implementation{
				Port: paced{port: adapter, pace: pace},
				Deliver: func(m contract.Message) (string, error) {
					id, messageID, err := g.insert(t.Context(), m, threads)
					if id != "" {
						messageIDs[id] = messageID
						t.Cleanup(func() { g.discard(t, id, m.Metadata.Labels) })
					}
					return id, err
				},
				Searchable: func(ids []string) error {
					for _, id := range ids {
						if err := g.searchable(t, messageIDs[id], id); err != nil {
							return err
						}
					}
					return nil
				},
			}
		},
		Run:    run,
		Base:   mail.UnixMilli(time.Now().Add(-10 * time.Minute).Truncate(time.Minute).UnixMilli()),
		Scoped: true,
		// Three hundred pages are 900 messages, far more than the run's own and the few that can
		// arrive while it runs, when a full enumeration lists the newest messages first. An order
		// that does not fails the enumeration cases there rather than at the timeout.
		EnumerationPages: 300,
		Reserve:          reserve,
	})
}

// environment returns the value of a variable the run needs.
func environment(t *testing.T, name string) string {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		t.Fatalf("%s is not set, and the run against Gmail needs the test account's credentials", name)
	}
	return v
}

// runText returns ten random lowercase letters, the run's own text, which no earlier run shares.
func runText(t *testing.T) string {
	t.Helper()
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("reading random bytes: %v", err)
	}
	for i := range b {
		b[i] = 'a' + b[i]%26
	}
	return string(b)
}

// pacer spaces the run's requests at the target rate, half the budget Gmail's profile declares
// (ADR-0024), so the run is not throttled for sending faster than a deployable would.
type pacer struct {
	mu   sync.Mutex
	next time.Time
}

// wait holds a call costing units until the calls before it have had their share of the rate. A
// cancelled call is not held, so it reaches the port at once.
func (p *pacer) wait(ctx context.Context, units float64) {
	perUnit := time.Duration(float64(time.Second) / (gmail.Profile{}.BudgetPerSecond() / 2))
	p.mu.Lock()
	start := time.Now()
	if p.next.After(start) {
		start = p.next
	}
	p.next = start.Add(time.Duration(units * float64(perUnit)))
	p.mu.Unlock()
	timer := time.NewTimer(time.Until(start))
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

// paced is the adapter behind the pacer, each call held at the cost the profile declares for it.
type paced struct {
	port mail.Port[context.Context]
	pace *pacer
}

func (p paced) hold(ctx context.Context, op mail.Operation, messages int) {
	p.pace.wait(ctx, gmail.Profile{}.Cost(mail.ProviderOp{Operation: op, Messages: messages}).Weight)
}

func (p paced) ListThreads(ctx context.Context, q mail.Query, page mail.PageToken) (mail.Page[mail.ThreadMetadata], error) {
	p.hold(ctx, mail.OpListThreads, 0)
	return p.port.ListThreads(ctx, q, page)
}

func (p paced) GetThreadMetadata(ctx context.Context, threadID string) (mail.ThreadMetadata, error) {
	p.hold(ctx, mail.OpGetThreadMetadata, 0)
	return p.port.GetThreadMetadata(ctx, threadID)
}

func (p paced) GetMessageMetadata(ctx context.Context, ids []string) ([]mail.MessageMetadata, error) {
	p.hold(ctx, mail.OpGetMessageMetadata, len(ids))
	return p.port.GetMessageMetadata(ctx, ids)
}

func (p paced) GetMessageBody(ctx context.Context, id string) (mail.MessageBody, error) {
	p.hold(ctx, mail.OpGetMessageBody, 1)
	return p.port.GetMessageBody(ctx, id)
}

func (p paced) ListLabels(ctx context.Context) ([]mail.Label, error) {
	p.hold(ctx, mail.OpListLabels, 0)
	return p.port.ListLabels(ctx)
}

func (p paced) EnsureLabel(ctx context.Context, path string) (mail.Label, error) {
	p.hold(ctx, mail.OpEnsureLabel, 0)
	return p.port.EnsureLabel(ctx, path)
}

func (p paced) Mutate(ctx context.Context, ops []mail.MutationOp) (mail.MutationResult, error) {
	p.hold(ctx, mail.OpMutate, len(ops))
	return p.port.Mutate(ctx, ops)
}

func (p paced) CurrentCursor(ctx context.Context) (mail.Cursor, error) {
	p.hold(ctx, mail.OpCurrentCursor, 0)
	return p.port.CurrentCursor(ctx)
}

func (p paced) ChangesSince(ctx context.Context, c mail.Cursor) (mail.ChangeSet, error) {
	p.hold(ctx, mail.OpChangesSince, 0)
	return p.port.ChangesSince(ctx, c)
}

func (p paced) EnumerateAll(ctx context.Context, page mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
	p.hold(ctx, mail.OpEnumerateAll, 0)
	return p.port.EnumerateAll(ctx, page)
}

func (p paced) RateProfile() mail.RateLimitProfile[context.Context] { return p.port.RateProfile() }

// The quota units the harness's own requests are charged
// (https://developers.google.com/workspace/gmail/api/reference/quota).
const (
	unitsGetProfile     = 1
	unitsLabelsList     = 1
	unitsLabelsCreate   = 5
	unitsMessagesInsert = 25
	unitsMessagesList   = 5
	unitsMessagesModify = 5
	unitsMessagesTrash  = 20
)

// apiBase is the root of the harness's requests, the account the access token belongs to.
const apiBase = "https://gmail.googleapis.com/gmail/v1/users/me/"

// searchableWithin bounds how long an inserted message may take to appear in Gmail's search.
const searchableWithin = 2 * time.Minute

// reserve is how long before the test's deadline the run stops, inside the suite's cases and the
// harness's own waits alike, so every case it ran cleans up before the test binary is stopped.
const reserve = 10 * time.Minute

// provider is the harness's own way into the test account, outside the port, for what only the
// provider's side does to a mailbox, adding mail and the labels it carries. It holds the same grant
// as the adapter.
type provider struct {
	client *http.Client
	tokens *gmail.TokenSource
	pace   *pacer
	// labels maps a label's name to its identifier, for the labels the harness has seen.
	labels map[string]string
}

// thread is the first message the harness inserted under a thread key, which later messages under
// the key reply to.
type thread struct {
	id        string
	messageID string
}

// call sends one request and decodes a successful response into out. It returns the response status.
func (p *provider) call(ctx context.Context, method, path string, query url.Values, in, out any, units float64) (int, error) {
	p.pace.wait(ctx, units)
	token, err := p.tokens.AccessToken(ctx)
	if err != nil {
		return 0, err
	}
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(b)
	}
	target := apiBase + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := p.client.Do(req)
	if err != nil {
		return 0, err
	}
	data, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err := errors.Join(readErr, res.Body.Close()); err != nil {
		return res.StatusCode, err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return res.StatusCode, fmt.Errorf("%s %s answered %s: %s", method, path, res.Status, bytes.TrimSpace(data))
	}
	if out == nil {
		return res.StatusCode, nil
	}
	return res.StatusCode, json.Unmarshal(data, out)
}

// listLabels reads every label's name and identifier.
func (p *provider) listLabels(ctx context.Context) error {
	var raw struct {
		Labels []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"labels"`
	}
	if _, err := p.call(ctx, http.MethodGet, "labels", nil, nil, &raw, unitsLabelsList); err != nil {
		return err
	}
	for _, l := range raw.Labels {
		p.labels[l.Name] = l.ID
	}
	return nil
}

// labelID returns the identifier of the label at path, creating the label when the account lacks
// it. A system label's name is its identifier.
func (p *provider) labelID(ctx context.Context, path string) (string, error) {
	if id, ok := p.labels[path]; ok {
		return id, nil
	}
	if err := p.listLabels(ctx); err != nil {
		return "", err
	}
	if id, ok := p.labels[path]; ok {
		return id, nil
	}
	var created struct {
		ID string `json:"id"`
	}
	status, err := p.call(ctx, http.MethodPost, "labels", nil, map[string]string{"name": path}, &created, unitsLabelsCreate)
	if status == http.StatusConflict {
		if err := p.listLabels(ctx); err != nil {
			return "", err
		}
		if id, ok := p.labels[path]; ok {
			return id, nil
		}
	}
	if err != nil {
		return "", err
	}
	p.labels[path] = created.ID
	return created.ID, nil
}

// insert adds m to the account through messages.insert, dated by its Date header, with its labels
// and flags, in the thread of the message inserted earlier under the same thread key. It returns
// the message's identifier and its Message-ID header.
func (p *provider) insert(ctx context.Context, m contract.Message, threads map[string]thread) (string, string, error) {
	md := m.Metadata
	var labelIDs []string
	for _, path := range md.Labels {
		id, err := p.labelID(ctx, path)
		if err != nil {
			return "", "", err
		}
		labelIDs = append(labelIDs, id)
	}
	if !md.Flags.Read {
		labelIDs = append(labelIDs, "UNREAD")
	}
	if md.Flags.Starred {
		labelIDs = append(labelIDs, "STARRED")
	}
	messageID := strings.ToLower(rand.Text()) + "@contract.example"
	parent, inThread := threads[md.ThreadID]
	raw, err := rfc822(m, messageID, parent, inThread)
	if err != nil {
		return "", "", err
	}
	in := struct {
		Raw      string   `json:"raw"`
		LabelIDs []string `json:"labelIds,omitempty"`
		ThreadID string   `json:"threadId,omitempty"`
	}{base64.URLEncoding.EncodeToString(raw), labelIDs, parent.id}
	var out struct {
		ID       string `json:"id"`
		ThreadID string `json:"threadId"`
	}
	query := url.Values{"internalDateSource": {"dateHeader"}}
	if _, err := p.call(ctx, http.MethodPost, "messages", query, in, &out, unitsMessagesInsert); err != nil {
		return "", "", err
	}
	if !inThread {
		threads[md.ThreadID] = thread{id: out.ThreadID, messageID: messageID}
	}
	return out.ID, messageID, nil
}

// discard takes the labels a case's message was inserted with off it and moves it to the trash,
// once the case has ended. The labels come off first, so a message a failed cleanup leaves behind
// is at least out of every label a later run lists.
func (p *provider) discard(t *testing.T, id string, labels []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 2*time.Minute)
	defer cancel()
	var remove []string
	for _, path := range labels {
		if labelID, ok := p.labels[path]; ok {
			remove = append(remove, labelID)
		}
	}
	path := "messages/" + url.PathEscape(id)
	if len(remove) > 0 {
		in := map[string][]string{"removeLabelIds": remove}
		if _, err := p.call(ctx, http.MethodPost, path+"/modify", nil, in, nil, unitsMessagesModify); err != nil {
			t.Errorf("taking the labels off the message %s the case inserted: %v", id, err)
			return
		}
	}
	if _, err := p.call(ctx, http.MethodPost, path+"/trash", nil, nil, nil, unitsMessagesTrash); err != nil {
		t.Errorf("moving the message %s the case inserted to the trash: %v", id, err)
	}
}

// searchable waits until Gmail's search finds the message with the Message-ID header messageID.
// It stops waiting once the reserve before the test's deadline is reached, and the suite then fails
// the case, so the case's cleanups still run.
func (p *provider) searchable(t *testing.T, messageID, id string) error {
	ctx := t.Context()
	deadline := time.Now().Add(searchableWithin)
	query := url.Values{"q": {"rfc822msgid:" + messageID}, "includeSpamTrash": {"true"}, "maxResults": {"1"}}
	for {
		var found struct {
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
		}
		if _, err := p.call(ctx, http.MethodGet, "messages", query, nil, &found, unitsMessagesList); err != nil {
			return err
		}
		if len(found.Messages) > 0 && found.Messages[0].ID == id {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the search did not find the inserted message %s within %s", id, searchableWithin)
		}
		if end, ok := t.Deadline(); ok && time.Until(end) < reserve {
			return fmt.Errorf("stopped waiting for the search %s before the test's deadline, so the case's cleanups run", reserve)
		}
		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// rfc822 writes m as a message. Each part's content is base64, so its bytes arrive exactly as the
// fixture holds them, and each attachment the metadata names is a part of its own.
func rfc822(m contract.Message, messageID string, parent thread, inThread bool) ([]byte, error) {
	md := m.Metadata
	var b bytes.Buffer
	header := func(name, value string) { fmt.Fprintf(&b, "%s: %s\r\n", name, value) }
	header("From", address(md.From))
	header("To", addressList(md.To))
	if len(md.Cc) > 0 {
		header("Cc", addressList(md.Cc))
	}
	header("Subject", mime.QEncoding.Encode("utf-8", md.Subject))
	header("Date", time.UnixMilli(int64(md.Date)).UTC().Format(time.RFC1123Z))
	header("Message-ID", "<"+messageID+">")
	if inThread {
		header("In-Reply-To", "<"+parent.messageID+">")
		header("References", "<"+parent.messageID+">")
	}
	if md.ListID != "" {
		header("List-Id", "<"+md.ListID+">")
	}
	header("MIME-Version", "1.0")

	type part struct {
		contentType string
		disposition string
		content     []byte
	}
	var parts []part
	if m.Body.Text != "" {
		parts = append(parts, part{contentType: `text/plain; charset="utf-8"`, content: []byte(m.Body.Text)})
	}
	if m.Body.HTML != "" {
		parts = append(parts, part{contentType: `text/html; charset="utf-8"`, content: []byte(m.Body.HTML)})
	}
	for _, name := range md.AttachmentNames {
		parts = append(parts, part{
			contentType: mime.FormatMediaType("application/octet-stream", map[string]string{"name": name}),
			disposition: mime.FormatMediaType("attachment", map[string]string{"filename": name}),
			content:     []byte(name),
		})
	}
	write := func(p part) {
		header("Content-Type", p.contentType)
		if p.disposition != "" {
			header("Content-Disposition", p.disposition)
		}
		header("Content-Transfer-Encoding", "base64")
		b.WriteString("\r\n")
		encoded := base64.StdEncoding.EncodeToString(p.content)
		for len(encoded) > 76 {
			b.WriteString(encoded[:76] + "\r\n")
			encoded = encoded[76:]
		}
		b.WriteString(encoded + "\r\n")
	}
	switch len(parts) {
	case 0:
		return nil, fmt.Errorf("the message keyed %q has no body and no attachment", md.ID)
	case 1:
		write(parts[0])
	default:
		boundary := strings.ToLower(rand.Text())
		header("Content-Type", mime.FormatMediaType("multipart/mixed", map[string]string{"boundary": boundary}))
		b.WriteString("\r\n")
		for _, p := range parts {
			b.WriteString("--" + boundary + "\r\n")
			write(p)
		}
		b.WriteString("--" + boundary + "--\r\n")
	}
	return b.Bytes(), nil
}

func address(a mail.Address) string {
	return (&netmail.Address{Name: a.Name, Address: a.Email}).String()
}

func addressList(list []mail.Address) string {
	out := make([]string, len(list))
	for i, a := range list {
		out[i] = address(a)
	}
	return strings.Join(out, ", ")
}
