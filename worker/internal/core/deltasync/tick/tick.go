// Package tick holds the decisions of one delta sync tick that are delta sync's own, as values
// (ADR-0040). What a message adds to the index, the delisting comparison, the gate and the scan are
// core/index's, which backfill shares. The shell asks for the changes, applies them and scans what
// waits, and enacts what this package decides.
//
// Window decides what a gap's recovery, or an account's first reconciliation, re-enumerates
// (ADR-0105). Unlisted and Gone decide which stored messages a gap's recovery removes (ADR-0105).
// Fetched decides which messages a change set's metadata is asked for, and core/mail's CallSize how
// many identifiers one metadata call names. OnBodyFailure decides what a tick does after a body fetch
// fails (ADR-0104).
package tick

import (
	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// Overlap is how far before the last cursor's write time a gap's window starts, so a provider's
// dates drifting from delta sync's clock leave no message out (ADR-0105).
const Overlap = mail.UnixMilli(60 * 60 * 1000)

// Window returns the start and end of the window a re-enumeration lists, ending at now. With a cursor
// write time it starts Overlap before it, with no cap. With none, it is the account's first
// reconciliation, and starts first before now (ADR-0105).
func Window(cursorAt *mail.UnixMilli, now, first mail.UnixMilli) (start, end mail.UnixMilli) {
	if cursorAt != nil {
		return *cursorAt - Overlap, now
	}
	return now - first, now
}

// Unlisted returns the stored messages a gap recovery's listing did not return, in the order of
// stored. Each is only a candidate for removal, since the listing selects through the provider's
// search, so the recovery asks the provider for each by its identifier first (ADR-0105).
func Unlisted(stored []string, listed map[string]bool) []string {
	var out []string
	for _, id := range stored {
		if !listed[id] {
			out = append(out, id)
		}
	}
	return out
}

// Gone returns the asked-for messages the provider did not return when asked for them by their
// identifiers, which the mailbox no longer holds, in the order asked. They are the messages a gap
// recovery removes (ADR-0105).
func Gone(asked []string, held []mail.MessageMetadata) []string {
	returned := map[string]bool{}
	for _, m := range held {
		returned[m.ID] = true
	}
	var out []string
	for _, id := range asked {
		if !returned[id] {
			out = append(out, id)
		}
	}
	return out
}

// Fetched returns the messages whose metadata a change set needs, those it reports added and those it
// reports changed, each once, in the order the change set gives them. A removed message needs none.
func Fetched(cs mail.ChangeSet) []string {
	seen := map[string]bool{}
	var out []string
	for _, id := range append(append([]string{}, cs.Added...), cs.Modified...) {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// Next is what a tick does after a body fetch fails.
type Next uint8

const (
	// FailTick fails the tick, since the failure is not the provider's. The zero value, so a decision
	// nobody made fails the tick rather than going on.
	FailTick Next = iota
	// LeaveWaiting records the message as a failed item and goes on. It stays waiting for a scan,
	// and the next pass over the backlog asks for it again.
	LeaveWaiting
	// StopScanning records the message as a failed item and ends the tick's scanning for the account,
	// since the provider is refusing the tick rather than the message. The next tick goes on.
	StopScanning
)

// OnBodyFailure decides what a tick does after a body fetch of class fails (ADR-0104). A throttle or
// a refused credential stops the account's scanning, a body the provider failed, no longer has or
// refused as malformed is left waiting, and a failure that is not the provider's fails the tick. A
// later tick asks again, so a tick never retries a body itself.
func OnBodyFailure(class index.ErrorClass) Next {
	switch class {
	case index.Throttled, index.Authentication:
		return StopScanning
	case index.ProviderError, index.Gone, index.Validation:
		return LeaveWaiting
	case index.NotProvider:
		return FailTick
	default:
		return FailTick
	}
}

// Disposition is what a failed item records became of a message whose body fetch failed, gone for one
// the provider no longer has and abandoned for any other (ADR-0016).
func Disposition(class index.ErrorClass) string {
	if class == index.Gone {
		return "gone"
	}
	return "abandoned"
}
