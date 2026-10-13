package gmail

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail/consent"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail/core"
)

// historyPage is the most history records one history.list returns. Gmail charges the same for any
// page size.
const historyPage = 500

// maxResponse bounds how much of a Gmail response body is read, for every request but a body read,
// whose caps derive from the port's bound (ADR-0131). A response past it is the provider's failure.
const maxResponse = 64 << 20

// Tokens hands out access tokens for the account. *TokenSource is the one the deployables use. A
// port call reads only *TokenSource's report of the token endpoint's refusal as a refused
// credential, and any other error as the provider failing the request, apart from a call cancelled
// or past its deadline, which returns the context's error.
type Tokens interface {
	AccessToken(ctx context.Context) (string, error)
}

// Config sets up an Adapter. Every field is required.
type Config struct {
	// Account is the account the mailbox belongs to. Every value the Adapter returns carries it.
	Account string
	Client  *http.Client
	Tokens  Tokens
	// Metrics are the process's adapter series, which every Adapter in the process shares.
	Metrics *Metrics
}

// Adapter is the Gmail adapter, one account's mailbox reached through Gmail's API.
type Adapter struct {
	account string
	client  *http.Client
	tokens  Tokens
	metrics *Metrics
}

var _ mail.Port[context.Context] = (*Adapter)(nil)

// New returns an Adapter for cfg.
func New(cfg Config) (*Adapter, error) {
	switch {
	case cfg.Account == "":
		return nil, errors.New("gmail: the account is empty")
	case cfg.Client == nil || cfg.Tokens == nil || cfg.Metrics == nil:
		return nil, errors.New("gmail: the HTTP client, the token source and the metrics are required")
	}
	return &Adapter{account: cfg.Account, client: cfg.Client, tokens: cfg.Tokens, metrics: cfg.Metrics}, nil
}

// call is one port call's requests, held to the cost the rate profile declares for the call, so the
// lease the rate limiter issued for it covers every request it sends (ADR-0023).
type call struct {
	adapter  *Adapter
	declared int
	spent    int
}

// begin starts a port call naming messages messages, as the rate profile counts them.
func (a *Adapter) begin(op mail.Operation, messages int) *call {
	cost := Profile{}.Cost(mail.ProviderOp{Operation: op, Messages: messages})
	return &call{adapter: a, declared: int(cost.Weight)}
}

// within reports whether a request costing units keeps a call that has spent spent within its
// declared cost.
func within(spent, units, declared int) bool { return spent+units <= declared }

// do sends a request of the call, and refuses without sending one that would take the call past its
// declared cost.
func (c *call) do(ctx context.Context, r request) ([]byte, error) {
	if !within(c.spent, r.Units, c.declared) {
		return nil, fmt.Errorf("gmail: a %s %s request would take the call past its declared %d units: %w",
			r.Method, r.Path, c.declared, mail.ErrInvalid)
	}
	c.spent += r.Units
	return c.adapter.send(ctx, r)
}

// send sends one request and returns the body of a successful response. The request's cost is
// counted once the request is built and before it leaves, so a request that fails in any way after
// that is counted too (ADR-0077).
func (a *Adapter) send(ctx context.Context, r request) ([]byte, error) {
	token, err := a.tokens.AccessToken(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("gmail: obtaining an access token: %w", tokenError(err))
	}
	var body io.Reader
	if r.Body != "" {
		body = strings.NewReader(r.Body)
	}
	req, err := http.NewRequestWithContext(withReadCap(ctx, r.Cap), r.Method, r.URL(), body)
	if err != nil {
		return nil, fmt.Errorf("gmail: building a request: %w", errors.Join(mail.ErrInvalid, err))
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	a.metrics.countRequest(a.account, r.Units)
	start := time.Now()
	data, err := a.exchange(ctx, req)
	a.metrics.observeRequest(r.Endpoint, outcomeOf(err), time.Since(start).Seconds())
	return data, err
}

// outcomeOf returns how a request ended, as its latency is labelled, throttled when the provider
// throttled it, failed for any other error, and ok otherwise (ADR-0125).
func outcomeOf(err error) string {
	if _, throttled := mail.Throttled(err); throttled {
		return "throttled"
	}
	if err != nil {
		return "failed"
	}
	return "ok"
}

// exchange sends req and reads its answer, returning the answer's body, or the port error for a
// request that failed or was refused. A request carrying no read cap is refused as the provider's
// failure before it is sent, since only a request no builder made has none.
func (a *Adapter) exchange(ctx context.Context, req *http.Request) ([]byte, error) {
	if _, ok := readCapOf(req); !ok {
		return nil, fmt.Errorf("gmail: a request with no read cap: %w", mail.ErrProvider)
	}
	res, err := a.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("gmail: sending a request: %w", errors.Join(mail.ErrProvider, err))
	}
	return answer(res)
}

