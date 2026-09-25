package gmail

import (
	"context"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// Gmail's published figures (ADR-0023). The per-minute limit per user per project is 6,000 units, so
// the budget is 100 units a second on average
// (https://developers.google.com/workspace/gmail/api/reference/quota).
const budgetPerSecond = 100

// The quota units each Gmail method the adapter calls is charged, from the same page.
const (
	unitsGetProfile    = 1
	unitsHistoryList   = 2
	unitsLabelsList    = 1
	unitsLabelsCreate  = 5
	unitsMessagesList  = 5
	unitsMessagesGet   = 20
	unitsMessageModify = 5
	unitsThreadsList   = 10
	unitsThreadsGet    = 40
)

// The largest page each listing returns, sized so that its worst case costs no more than one
// second's worth at the hard cap, 80 units at Gmail's budget (ADR-0023, ADR-0024). A page of threads
// is two reads of the label table, one threads.list and a threads.get per thread, 52 units for one
// thread. A page of EnumerateAll is one messages.list, a messages.get per message and one read of the
// label table, 66 units for three messages.
const (
	threadsPerPage  = 1
	messagesPerPage = 3
)

// The most a caller may ask of one call whose size it sets, the largest that fits one second's worth
// at the hard cap. A metadata fetch of three identifiers costs 61 units, and a mutation of three ops
// costs 61. The adapter refuses a larger call, so the caller splits its work.
const (
	idsPerMetadataCall = 3
	opsPerMutation     = 3
)

// Profile is Gmail's rate profile. Every call's cost is declared before the call as its worst case in
// Gmail's quota units, the sum of what the Gmail methods it calls are charged, counting every read of
// the label table the adapter makes to map label identifiers to paths.
//
// Where a call's size depends on what it returns, the adapter sizes its page so the worst case fits
// one second's worth at the hard cap. Where the caller sets the size, as with the identifiers of a
// metadata fetch or the ops of a mutation, the cost grows with it, and a call past one second's worth
// is refused at lease issuance, so the caller splits its work into calls that fit. At today's prices
// a metadata fetch holds three identifiers and a mutation three ops. An HTTP batch is one call, holding
// only as many sub-requests as that second pays for.
//
//   - ListThreads costs a read of the label table for its query, a threads.list, a threads.get for
//     each thread of its page, and a read of the label table for the messages.
//   - GetThreadMetadata costs a threads.get and a read of the label table.
//   - GetMessageMetadata costs a messages.get per identifier and a read of the label table. With no
//     identifier it calls nothing, and it still declares the read of the label table, because the
//     rate limiter refuses a call costing nothing (ADR-0024) and the port gives every operation a
//     positive cost, so a caller never has to leave that call out.
//   - GetMessageBody costs a messages.get.
//   - ListLabels costs a read of the label table. EnsureLabel costs a read of the label table, a
//     labels.create, and a second read when Gmail answers that the label already exists.
//   - Mutate costs a read of the label table and, per op, the dearest way an op is applied, a read
//     of the message for an op left with nothing to change.
//   - CurrentCursor costs a getProfile, and ChangesSince a history.list and a read of the label
//     table.
//   - EnumerateAll costs a messages.list, a messages.get for each message of its page and a read
//     of the label table.
//
// A call naming no operation costs nothing and is refused at lease issuance.
type Profile struct{}

var _ mail.RateLimitProfile[context.Context] = Profile{}

// Cost implements the rate profile.
func (Profile) Cost(op mail.ProviderOp) mail.OpCost {
	n := max(op.Messages, 0)
	var units int
	switch op.Operation {
	case mail.OpListThreads:
		units = unitsLabelsList + unitsThreadsList + unitsThreadsGet*threadsPerPage + unitsLabelsList
	case mail.OpGetThreadMetadata:
		units = unitsThreadsGet + unitsLabelsList
	case mail.OpGetMessageMetadata:
		units = unitsMessagesGet*n + unitsLabelsList
	case mail.OpGetMessageBody:
		units = unitsMessagesGet
	case mail.OpListLabels:
		units = unitsLabelsList
	case mail.OpEnsureLabel:
		units = unitsLabelsList + unitsLabelsCreate + unitsLabelsList
	case mail.OpMutate:
		units = unitsLabelsList + max(unitsMessageModify, unitsMessagesGet)*n
	case mail.OpCurrentCursor:
		units = unitsGetProfile
	case mail.OpChangesSince:
		units = unitsHistoryList + unitsLabelsList
	case mail.OpEnumerateAll:
		units = unitsMessagesList + unitsMessagesGet*messagesPerPage + unitsLabelsList
	default:
		return mail.OpCost{}
	}
	return mail.OpCost{Weight: float64(units), OpsCount: n}
}

// BudgetPerSecond implements the rate profile. It is Gmail's per-minute limit per user per project
// averaged over its minute.
func (Profile) BudgetPerSecond() float64 { return budgetPerSecond }

// ParseThrottle implements the rate profile. The adapter returns a throttle as the port's
// ThrottleError, whose scope it takes from the reason Gmail gives, so a throttle's signal is read from
// that error.
func (Profile) ParseThrottle(err error) (mail.ThrottleSignal, bool) { return mail.Throttled(err) }

// RefreshLimits implements the rate profile. Gmail's limits are published constants, so there is
// nothing to re-read.
func (Profile) RefreshLimits(context.Context) error { return nil }
