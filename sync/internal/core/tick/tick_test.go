package tick_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/sync/internal/core/tick"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

const hour = mail.UnixMilli(3_600_000)

// A gap's window starts an hour before the last cursor's write time and ends now, however long ago
// the cursor was written, and an account with no cursor reconciles the first window before now
// (ADR-0105).
func TestWindow(t *testing.T) {
	type window struct{ Start, End mail.UnixMilli }
	written := mail.UnixMilli(1_000 * hour)
	longAgo := mail.UnixMilli(10 * hour)
	cases := []struct {
		name     string
		cursorAt *mail.UnixMilli
		want     window
	}{
		{"a cursor written five minutes ago", &written, window{999 * hour, 1_000*hour + 5*60_000}},
		{"a cursor written long ago", &longAgo, window{9 * hour, 1_000*hour + 5*60_000}},
		{"no cursor", nil, window{1_000*hour + 5*60_000 - 168*hour, 1_000*hour + 5*60_000}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			start, end := tick.Window(c.cursorAt, 1_000*hour+5*60_000, 168*hour)
			if diff := cmp.Diff(c.want, window{start, end}, compare.Options); diff != "" {
				t.Errorf("Window (-want +got):\n%s", diff)
			}
		})
	}
}

// A change set's metadata is asked for the messages it reports added or changed, each once, in its
// order, and never for one it reports removed.
func TestFetched(t *testing.T) {
	cs := mail.ChangeSet{Added: []string{"a", "b"}, Modified: []string{"c", "a"}, Removed: []string{"d"}}
	if diff := cmp.Diff([]string{"a", "b", "c"}, tick.Fetched(cs), compare.Options); diff != "" {
		t.Errorf("Fetched (-want +got):\n%s", diff)
	}
	if got := tick.Fetched(mail.ChangeSet{Removed: []string{"d"}}); got != nil {
		t.Errorf("Fetched of removals alone = %v, want none", got)
	}
}

// A throttle or a refused credential stops the account's scanning, a provider failure, a body the
// provider no longer has and a refused request leave the message waiting, and a failure that is not
// the provider's fails the tick, as does a decision nobody made (ADR-0104).
func TestOnBodyFailure(t *testing.T) {
	cases := []struct {
		class       index.ErrorClass
		want        tick.Next
		disposition string
	}{
		{index.Throttled, tick.StopScanning, "abandoned"},
		{index.Authentication, tick.StopScanning, "abandoned"},
		{index.ProviderError, tick.LeaveWaiting, "abandoned"},
		{index.Gone, tick.LeaveWaiting, "gone"},
		{index.Validation, tick.LeaveWaiting, "abandoned"},
		{index.NotProvider, tick.FailTick, "abandoned"},
	}
	for _, c := range cases {
		if got := tick.OnBodyFailure(c.class); got != c.want {
			t.Errorf("OnBodyFailure(%q) = %d, want %d", c.class, got, c.want)
		}
		if got := tick.Disposition(c.class); got != c.disposition {
			t.Errorf("Disposition(%q) = %q, want %q", c.class, got, c.disposition)
		}
	}
	if tick.Next(0) != tick.FailTick {
		t.Errorf("the zero Next is %d, want FailTick", tick.Next(0))
	}
}

// A gap recovery's candidates for removal are the stored messages its listing left out, and of those
// it removes only the ones the provider no longer returns by identifier, so a listing that misses a
// message the mailbox holds removes nothing (ADR-0105).
func TestRemoval(t *testing.T) {
	unlisted := tick.Unlisted([]string{"m1", "m2", "m3", "m4"}, map[string]bool{"m2": true, "m4": true, "m9": true})
	if diff := cmp.Diff([]string{"m1", "m3"}, unlisted, compare.Options); diff != "" {
		t.Errorf("Unlisted (-want +got):\n%s", diff)
	}
	if got := tick.Unlisted([]string{"m1"}, map[string]bool{"m1": true}); got != nil {
		t.Errorf("Unlisted of a message the listing returned is %v, want none", got)
	}
	gone := tick.Gone([]string{"m1", "m3"}, []mail.MessageMetadata{{ID: "m3"}})
	if diff := cmp.Diff([]string{"m1"}, gone, compare.Options); diff != "" {
		t.Errorf("Gone (-want +got):\n%s", diff)
	}
	if got := tick.Gone([]string{"m1"}, []mail.MessageMetadata{{ID: "m1"}}); got != nil {
		t.Errorf("Gone of a message the provider returned is %v, want none", got)
	}
}
