//go:build integration

package provider

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"

	coremail "github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// testToken is a token holding a credential and making no attempt of its own.
type testToken []byte

func (t testToken) Credential() []byte              { return t }
func (testToken) LastAttempt() coremail.AuthAttempt { return coremail.AuthAttempt{} }

// refused is the credential the provider behind testAdapters refuses.
const refused = "refused"

// provider is the provider behind a test's adapters, the provider fake behind a schedule, which
// refuses every call made with the refused credential as a provider refusing it would, and lets the
// test's own schedule decide the rest. It counts the calls that reach it with each credential.
type provider struct {
	fake     *fake.Fake
	schedule fake.Schedule

	mu    sync.Mutex
	calls map[string]int
}

func newProvider(t *testing.T, account string, schedule fake.Schedule, messages ...fake.Message) *provider {
	t.Helper()
	f, err := fake.New(fake.Config{Account: account}, messages...)
	if err != nil {
		t.Fatal(err)
	}
	if schedule == nil {
		schedule = fake.Never()
	}
	return &provider{fake: f, schedule: schedule, calls: map[string]int{}}
}

// called returns how many calls reached the provider with credential.
func (p *provider) called(credential string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls[credential]
}

// adapters returns the list of adapters a test's caller is built over, one adapter under Gmail's
// name, whose facts the caller reads, calling p. As a real adapter does, it reports each call that
// reaches the provider to the series the caller hands it, at the call's cost.
func (p *provider) adapters() Adapters {
	return Adapters{byName: map[string]adapter{gmailName: {
		token: func(_ coremail.OAuthClient, credential []byte) tokenSource { return testToken(credential) },
		mailbox: func(account string, source tokenSource, requests Requests) (adapterMailbox, error) {
			credential := string(source.Credential())
			return fake.Throttle(p.fake, func(c fake.Call) error {
				p.mu.Lock()
				p.calls[credential]++
				p.mu.Unlock()
				requests.Sent(account, int(c.Cost.Weight))
				if credential == refused {
					return coremail.ErrAuthentication
				}
				return p.schedule(c)
			}, time.Now), nil
		},
	}}}
}

