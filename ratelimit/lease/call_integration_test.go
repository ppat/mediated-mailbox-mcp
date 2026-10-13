//go:build integration

package lease_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
)

// rate returns the account's current rate as the rate state stores it.
func rate(t *testing.T, conn *pgx.Conn, account string) float64 {
	t.Helper()
	var r float64
	if err := conn.QueryRow(t.Context(), "SELECT current_rate FROM rate_state WHERE account_id = $1", account).Scan(&r); err != nil {
		t.Fatal(err)
	}
	return r
}

// counted wraps f so every call it receives is counted in calls.
func counted(f *fake.Fake, calls *int) *fake.Throttled {
	return fake.Throttle(f, func(fake.Call) error { *calls++; return nil }, time.Now)
}

// VERIFICATIONS' rows for the body fetched by part and for a refusal reaching no controller. A body
// whose rendered part comes with the message takes one call, one whose part the provider holds apart
// within the bound takes a second call naming it, and one whose part is past the bound takes one
// call and is refused. The refusal tells the controller nothing, so the account's rate stays where it
// was, while a provider failure through the same path cuts it (ADR-0131, ADR-0024).
func TestABodyIsFetchedByPartAndARefusalTellsTheControllerNothing(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	l := lease.New(spenders(t), ceiling, nil)
	f, err := fake.New(fake.Config{Account: account},
		fake.Message{Metadata: mail.MessageMetadata{ID: "inline", ThreadID: "t1"}, Body: mail.MessageBody{HTML: "<p>inline</p>"}},
		fake.Message{Metadata: mail.MessageMetadata{ID: "apart", ThreadID: "t2"}, Body: mail.MessageBody{HTML: "<p>apart</p>"}, HeldApart: true},
		fake.Message{Metadata: mail.MessageMetadata{ID: "past", ThreadID: "t3"}, Body: mail.MessageBody{Text: strings.Repeat("a", mail.MaxBodyBytes+1)}},
	)
	if err != nil {
		t.Fatal(err)
	}
	var calls int
	port := counted(f, &calls)

	for _, c := range []struct {
		id, html string
		calls    int
	}{{"inline", "<p>inline</p>", 1}, {"apart", "<p>apart</p>", 2}} {
		before := calls
		body, err := lease.Body(t.Context(), l, port, account, core.Batch, c.id)
		if err != nil || body.HTML != c.html || body.MessageID != c.id {
			t.Errorf("the body of %s is %+v, %v, want its HTML part", c.id, body, err)
		}
		if n := calls - before; n != c.calls {
			t.Errorf("the body of %s took %d calls, want %d", c.id, n, c.calls)
		}
	}

	before, made := rate(t, conn, account), calls
	if _, err := lease.Body(t.Context(), l, port, account, core.Batch, "past"); !errors.Is(err, mail.ErrRefused) || errors.Is(err, mail.ErrProvider) {
		t.Errorf("the body past the bound failed with %v, want a refusal that is not the provider's failure", err)
	}
	if n := calls - made; n != 1 {
		t.Errorf("the body past the bound took %d calls, want 1", n)
	}
	if after := rate(t, conn, account); after != before {
		t.Errorf("the refusal moved the account's rate from %v to %v", before, after)
	}

	failed := fake.Throttle(f, func(fake.Call) error { return mail.ErrProvider }, time.Now)
	if _, err := lease.Body(t.Context(), l, failed, account, core.Batch, "inline"); !errors.Is(err, mail.ErrProvider) {
		t.Fatalf("the failing provider's body answered %v", err)
	}
	if after := rate(t, conn, account); after >= before {
		t.Errorf("a provider failure left the account's rate at %v from %v, so the rate state shows no cut", after, before)
	}
}

// VERIFICATIONS' row for the worker's provider-call deadline. A call that does not answer within
// CallTimeout of its lease while its caller waits is cut off and fails as the provider's failure,
// which cuts the account's rate, and a call its caller gives up on first is the caller's, which tells
// the controller nothing (ADR-0119, ADR-0024).
func TestACallThatDoesNotAnswerIsCutOffAsTheProvidersFailure(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	l := lease.New(spenders(t), ceiling, nil)
	f, err := fake.New(fake.Config{Account: account})
	if err != nil {
		t.Fatal(err)
	}
	op := mail.ProviderOp{Operation: mail.OpGetMessageBody, Messages: 1}
	hang := func(ctx context.Context) (mail.MessageBody, error) {
		<-ctx.Done()
		return mail.MessageBody{}, ctx.Err()
	}
	if _, err := lease.Call(t.Context(), l, f, account, core.Batch, op, func(context.Context) (mail.MessageBody, error) { return mail.MessageBody{}, nil }); err != nil {
		t.Fatal(err)
	}
	before := rate(t, conn, account)

	gaveUp, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if _, err := lease.Call(gaveUp, l, f, account, core.Batch, op, hang); errors.Is(err, mail.ErrProvider) || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a call its caller gave up on failed with %v, want the caller's deadline and not the provider's failure", err)
	}
	if after := rate(t, conn, account); after != before {
		t.Errorf("a call its caller gave up on moved the account's rate from %v to %v", before, after)
	}

	// The deadline is the 30 seconds ADR-0119 states, written out rather than read from the package.
	const deadline = 30 * time.Second
	start := time.Now()
	if _, err := lease.Call(t.Context(), l, f, account, core.Batch, op, hang); !errors.Is(err, mail.ErrProvider) {
		t.Errorf("a call that never answered failed with %v, want the provider's failure", err)
	}
	if waited := time.Since(start); waited < deadline || waited > deadline+10*time.Second {
		t.Errorf("the call was cut off after %v, want %v", waited, deadline)
	}
	if after := rate(t, conn, account); after >= before {
		t.Errorf("a call cut off left the account's rate at %v from %v, want it cut", after, before)
	}
}