// answer reads res, returning its body or the port error. The answer is read to the cap its request
// carries, the very request exchange checked before sending it, and one byte more, so an answer past
// it is told apart rather than truncated. A body read's answer past its cap is the body decision's
// PastCap (ADR-0131), and any other answer past maxResponse is the provider's failure.
func answer(res *http.Response) ([]byte, error) {
	c, ok := readCapOf(res.Request)
	if !ok {
		return nil, errors.Join(fmt.Errorf("gmail: an answer to a request with no read cap: %w", mail.ErrProvider), res.Body.Close())
	}
	data, over, readErr := capped(res.Body, c.Bytes)
	if err := errors.Join(readErr, res.Body.Close()); err != nil {
		return nil, fmt.Errorf("gmail: reading a response: %w", errors.Join(mail.ErrProvider, err))
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, parseError(res.StatusCode, res.Header.Get("Retry-After"), time.Now(), data)
	}
	switch {
	case over && c.Refuses:
		return nil, fmt.Errorf("gmail: the answer passes its cap of %d bytes, as %v: %w", c.Bytes, core.PastCap, portError(core.PastCap))
	case over:
		return nil, fmt.Errorf("gmail: the response passes its cap of %d bytes: %w", c.Bytes, mail.ErrProvider)
	}
	return data, nil
}

// readCapKey is the context key a request's read cap travels under, on the *http.Request itself, so
// the cap checked before the request is sent is the cap its answer is read to. The cap rides that one
// request's context, passed with it, not an ambient value (ADR-0040), and that carrier is what lets
// a test observe the cap a sent request carries.
type readCapKey struct{}

// withReadCap returns ctx carrying c.
func withReadCap(ctx context.Context, c readCap) context.Context {
	return context.WithValue(ctx, readCapKey{}, c)
}

// readCapOf returns the read cap req carries, and false when it carries none or one of no bytes.
func readCapOf(req *http.Request) (readCap, bool) {
	if req == nil {
		return readCap{}, false
	}
	c, ok := req.Context().Value(readCapKey{}).(readCap)
	return c, ok && c.Bytes > 0
}

// capped reads r to ceiling bytes and one more, so a response past its cap is told apart rather than
// truncated, and reports whether it passed.
func capped(r io.Reader, ceiling int) ([]byte, bool, error) {
	data, err := io.ReadAll(io.LimitReader(r, int64(ceiling)+1))
	if err == nil && len(data) > ceiling {
		return data[:ceiling], true, nil
	}
	return data, false, err
}

// tokenError is the port error for err, a failure to obtain an access token, read the way the token
// source reads its attempt's outcome (ADR-0097). A refusal from the token endpoint is a credential
// the provider refused. Any other failure, an error from a Tokens other than *TokenSource included,
// is the provider failing the request, since only the token endpoint's own answer shows a refusal.
// send returns the context's error instead, without asking, for a call cancelled or past its
// deadline.
func tokenError(err error) error {
	if consent.Outcome(err) == mail.AuthRefused {
		return errors.Join(mail.ErrAuthentication, err)
	}
	return errors.Join(mail.ErrProvider, err)
}

// labels reads the account's labels.
func (c *call) labels(ctx context.Context) (labelTable, error) {
	body, err := c.do(ctx, labelsRequest())
	if err != nil {
		return labelTable{}, err
	}
	return parseLabels(body)
}

// mapped maps messages read through the metadata mask, reading the labels after the messages, so a
// label a message carries is listed unless it was deleted meanwhile. No messages read no labels.
func (c *call) mapped(ctx context.Context, read []gmailMessage) ([]mail.MessageMetadata, error) {
	if len(read) == 0 {
		return nil, nil
	}
	labels, err := c.labels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]mail.MessageMetadata, len(read))
	for i, m := range read {
		out[i] = metadata(c.adapter.account, labels, m)
	}
	return out, nil
}