// token makes a token holding credential through the adapters.
func token(t *testing.T, a Adapters, credential string) *Token {
	t.Helper()
	tok, err := a.NewToken(gmailName, coremail.OAuthClient{ID: "client", Secret: "secret"}, []byte(credential))
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// caller returns a caller over a in class with timeout, on a registry of its own.
func caller(t *testing.T, a Adapters, class Class, timeout time.Duration) *Caller {
	t.Helper()
	return callerOn(t, a, class, timeout, prometheus.NewRegistry())
}

// callerOn returns a caller over a in class with timeout, registering its series on reg.
func callerOn(t *testing.T, a Adapters, class Class, timeout time.Duration, reg prometheus.Registerer) *Caller {
	t.Helper()
	c, err := NewCaller(a, reg, spenders(t), Options{Class: class, Timeout: timeout})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// open returns the account's mailbox through c over conn, the account and its token filled in.
func open(t *testing.T, c *Caller, conn Connection) mail.Mailbox {
	t.Helper()
	conn.Provider = gmailName
	m, err := c.Mailbox(conn)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// rateState is what the account's rate state shows of its rate.
type rateState struct {
	Rate, Target float64
	Samples      int
	Backoff      bool
}

func readRate(t *testing.T, conn *pgx.Conn, account string) rateState {
	t.Helper()
	var s rateState
	err := conn.QueryRow(t.Context(), `SELECT current_rate, target_rate, coalesce(cardinality(latency_samples), 0),
		coalesce(backoff_until > clock_timestamp(), false) FROM rate_state WHERE account_id = $1`, account).Scan(&s.Rate, &s.Target, &s.Samples, &s.Backoff)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// grant is one lease the account was granted, its class and its tokens.
type grant struct {
	Class  string
	Tokens float64
}

func readGrants(t *testing.T, conn *pgx.Conn, account string) []grant {
	t.Helper()
	rows, err := conn.Query(t.Context(), "SELECT class, tokens FROM rate_grants WHERE account_id = $1 ORDER BY issued_at", account)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[grant])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// VERIFICATIONS' row for a call through the wrapper taking a lease at the account's target. Each
// operation of a mailbox takes one lease in the caller's class, of what the adapter's rate profile
// says the operation costs, and the account spends at its lowered target where it has one and at the
// default target otherwise, both fractions of the provider's ceiling, Gmail's 100 (ADR-0024,
// ADR-0025).
func TestEachCallLeasesItsCostInTheCallersClassAtTheAccountsTarget(t *testing.T) {
	conn := superuser(t)
	for _, c := range []struct {
		name    string
		class   Class
		stored  string
		lowered bool
		target  float64
	}{
		{"batch at the default target", Batch, "batch", false, 50},
		{"interactive at a lowered target", Interactive, "interactive", true, 20},
		{"sync at the default target", Sync, "sync", false, 50},
	} {
		t.Run(c.name, func(t *testing.T) {
			account := newAccount(t, conn)
			p := newProvider(t, account, nil)
			a := p.adapters()
			m := open(t, caller(t, a, c.class, time.Minute), Connection{Account: account, Token: token(t, a, "good"), Target: 0.2, Lowered: c.lowered})
			if _, err := m.ListLabels(t.Context()); err != nil {
				t.Fatal(err)
			}
			if _, err := m.GetMessageMetadata(t.Context(), []string{"a", "b", "c"}); err != nil {
				t.Fatal(err)
			}
			// The fake prices a call at one unit per message it names, and at least one.
			want := []grant{{c.stored, 1}, {c.stored, 3}}
			if diff := cmp.Diff(want, readGrants(t, conn, account), compare.Options); diff != "" {
				t.Errorf("the leases granted (-want +got):\n%s", diff)
			}
			if got := readRate(t, conn, account).Target; got != c.target {
				t.Errorf("the account spends at target %v, want %v", got, c.target)
			}
		})
	}
}

// VERIFICATIONS' row for a call through the wrapper waiting no later than its deadline. A lease the
// limiter cannot grant before the unit's deadline, here held back by a throttle's retry-after of ten
// seconds, is waited for until the deadline and no longer, the provider is not called, and the
// failure is the caller's own rather than the provider's (ADR-0130). With no deadline the same wait
// lasts as long as the caller's context.
func TestALeaseIsWaitedForNoLaterThanTheUnitsDeadline(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	retry := coremail.ThrottleSignal{RetryAfterMillis: 10_000, HasRetryAfter: true, Scope: coremail.ScopePerUser}
	p := newProvider(t, account, fake.ThrottleCalls(1, 1, retry))
	a := p.adapters()
	c := caller(t, a, Interactive, time.Minute)
	if _, err := open(t, c, Connection{Account: account, Token: token(t, a, "good")}).ListLabels(t.Context()); !errors.Is(err, coremail.ErrThrottled) {
		t.Fatalf("the first call answered %v, want the throttle", err)
	}

	const wait = 500 * time.Millisecond
	start := time.Now()
	m := open(t, c, Connection{Account: account, Token: token(t, a, "good"), WaitUntil: start.Add(wait)})
	_, err := m.ListLabels(t.Context())
	waited := time.Since(start)
	if err == nil || errors.Is(err, coremail.ErrProvider) || !strings.Contains(err.Error(), "no lease by the deadline") {
		t.Errorf("a lease past the deadline failed with %v, want the caller's own failure", err)
	}
	if waited < wait || waited > wait+2*time.Second {
		t.Errorf("the lease was waited for %v, want about %v", waited, wait)
	}
	if n := p.called("good"); n != 1 {
		t.Errorf("the provider was called %d times, want once, by the throttled call alone", n)
	}

	gaveUp, cancel := context.WithTimeout(t.Context(), wait)
	defer cancel()
	start = time.Now()
	_, err = open(t, c, Connection{Account: account, Token: token(t, a, "good")}).ListLabels(gaveUp)
	if !errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "no lease by the deadline") {
		t.Errorf("a lease with no deadline of its own failed with %v, want the caller's context's deadline", err)
	}
	if waited := time.Since(start); waited < wait {
		t.Errorf("a lease with no deadline of its own was waited for %v, less than the caller's context allowed", waited)
	}
}

// VERIFICATIONS' row for a call through the wrapper reporting its outcome. A call that succeeds adds
// its latency to the account's window, a throttle halves the rate and starts a backoff, a provider
// failure cuts the rate, and a refusal that is no failure of the provider, a body past the bound,
// leaves the rate where it was (ADR-0024, ADR-0131).
func TestEachCallTellsTheLimiterHowItWent(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	var fail error
	p := newProvider(t, account, func(fake.Call) error { return fail },
		fake.Message{Metadata: coremail.MessageMetadata{ID: "past", ThreadID: "t1"}, Body: coremail.MessageBody{Text: strings.Repeat("a", coremail.MaxBodyBytes+1)}})
	a := p.adapters()
	m := open(t, caller(t, a, Batch, time.Minute), Connection{Account: account, Token: token(t, a, "good")})

	if _, err := m.ListLabels(t.Context()); err != nil {
		t.Fatal(err)
	}
	start := readRate(t, conn, account)
	if start.Samples != 1 || start.Backoff {
		t.Errorf("after a call that succeeded the rate state is %+v, want its latency in the window and no backoff", start)
	}

	if _, err := m.GetMessageBody(t.Context(), "past", ""); !errors.Is(err, coremail.ErrRefused) {
		t.Fatalf("the body past the bound answered %v", err)
	}
	if got := readRate(t, conn, account); got != start {
		t.Errorf("a refused body moved the rate state from %+v to %+v", start, got)
	}

	fail = coremail.ErrProvider
	if _, err := m.ListLabels(t.Context()); !errors.Is(err, coremail.ErrProvider) {
		t.Fatalf("the failing call answered %v", err)
	}
	failed := readRate(t, conn, account)
	if failed.Rate >= start.Rate {
		t.Errorf("a provider failure left the rate at %v from %v, want it cut", failed.Rate, start.Rate)
	}

	fail = coremail.ThrottleError{Signal: coremail.ThrottleSignal{RetryAfterMillis: 50, HasRetryAfter: true}}
	if _, err := m.ListLabels(t.Context()); !errors.Is(err, coremail.ErrThrottled) {
		t.Fatalf("the throttled call answered %v", err)
	}
	if got := readRate(t, conn, account); got.Rate != failed.Rate/2 || !got.Backoff {
		t.Errorf("a throttle left the rate state at %+v from rate %v, want the rate halved and a backoff", got, failed.Rate)
	}
}

// hanging is the fake whose label listing never answers until its caller gives up.
type hanging struct{ *fake.Fake }

func (hanging) ListLabels(ctx context.Context) ([]coremail.Label, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// VERIFICATIONS' row for a call through the wrapper running under its caller's timeout. A call that
// has not answered within the caller's timeout of its lease, while the caller still waits, is cut
// off as the provider's failure, which cuts the account's rate (ADR-0101).
func TestACallIsCutOffAtTheCallersTimeoutAsTheProvidersFailure(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	f, err := fake.New(fake.Config{Account: account})
	if err != nil {
		t.Fatal(err)
	}
	a := Adapters{byName: map[string]adapter{gmailName: {
		token:   func(_ coremail.OAuthClient, credential []byte) tokenSource { return testToken(credential) },
		mailbox: func(string, tokenSource, Requests) (adapterMailbox, error) { return hanging{f}, nil },
	}}}
	m := open(t, caller(t, a, Batch, 300*time.Millisecond), Connection{Account: account, Token: token(t, a, "good")})
	if _, err := m.ListThreads(t.Context(), coremail.All(), ""); err != nil {
		t.Fatal(err)
	}
	before := readRate(t, conn, account).Rate
	start := time.Now()
	_, err = m.ListLabels(t.Context())
	if !errors.Is(err, coremail.ErrProvider) {
		t.Errorf("a call that never answered failed with %v, want the provider's failure", err)
	}
	if waited := time.Since(start); waited < 300*time.Millisecond || waited > 5*time.Second {
		t.Errorf("the call was cut off after %v, want the caller's timeout of 300ms", waited)
	}
	cut := readRate(t, conn, account).Rate
	if cut >= before {
		t.Errorf("a call cut off left the account's rate at %v from %v, want it cut", cut, before)
	}

	gaveUp, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	if _, err := m.ListLabels(gaveUp); errors.Is(err, coremail.ErrProvider) || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a call its caller gave up on failed with %v, want the caller's deadline and not the provider's failure", err)
	}
	if after := readRate(t, conn, account).Rate; after != cut {
		t.Errorf("a call its caller gave up on moved the account's rate from %v to %v", cut, after)
	}
}

// VERIFICATIONS' row for a call through the wrapper trying once more on a refused credential. A call
// the provider refuses reads the account again, and when the account holds another credential the
// call is made once more over a token made from it, under a lease of its own, and the later calls use
// that token. When the account holds the refused credential or none, or the read fails, the refusal
// stands and the call is not made again. A call made again that is refused once more is not made a
// third time (ADR-0090, ADR-0106).
func TestARefusedCredentialIsReadAgainAndTheCallMadeOnceMore(t *testing.T) {
	conn := superuser(t)
	for _, c := range []struct {
		name       string
		again      string
		againErr   error
		succeeds   bool
		refusedFor int
		goodFor    int
	}{
		{"another credential", "good", nil, true, 1, 2},
		{"the refused credential again", "", nil, false, 1, 0},
		{"a read that fails", "", errors.New("the account's row could not be read"), false, 1, 0},
		{"another credential refused too", refused, nil, false, 2, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			account := newAccount(t, conn)
			p := newProvider(t, account, nil)
			a := p.adapters()
			reads := 0
			again := func(context.Context) (*Token, error) {
				reads++
				if c.againErr != nil || c.again == "" {
					return nil, c.againErr
				}
				return token(t, a, c.again), nil
			}
			m := open(t, caller(t, a, Sync, time.Minute), Connection{Account: account, Token: token(t, a, refused), Again: again})
			_, err := m.CurrentCursor(t.Context())
			if c.succeeds != (err == nil) {
				t.Fatalf("the call answered %v, want it to succeed: %v", err, c.succeeds)
			}
			if !c.succeeds && !errors.Is(err, coremail.ErrAuthentication) {
				t.Errorf("the call answered %v, want the refusal", err)
			}
			if c.againErr != nil && !errors.Is(err, c.againErr) {
				t.Errorf("the call answered %v, want the read's failure joined to the refusal", err)
			}
			if c.succeeds {
				if _, err := m.ListLabels(t.Context()); err != nil {
					t.Errorf("a later call over the token made again answered %v", err)
				}
			}
			if reads != 1 {
				t.Errorf("the account was read again %d times, want once", reads)
			}
			if got, want := [2]int{p.called(refused), p.called("good")}, [2]int{c.refusedFor, c.goodFor}; got != want {
				t.Errorf("the provider was called %v times with the refused and the other credential, want %v", got, want)
			}
			wantGrants := c.refusedFor + c.goodFor
			if got := len(readGrants(t, conn, account)); got != wantGrants {
				t.Errorf("the calls took %d leases, want %d, one per call made", got, wantGrants)
			}
		})
	}
}

// VERIFICATIONS' rows for every series a job kind produces carrying its labels and for delta sync's
// series served between its ticks, for their part in the provider library. A caller registers the
// rate limiter's series and the shared provider series on the registerer it is given, so a job kind
// that builds its caller on a registerer carrying its labels has every series of its calls carry
// them. Here the registerer adds a label of its own, and the calls of a mailbox are counted on the
// caller's lease series and on the provider request cost its adapter reports (ADR-0076, ADR-0117).
func TestACallersSeriesAreOnTheRegistererItWasGiven(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	p := newProvider(t, account, nil)
	a := p.adapters()
	reg := prometheus.NewRegistry()
	labelled := prometheus.WrapRegistererWith(prometheus.Labels{"job_kind": "deltasync"}, reg)
	m := open(t, callerOn(t, a, Sync, time.Minute, labelled), Connection{Account: account, Token: token(t, a, "good")})
	if _, err := m.GetMessageMetadata(t.Context(), []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for _, f := range families {
		for _, series := range f.GetMetric() {
			labels := map[string]string{}
			for _, l := range series.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			if labels["job_kind"] != "deltasync" {
				t.Errorf("the series %s carries the labels %v, without the caller's job kind", f.GetName(), labels)
			}
			if labels["account"] == account {
				got[f.GetName()] += series.GetCounter().GetValue()
			}
		}
	}
	// The fake prices a call at one unit per message it names.
	want := map[string]float64{
		"mediated_mailbox_provider_hard_cap":           0,
		"mediated_mailbox_provider_request_cost_total": 2,
		"mediated_mailbox_ratelimit_granted_total":     2,
		"mediated_mailbox_ratelimit_rate":              0,
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the account's counters on the caller's registerer (-want +got):\n%s", diff)
	}
}

// VERIFICATIONS' row for a body fetched by part, for the provider library's wrapper. A body whose
// rendered part the provider holds apart is told so by the first call, and a second call naming the
// part returns it, each call leased on its own, so only such a body pays for two calls (ADR-0131).
func TestABodyHeldApartTakesTwoLeasedCallsThroughTheMailbox(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	p := newProvider(t, account, nil,
		fake.Message{Metadata: coremail.MessageMetadata{ID: "inline", ThreadID: "t1"}, Body: coremail.MessageBody{HTML: "<p>inline</p>"}},
		fake.Message{Metadata: coremail.MessageMetadata{ID: "apart", ThreadID: "t2"}, Body: coremail.MessageBody{HTML: "<p>apart</p>"}, HeldApart: true})
	a := p.adapters()
	m := open(t, caller(t, a, Interactive, time.Minute), Connection{Account: account, Token: token(t, a, "good")})
	for _, c := range []struct {
		id, html string
		calls    int
	}{{"inline", "<p>inline</p>", 1}, {"apart", "<p>apart</p>", 2}} {
		before := p.called("good")
		var part coremail.BodyPart
		for {
			body, err := m.GetMessageBody(t.Context(), c.id, part)
			next, due := coremail.NextBodyPart(part, err)
			if !due {
				if err != nil || body.HTML != c.html {
					t.Errorf("the body of %s is %+v, %v, want its HTML part", c.id, body, err)
				}
				break
			}
			part = next
		}
		if n := p.called("good") - before; n != c.calls {
			t.Errorf("the body of %s took %d calls, want %d", c.id, n, c.calls)
		}
	}
	if got := len(readGrants(t, conn, account)); got != 3 {
		t.Errorf("the bodies took %d leases, want 3, one per call", got)
	}
}

// priced is the fake behind a rate profile that prices every operation apart from every other, at
// its operation's number plus a quarter of a unit for each message it names, so a lease's tokens
// tell which operation it was asked for and how many messages it was priced for.
type priced struct{ *fake.Throttled }

func (priced) RateProfile() coremail.RateLimitProfile[context.Context] { return pricedProfile{} }

// pricedProfile is priced's rate profile.
type pricedProfile struct{}

func (pricedProfile) Cost(op coremail.ProviderOp) coremail.OpCost {
	return coremail.OpCost{Weight: float64(op.Operation) + 0.25*float64(max(op.Messages, 0)), OpsCount: op.Messages}
}
func (pricedProfile) BudgetPerSecond() float64 { return 100 }
func (pricedProfile) ParseThrottle(err error) (coremail.ThrottleSignal, bool) {
	return coremail.Throttled(err)
}
func (pricedProfile) RefreshLimits(context.Context) error { return nil }

// operation is one call of an operation of the port, made through a mailbox.
type operation struct {
	name string
	op   coremail.ProviderOp
	call func(context.Context, mail.Mailbox) error
}

// everyOperation returns one call of each of the port's ten operations, each over what the fake
// holds, with the operation and the messages it names as its price reads them. ChangesSince asks
// from cursor.
func everyOperation(t *testing.T, cursor coremail.Cursor) []operation {
	t.Helper()
	star, err := coremail.StarOp("a")
	if err != nil {
		t.Fatal(err)
	}
	unstar, err := coremail.MarkReadOp("b")
	if err != nil {
		t.Fatal(err)
	}
	return []operation{
		{"ListThreads", coremail.ProviderOp{Operation: coremail.OpListThreads}, func(ctx context.Context, m mail.Mailbox) error {
			_, err := m.ListThreads(ctx, coremail.All(), "")
			return err
		}},
		{"GetThreadMetadata", coremail.ProviderOp{Operation: coremail.OpGetThreadMetadata}, func(ctx context.Context, m mail.Mailbox) error {
			_, err := m.GetThreadMetadata(ctx, "t1")
			return err
		}},
		{"GetMessageMetadata", coremail.ProviderOp{Operation: coremail.OpGetMessageMetadata, Messages: 2}, func(ctx context.Context, m mail.Mailbox) error {
			_, err := m.GetMessageMetadata(ctx, []string{"a", "b"})
			return err
		}},
		{"GetMessageBody", coremail.ProviderOp{Operation: coremail.OpGetMessageBody, Messages: 1}, func(ctx context.Context, m mail.Mailbox) error {
			_, err := m.GetMessageBody(ctx, "a", "")
			return err
		}},
		{"ListLabels", coremail.ProviderOp{Operation: coremail.OpListLabels}, func(ctx context.Context, m mail.Mailbox) error {
			_, err := m.ListLabels(ctx)
			return err
		}},
		{"EnsureLabel", coremail.ProviderOp{Operation: coremail.OpEnsureLabel}, func(ctx context.Context, m mail.Mailbox) error {
			_, err := m.EnsureLabel(ctx, "Projects")
			return err
		}},
		{"Mutate", coremail.ProviderOp{Operation: coremail.OpMutate, Messages: 2}, func(ctx context.Context, m mail.Mailbox) error {
			_, err := m.Mutate(ctx, []coremail.MutationOp{star, unstar})
			return err
		}},
		{"CurrentCursor", coremail.ProviderOp{Operation: coremail.OpCurrentCursor}, func(ctx context.Context, m mail.Mailbox) error {
			_, err := m.CurrentCursor(ctx)
			return err
		}},
		{"ChangesSince", coremail.ProviderOp{Operation: coremail.OpChangesSince}, func(ctx context.Context, m mail.Mailbox) error {
			_, err := m.ChangesSince(ctx, cursor)
			return err
		}},
		{"EnumerateAll", coremail.ProviderOp{Operation: coremail.OpEnumerateAll}, func(ctx context.Context, m mail.Mailbox) error {
			_, err := m.EnumerateAll(ctx, "")
			return err
		}},
	}
}

// VERIFICATIONS' rows for a call through the wrapper taking a lease at its cost, telling the limiter
// how it went and trying once more on a refused credential, for each of the port's ten operations.
// Each operation's call takes one lease of exactly what the rate profile prices that operation at,
// with the messages it names, and none other. Each call the provider refuses reads the account again
// and is made once more over the token made from it, under a lease of its own. Each call the provider
// fails cuts the account's rate.
func TestEveryOperationIsLeasedAtItsCostReportedAndTriedOnceMore(t *testing.T) {
	conn := superuser(t)
	cursor, err := newProvider(t, "cursor", nil).fake.CurrentCursor(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range everyOperation(t, cursor) {
		t.Run(o.name, func(t *testing.T) {
			want := pricedProfile{}.Cost(o.op).Weight
			wantLeases := func(n int) []float64 {
				var out []float64
				for range n {
					out = append(out, want)
				}
				return out
			}
			var fail error
			account := newAccount(t, conn)
			p := newProvider(t, account, func(fake.Call) error { return fail },
				fake.Message{Metadata: coremail.MessageMetadata{ID: "a", ThreadID: "t1"}, Body: coremail.MessageBody{HTML: "<p>a</p>"}},
				fake.Message{Metadata: coremail.MessageMetadata{ID: "b", ThreadID: "t1"}, Body: coremail.MessageBody{HTML: "<p>b</p>"}})
			base := p.adapters()
			a := Adapters{byName: map[string]adapter{gmailName: {
				token: base.byName[gmailName].token,
				mailbox: func(account string, source tokenSource, requests Requests) (adapterMailbox, error) {
					inner, err := base.byName[gmailName].mailbox(account, source, requests)
					if err != nil {
						return nil, err
					}
					throttled, ok := inner.(*fake.Throttled)
					if !ok {
						return nil, errors.New("the test's adapter is not the throttled fake")
					}
					return priced{throttled}, nil
				},
			}}}
			again := func(context.Context) (*Token, error) { return token(t, a, "good"), nil }
			c := caller(t, a, Batch, time.Minute)

			if err := o.call(t.Context(), open(t, c, Connection{Account: account, Token: token(t, a, "good")})); err != nil {
				t.Fatalf("the call answered %v", err)
			}
			if diff := cmp.Diff(wantLeases(1), grantedTokens(t, conn, account), compare.Options); diff != "" {
				t.Errorf("the leases a call took (-want +got):\n%s", diff)
			}

			refusedFor := p.called(refused)
			if err := o.call(t.Context(), open(t, c, Connection{Account: account, Token: token(t, a, refused), Again: again})); err != nil {
				t.Errorf("a call over a refused credential answered %v, want it made once more over the credential read again", err)
			}
			if n := p.called(refused) - refusedFor; n != 1 {
				t.Errorf("the refused credential was sent %d times, want once", n)
			}
			if diff := cmp.Diff(wantLeases(3), grantedTokens(t, conn, account), compare.Options); diff != "" {
				t.Errorf("the leases after a call made once more (-want +got):\n%s", diff)
			}

			before := readRate(t, conn, account).Rate
			fail = coremail.ErrProvider
			if err := o.call(t.Context(), open(t, c, Connection{Account: account, Token: token(t, a, "good")})); !errors.Is(err, coremail.ErrProvider) {
				t.Fatalf("the failing call answered %v", err)
			}
			if after := readRate(t, conn, account).Rate; after >= before {
				t.Errorf("a provider failure left the rate at %v from %v, want it cut", after, before)
			}
		})
	}
}

// grantedTokens returns the tokens of each lease the account was granted, in the order granted.
func grantedTokens(t *testing.T, conn *pgx.Conn, account string) []float64 {
	t.Helper()
	var out []float64
	for _, g := range readGrants(t, conn, account) {
		out = append(out, g.Tokens)
	}
	return out
}
