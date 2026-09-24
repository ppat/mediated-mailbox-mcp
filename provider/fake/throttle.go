package fake

import (
	"context"
	"sync"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// Call is one call a Throttled port received, as its schedule sees it.
type Call struct {
	// Seq numbers the calls from one, in the order the port received them.
	Seq int
	// Op is the operation called and the number of messages it names.
	Op mail.ProviderOp
	// Cost is what the wrapped port's rate profile says the call costs.
	Cost mail.OpCost
	// At is the time the port's clock read when the call arrived.
	At time.Time
}

// Schedule decides whether a call is refused. It returns nil to let the call through, or the error
// the port returns instead, a mail.ThrottleError for a throttle. A refused call has no effect. The
// Throttled port consults its schedule for one call at a time, in the order the calls arrive, so a
// schedule that keeps state needs no lock of its own.
type Schedule func(Call) error

// Never lets every call through.
func Never() Schedule { return func(Call) error { return nil } }

// ThrottleCalls throttles the calls numbered first to last, both included, with signal, and lets
// every other call through. A test proves recovery by throttling a run of calls and watching what
// follows it.
func ThrottleCalls(first, last int, signal mail.ThrottleSignal) Schedule {
	return func(c Call) error {
		if c.Seq >= first && c.Seq <= last {
			return mail.ThrottleError{Signal: signal}
		}
		return nil
	}
}

// Ceiling throttles, with signal, a call that would take the weight of the calls let through in the
// second before it, this call's included, above weightPerSecond. A refused call does not count
// against the ceiling. It stands for a provider whose real ceiling is unknown to the caller, so a
// test can prove a rate controller converges below it. A call weighing more than the ceiling is
// always refused.
func Ceiling(weightPerSecond float64, signal mail.ThrottleSignal) Schedule {
	type spent struct {
		at     time.Time
		weight float64
	}
	var window []spent
	return func(c Call) error {
		kept := window[:0]
		total := 0.0
		for _, s := range window {
			if c.At.Sub(s.at) < time.Second {
				kept = append(kept, s)
				total += s.weight
			}
		}
		window = kept
		if total+c.Cost.Weight > weightPerSecond {
			return mail.ThrottleError{Signal: signal}
		}
		window = append(window, spent{at: c.At, weight: c.Cost.Weight})
		return nil
	}
}

// Throttled is a port whose calls pass a schedule before they reach the port it wraps.
//
// The schedule decides in the order calls arrive. A call it lets through reaches the wrapped port
// after that decision and outside its lock, so calls made at the same time may take effect in an
// order other than the one they were decided in.
type Throttled struct {
	inner    mail.Port[context.Context]
	schedule Schedule
	clock    func() time.Time
	mu       sync.Mutex
	seq      int
}

var _ mail.Port[context.Context] = (*Throttled)(nil)

// Throttle wraps inner so every call first passes schedule. The clock stamps each call, and a test
// passes its own clock, so the schedule's decisions depend only on the calls and the times the test
// chose.
func Throttle(inner mail.Port[context.Context], schedule Schedule, clock func() time.Time) *Throttled {
	return &Throttled{inner: inner, schedule: schedule, clock: clock}
}

func (t *Throttled) admit(ctx context.Context, op mail.Operation, messages int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p := mail.ProviderOp{Operation: op, Messages: messages}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.seq++
	return t.schedule(Call{Seq: t.seq, Op: p, Cost: t.inner.RateProfile().Cost(p), At: t.clock()})
}

// ListThreads implements the port.
func (t *Throttled) ListThreads(ctx context.Context, q mail.Query, page mail.PageToken) (mail.Page[mail.ThreadMetadata], error) {
	if err := t.admit(ctx, mail.OpListThreads, 0); err != nil {
		return mail.Page[mail.ThreadMetadata]{}, err
	}
	return t.inner.ListThreads(ctx, q, page)
}

// GetThreadMetadata implements the port.
func (t *Throttled) GetThreadMetadata(ctx context.Context, threadID string) (mail.ThreadMetadata, error) {
	if err := t.admit(ctx, mail.OpGetThreadMetadata, 0); err != nil {
		return mail.ThreadMetadata{}, err
	}
	return t.inner.GetThreadMetadata(ctx, threadID)
}

// GetMessageMetadata implements the port.
func (t *Throttled) GetMessageMetadata(ctx context.Context, ids []string) ([]mail.MessageMetadata, error) {
	if err := t.admit(ctx, mail.OpGetMessageMetadata, len(ids)); err != nil {
		return nil, err
	}
	return t.inner.GetMessageMetadata(ctx, ids)
}

// GetMessageBody implements the port.
func (t *Throttled) GetMessageBody(ctx context.Context, id string) (mail.MessageBody, error) {
	if err := t.admit(ctx, mail.OpGetMessageBody, 1); err != nil {
		return mail.MessageBody{}, err
	}
	return t.inner.GetMessageBody(ctx, id)
}

// ListLabels implements the port.
func (t *Throttled) ListLabels(ctx context.Context) ([]mail.Label, error) {
	if err := t.admit(ctx, mail.OpListLabels, 0); err != nil {
		return nil, err
	}
	return t.inner.ListLabels(ctx)
}

// EnsureLabel implements the port.
func (t *Throttled) EnsureLabel(ctx context.Context, path string) (mail.Label, error) {
	if err := t.admit(ctx, mail.OpEnsureLabel, 0); err != nil {
		return mail.Label{}, err
	}
	return t.inner.EnsureLabel(ctx, path)
}

// Mutate implements the port.
func (t *Throttled) Mutate(ctx context.Context, ops []mail.MutationOp) (mail.MutationResult, error) {
	if err := t.admit(ctx, mail.OpMutate, len(ops)); err != nil {
		return mail.MutationResult{}, err
	}
	return t.inner.Mutate(ctx, ops)
}

// CurrentCursor implements the port.
func (t *Throttled) CurrentCursor(ctx context.Context) (mail.Cursor, error) {
	if err := t.admit(ctx, mail.OpCurrentCursor, 0); err != nil {
		return "", err
	}
	return t.inner.CurrentCursor(ctx)
}

// ChangesSince implements the port.
func (t *Throttled) ChangesSince(ctx context.Context, c mail.Cursor) (mail.ChangeSet, error) {
	if err := t.admit(ctx, mail.OpChangesSince, 0); err != nil {
		return mail.ChangeSet{}, err
	}
	return t.inner.ChangesSince(ctx, c)
}

// EnumerateAll implements the port.
func (t *Throttled) EnumerateAll(ctx context.Context, page mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
	if err := t.admit(ctx, mail.OpEnumerateAll, 0); err != nil {
		return mail.Page[mail.MessageMetadata]{}, err
	}
	return t.inner.EnumerateAll(ctx, page)
}

// RateProfile implements the port. It is the wrapped port's profile, so the costs a caller reads are
// the costs the schedule weighs.
func (t *Throttled) RateProfile() mail.RateLimitProfile[context.Context] {
	return t.inner.RateProfile()
}