// metadata reads each message ids names through the metadata mask, leaving out any the account does
// not have.
func (c *call) metadata(ctx context.Context, ids []string) ([]mail.MessageMetadata, error) {
	var read []gmailMessage
	for _, id := range ids {
		if !validID(id) {
			continue
		}
		body, err := c.do(ctx, metadataRequest(id))
		if errors.Is(err, mail.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		m, err := parseMessage(body)
		if err != nil {
			return nil, err
		}
		read = append(read, m)
	}
	return c.mapped(ctx, read)
}

// threads reads the threads threadIDs names, each with every one of its messages, in that order,
// leaving out any the account does not have.
func (c *call) threads(ctx context.Context, threadIDs []string) ([]mail.ThreadMetadata, error) {
	var read []gmailMessage
	for _, id := range threadIDs {
		if !validID(id) {
			continue
		}
		body, err := c.do(ctx, threadRequest(id))
		if errors.Is(err, mail.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		messages, err := parseThread(body)
		if err != nil {
			return nil, err
		}
		read = append(read, messages...)
	}
	messages, err := c.mapped(ctx, read)
	if err != nil {
		return nil, err
	}
	return threads(c.adapter.account, threadIDs, messages), nil
}

// ListThreads implements the port. A page holds threadsPerPage threads, so its worst case fits the
// cost the rate profile declares.
func (a *Adapter) ListThreads(ctx context.Context, q mail.Query, page mail.PageToken) (mail.Page[mail.ThreadMetadata], error) {
	if err := ctx.Err(); err != nil {
		return mail.Page[mail.ThreadMetadata]{}, err
	}
	if !q.Valid() {
		return mail.Page[mail.ThreadMetadata]{}, fmt.Errorf("gmail: the query is not valid: %w", mail.ErrInvalid)
	}
	gmailToken, err := readPageToken(threadListing, page)
	if err != nil {
		return mail.Page[mail.ThreadMetadata]{}, err
	}
	c := a.begin(mail.OpListThreads, 0)
	labels, err := c.labels(ctx)
	if err != nil {
		return mail.Page[mail.ThreadMetadata]{}, err
	}
	s, err := compileQuery(q, labels)
	if err != nil || s.None {
		return mail.Page[mail.ThreadMetadata]{}, err
	}
	body, err := c.do(ctx, threadsRequest(s, gmailToken, threadsPerPage))
	if err != nil {
		return mail.Page[mail.ThreadMetadata]{}, err
	}
	l, err := parseListing(body)
	if err != nil {
		return mail.Page[mail.ThreadMetadata]{}, err
	}
	all, err := c.threads(ctx, l.Threads)
	if err != nil {
		return mail.Page[mail.ThreadMetadata]{}, err
	}
	return mail.Page[mail.ThreadMetadata]{Items: selected(q, all), Next: pageToken(threadListing, l.Next)}, nil
}

// GetThreadMetadata implements the port.
func (a *Adapter) GetThreadMetadata(ctx context.Context, threadID string) (mail.ThreadMetadata, error) {
	if err := ctx.Err(); err != nil {
		return mail.ThreadMetadata{}, err
	}
	got, err := a.begin(mail.OpGetThreadMetadata, 0).threads(ctx, []string{threadID})
	if err != nil {
		return mail.ThreadMetadata{}, err
	}
	if len(got) == 0 {
		return mail.ThreadMetadata{}, fmt.Errorf("gmail: thread %q: %w", threadID, mail.ErrNotFound)
	}
	return got[0], nil
}

// GetMessageMetadata implements the port. It refuses more than idsPerMetadataCall identifiers,
// since a larger call costs more than the rate limiter issues at once, and the caller splits them.
func (a *Adapter) GetMessageMetadata(ctx context.Context, ids []string) ([]mail.MessageMetadata, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(ids) > idsPerMetadataCall {
		return nil, fmt.Errorf("gmail: %d identifiers in one call, at most %d: %w", len(ids), idsPerMetadataCall, mail.ErrInvalid)
	}
	return a.begin(mail.OpGetMessageMetadata, len(ids)).metadata(ctx, ids)
}

// GetMessageBody implements the port. With the empty part it reads the message and returns its
// rendered part, refuses one whose declared size passes the bound without downloading it, and
// answers a part held apart within the bound with a HeldApartError naming it. With a part it named,
// it reads that part with messages.attachments.get (ADR-0131).
func (a *Adapter) GetMessageBody(ctx context.Context, id string, part mail.BodyPart) (mail.MessageBody, error) {
	if err := ctx.Err(); err != nil {
		return mail.MessageBody{}, err
	}
	if !validID(id) {
		return mail.MessageBody{}, fmt.Errorf("gmail: message %q: %w", id, mail.ErrNotFound)
	}
	c := a.begin(mail.OpGetMessageBody, 1)
	if part != "" {
		h, err := readBodyPart(part, id)
		if err != nil {
			return mail.MessageBody{}, err
		}
		body, err := c.do(ctx, partRequest(id, h.attachment))
		if err != nil {
			return mail.MessageBody{}, err
		}
		return partBody(a.account, h, body)
	}
	body, err := c.do(ctx, bodyRequest(id))
	if err != nil {
		return mail.MessageBody{}, err
	}
	m, err := parseMessage(body)
	if err != nil {
		return mail.MessageBody{}, err
	}
	return messageBody(a.account, m)
}

// ListLabels implements the port.
func (a *Adapter) ListLabels(ctx context.Context) ([]mail.Label, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	labels, err := a.begin(mail.OpListLabels, 0).labels(ctx)
	if err != nil {
		return nil, err
	}
	return labels.canonicalLabels(a.account), nil
}

// EnsureLabel implements the port. A label another caller creates between the listing and the
// creation makes Gmail answer 409, and the label is then read from a second listing.
func (a *Adapter) EnsureLabel(ctx context.Context, path string) (mail.Label, error) {
	if err := ctx.Err(); err != nil {
		return mail.Label{}, err
	}
	if path == "" {
		return mail.Label{}, fmt.Errorf("gmail: the label path is empty: %w", mail.ErrInvalid)
	}
	c := a.begin(mail.OpEnsureLabel, 0)
	labels, err := c.labels(ctx)
	if err != nil {
		return mail.Label{}, err
	}
	if _, ok := labels.idByPath[path]; ok {
		return mail.Label{AccountID: a.account, Path: path}, nil
	}
	req, err := createLabelRequest(path)
	if err != nil {
		return mail.Label{}, err
	}
	body, err := c.do(ctx, req)
	var status statusError
	if errors.As(err, &status) && status.status == http.StatusConflict {
		if labels, err = c.labels(ctx); err != nil {
			return mail.Label{}, err
		}
		if _, ok := labels.idByPath[path]; ok {
			return mail.Label{AccountID: a.account, Path: path}, nil
		}
		return mail.Label{}, fmt.Errorf("gmail: label %q conflicts with a label of another name: %w", path, mail.ErrInvalid)
	}
	if err != nil {
		return mail.Label{}, err
	}
	created, err := parseLabel(body)
	if err != nil {
		return mail.Label{}, err
	}
	return mail.Label{AccountID: a.account, Path: created}, nil
}

// Mutate implements the port. It refuses more than opsPerMutation ops, since a larger call costs
// more than the rate limiter issues at once, and the caller splits them.
func (a *Adapter) Mutate(ctx context.Context, ops []mail.MutationOp) (mail.MutationResult, error) {
	for i, op := range ops {
		if !op.Built() {
			return mail.MutationResult{}, fmt.Errorf("gmail: op %d was not built by a constructor: %w", i, mail.ErrInvalid)
		}
	}
	if len(ops) > opsPerMutation {
		return mail.MutationResult{}, fmt.Errorf("gmail: %d ops in one call, at most %d: %w", len(ops), opsPerMutation, mail.ErrInvalid)
	}
	if err := ctx.Err(); err != nil {
		return mail.MutationResult{}, err
	}
	c := a.begin(mail.OpMutate, len(ops))
	labels, err := c.labels(ctx)
	if err != nil {
		return mail.MutationResult{}, err
	}
	result := mail.MutationResult{Errors: make([]error, len(ops))}
	var stop error
	for i, op := range ops {
		if stop != nil {
			result.Errors[i] = stop
			continue
		}
		result.Errors[i] = c.apply(ctx, op, labels)
		if stopsBatch(result.Errors[i]) {
			stop = result.Errors[i]
		}
	}
	return result, nil
}

// apply sends one op. An op that changes nothing, such as one removing only a label the account
// lacks, reads the message and sends nothing else, so an op on a message the account does not have
// is still not found.
func (c *call) apply(ctx context.Context, op mail.MutationOp, labels labelTable) error {
	change, err := opChange(op, labels)
	if err != nil {
		return err
	}
	id := op.MessageID()
	if !validID(id) {
		return fmt.Errorf("gmail: message %q: %w", id, mail.ErrNotFound)
	}
	if change.none() {
		body, err := c.do(ctx, messageRequest(id, formatMinimal))
		if err != nil {
			return err
		}
		_, err = parseMessage(body)
		return err
	}
	req, err := modifyRequest(id, change.Add, change.Remove)
	if err != nil {
		return err
	}
	_, err = c.do(ctx, req)
	return err
}

// CurrentCursor implements the port.
func (a *Adapter) CurrentCursor(ctx context.Context) (mail.Cursor, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	body, err := a.begin(mail.OpCurrentCursor, 0).do(ctx, profileRequest())
	if err != nil {
		return "", err
	}
	p, err := parseProfile(body)
	if err != nil {
		return "", err
	}
	return cursor(p.historyID), nil
}

// ChangesSince implements the port. It reads one page of Gmail's history.
func (a *Adapter) ChangesSince(ctx context.Context, cur mail.Cursor) (mail.ChangeSet, error) {
	if err := ctx.Err(); err != nil {
		return mail.ChangeSet{}, err
	}
	start, err := readCursor(cur)
	if err != nil {
		return mail.ChangeSet{}, err
	}
	c := a.begin(mail.OpChangesSince, 0)
	body, err := c.do(ctx, historyRequest(start, historyPage))
	if errors.Is(err, mail.ErrNotFound) {
		return mail.ChangeSet{}, fmt.Errorf("gmail: cursor %q: %w", cur, errors.Join(mail.ErrCursorGap, err))
	}
	if err != nil {
		return mail.ChangeSet{}, err
	}
	h, err := parseHistory(body)
	if err != nil {
		return mail.ChangeSet{}, err
	}
	labels, err := c.labels(ctx)
	if err != nil {
		return mail.ChangeSet{}, err
	}
	return changes(a.account, labels, h), nil
}

// EnumerateAll implements the port. A page holds messagesPerPage messages, so its worst case fits
// the cost the rate profile declares. Every page carries the mailbox's total from a profile read
// before the listing (ADR-0095).
func (a *Adapter) EnumerateAll(ctx context.Context, page mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
	if err := ctx.Err(); err != nil {
		return mail.Page[mail.MessageMetadata]{}, err
	}
	gmailToken, err := readPageToken(messageListing, page)
	if err != nil {
		return mail.Page[mail.MessageMetadata]{}, err
	}
	c := a.begin(mail.OpEnumerateAll, 0)
	body, err := c.do(ctx, profileRequest())
	if err != nil {
		return mail.Page[mail.MessageMetadata]{}, err
	}
	p, err := parseProfile(body)
	if err != nil {
		return mail.Page[mail.MessageMetadata]{}, err
	}
	body, err = c.do(ctx, messagesRequest(gmailToken, messagesPerPage))
	if err != nil {
		return mail.Page[mail.MessageMetadata]{}, err
	}
	l, err := parseListing(body)
	if err != nil {
		return mail.Page[mail.MessageMetadata]{}, err
	}
	ids := make([]string, len(l.Messages))
	for i, m := range l.Messages {
		ids[i] = m.ID
	}
	items, err := c.metadata(ctx, ids)
	if err != nil {
		return mail.Page[mail.MessageMetadata]{}, err
	}
	return mail.Page[mail.MessageMetadata]{Items: items, Next: pageToken(messageListing, l.Next), Total: p.total()}, nil
}

// RateProfile implements the port.
func (a *Adapter) RateProfile() mail.RateLimitProfile[context.Context] { return Profile{} }
