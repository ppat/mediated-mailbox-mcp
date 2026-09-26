package scangate_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// outcome is what a test can observe of a verdict.
type outcome struct {
	Scans   bool
	Decided bool
	Reason  string
	State   string
}

func observe(v scangate.Verdict) outcome {
	return outcome{Scans: v.Scans(), Decided: v.Decided(), Reason: v.Reason().String(), State: v.State().String()}
}

func scanned(reason string) outcome {
	return outcome{Scans: true, Decided: true, Reason: reason, State: "pending"}
}

var (
	restricted = outcome{Decided: true, Reason: "restricted", State: "skipped_restricted"}
	highVolume = outcome{Decided: true, Reason: "high_volume_no_hits", State: "skipped_gate"}
	undecided  = outcome{Reason: "undecided", State: "pending"}
)

// now is 2026-09-25T12:00:00Z.
var now mail.UnixMilli = 1_790_337_600_000

const (
	minute = 60 * 1000
	hour   = 60 * minute
)

// skippable is a message the high-volume rule skips under the default thresholds. Each case below
// changes it.
func skippable() scangate.Input {
	return scangate.Input{
		Class:         sensitivity.NormalSender(),
		SubjectMasked: false,
		ListID:        true,
		LocalPart:     "news",
		SizeBytes:     100_000,
		SentAt:        now - 48*hour,
		SenderVolume:  501,
		PriorHits:     0,
	}
}

func with(change func(*scangate.Input)) scangate.Input {
	in := skippable()
	change(&in)
	return in
}

func TestDecide(t *testing.T) {
	cases := []struct {
		name string
		in   scangate.Input
		want outcome
	}{
		{"a high-volume sender with no prior hit, a List-Id and no subject signal", skippable(), highVolume},
		{"a sender far above the high-volume mark", with(func(in *scangate.Input) { in.SenderVolume = 1_000_000 }), highVolume},
		{"a sender at the high-volume mark", with(func(in *scangate.Input) { in.SenderVolume = 500 }), scanned("default")},
		{"a message with no List-Id no other rule names", with(func(in *scangate.Input) { in.ListID = false }), scanned("default")},
		{"a subject pass 1 masked", with(func(in *scangate.Input) { in.SubjectMasked = true }), scanned("subject_signal")},
		{"a noreply local part with no List-Id", with(func(in *scangate.Input) { in.ListID, in.LocalPart = false, "noreply" }), scanned("noreply_local_part")},
		{"a noreply local part in another letter case", with(func(in *scangate.Input) { in.ListID, in.LocalPart = false, "No-Reply" }), scanned("noreply_local_part")},
		{"a local part holding a listed one inside it", with(func(in *scangate.Input) { in.ListID, in.LocalPart = false, "noreply-billing" }), scanned("default")},
		{"a noreply local part with a List-Id", with(func(in *scangate.Input) { in.LocalPart = "noreply" }), highVolume},
		{"a small recent message with no List-Id", with(func(in *scangate.Input) {
			in.ListID, in.SizeBytes, in.SentAt = false, 30_719, now-hour
		}), scanned("recent_small")},
		{"a recent message at the size mark", with(func(in *scangate.Input) {
			in.ListID, in.SizeBytes, in.SentAt = false, 30_720, now-hour
		}), scanned("default")},
		{"a small message at the age mark", with(func(in *scangate.Input) {
			in.ListID, in.SizeBytes, in.SentAt = false, 1_000, now-24*hour
		}), scanned("default")},
		{"a small recent message with a List-Id", with(func(in *scangate.Input) {
			in.SizeBytes, in.SentAt = 1_000, now-hour
		}), highVolume},
		{"a sender below the low-volume mark", with(func(in *scangate.Input) { in.SenderVolume = 19 }), scanned("low_volume")},
		{"a sender at the low-volume mark", with(func(in *scangate.Input) { in.SenderVolume = 20 }), scanned("default")},
		{"a sender with one prior hit", with(func(in *scangate.Input) { in.PriorHits = 1 }), scanned("prior_hit")},
		{"a sender with a negative hit count", with(func(in *scangate.Input) { in.PriorHits = -1 }), scanned("default")},
		{"a subject signal wins over every later rule", with(func(in *scangate.Input) {
			in.SubjectMasked, in.ListID, in.LocalPart, in.SizeBytes, in.SentAt = true, false, "noreply", 1_000, now
			in.SenderVolume, in.PriorHits = 3, 2
		}), scanned("subject_signal")},
		{"a noreply local part wins over the recent small rule", with(func(in *scangate.Input) {
			in.ListID, in.LocalPart, in.SizeBytes, in.SentAt, in.SenderVolume, in.PriorHits = false, "noreply", 1_000, now, 3, 2
		}), scanned("noreply_local_part")},
		{"a recent small message wins over low volume", with(func(in *scangate.Input) {
			in.ListID, in.SizeBytes, in.SentAt, in.SenderVolume, in.PriorHits = false, 1_000, now, 3, 2
		}), scanned("recent_small")},
		{"low volume wins over a prior hit", with(func(in *scangate.Input) { in.SenderVolume, in.PriorHits = 3, 2 }), scanned("low_volume")},
		{"a restricted sender otherwise skipped by the high-volume rule", with(func(in *scangate.Input) { in.Class = sensitivity.RestrictedSender() }), restricted},
		{"a restricted sender whose every other input asks for a scan", with(func(in *scangate.Input) {
			in.Class, in.SubjectMasked, in.ListID, in.LocalPart = sensitivity.RestrictedSender(), true, false, "noreply"
			in.SizeBytes, in.SentAt, in.SenderVolume, in.PriorHits = 1_000, now, 3, 2
		}), restricted},
		{"an input nobody filled in", scangate.Input{}, restricted},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := observe(scangate.Decide(scangate.DefaultConfig(), now, c.in))
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("Decide(%+v) (-want +got):\n%s", c.in, diff)
			}
		})
	}
}

// The defaults are ADR-0007's figures, with 30KB as 30,720 bytes and the age mark as 24 hours.
func TestDefaultConfigIsADR0007s(t *testing.T) {
	want := scangate.Config{
		NoReplyLocalParts: []string{"noreply", "no-reply", "security", "accounts", "verify", "auth", "support"},
		SmallBytes:        30_720,
		RecentAgeMillis:   86_400_000,
		LowVolume:         20,
		HighVolume:        500,
	}
	if diff := cmp.Diff(want, scangate.DefaultConfig(), compare.Options); diff != "" {
		t.Errorf("DefaultConfig() (-want +got):\n%s", diff)
	}
}

// The thresholds are read from the Config the caller passes, not fixed in the package.
func TestDecideReadsTheConfig(t *testing.T) {
	custom := scangate.Config{
		NoReplyLocalParts: []string{"alerts"},
		SmallBytes:        100,
		RecentAgeMillis:   hour,
		LowVolume:         50,
		HighVolume:        100,
	}
	cases := []struct {
		name string
		in   scangate.Input
		want outcome
	}{
		{"a sender above a lowered high-volume mark", with(func(in *scangate.Input) { in.SenderVolume = 101 }), highVolume},
		{"a sender at a lowered high-volume mark", with(func(in *scangate.Input) { in.SenderVolume = 100 }), scanned("default")},
		{"a sender below a raised low-volume mark", with(func(in *scangate.Input) { in.SenderVolume = 49 }), scanned("low_volume")},
		{"a configured local part", with(func(in *scangate.Input) { in.ListID, in.LocalPart = false, "alerts" }), scanned("noreply_local_part")},
		{"a default local part left out of the configuration", with(func(in *scangate.Input) { in.ListID, in.LocalPart = false, "noreply" }), scanned("default")},
		{"a message below a lowered size mark", with(func(in *scangate.Input) {
			in.ListID, in.SizeBytes, in.SentAt = false, 99, now-minute
		}), scanned("recent_small")},
		{"a message at a lowered size mark", with(func(in *scangate.Input) {
			in.ListID, in.SizeBytes, in.SentAt = false, 100, now-minute
		}), scanned("default")},
		{"a message at a shortened age mark", with(func(in *scangate.Input) {
			in.ListID, in.SizeBytes, in.SentAt = false, 99, now-hour
		}), scanned("default")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := observe(scangate.Decide(custom, now, c.in))
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("Decide(%+v) (-want +got):\n%s", c.in, diff)
			}
		})
	}
}

// A Config that cannot decide decides nothing, so thresholds nobody set skip nothing. A restricted
// sender is still skipped as restricted.
func TestAConfigThatCannotDecide(t *testing.T) {
	valid := scangate.DefaultConfig()
	change := func(f func(*scangate.Config)) scangate.Config {
		c := scangate.DefaultConfig()
		f(&c)
		return c
	}
	cases := []struct {
		name string
		c    scangate.Config
		in   scangate.Input
		want outcome
	}{
		{"a zero Config", scangate.Config{}, skippable(), undecided},
		{"a high-volume mark below the low-volume mark", change(func(c *scangate.Config) { c.HighVolume = 19 }), skippable(), undecided},
		{"a low-volume mark of zero", change(func(c *scangate.Config) { c.LowVolume = 0 }), skippable(), undecided},
		{"a size mark of zero", change(func(c *scangate.Config) { c.SmallBytes = 0 }), skippable(), undecided},
		{"an age mark of zero", change(func(c *scangate.Config) { c.RecentAgeMillis = 0 }), skippable(), undecided},
		{"a restricted sender under a zero Config", scangate.Config{}, scangate.Input{}, restricted},
		{"the default Config", valid, skippable(), highVolume},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := observe(scangate.Decide(c.c, now, c.in))
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("Decide(%+v) (-want +got):\n%s", c.c, diff)
			}
		})
	}
}

// A verdict nobody built neither scans nor skips, so the message stays pending and its body denied
// (ADR-0042).
func TestTheZeroVerdictDecidesNothing(t *testing.T) {
	if diff := cmp.Diff(undecided, observe(scangate.Verdict{}), compare.Options); diff != "" {
		t.Errorf("zero Verdict (-want +got):\n%s", diff)
	}
}

// The decision takes the thresholds, the instant and the message's metadata and statistics, and
// nothing a provider or a body could arrive through, so a restricted sender's body is never fetched
// to decide it (ADR-0008). Each struct among them is checked down to its last field.
func TestTheDecisionTakesNoProviderAndNoBody(t *testing.T) {
	const pkg = "github.com/ppat/mediated-mailbox-mcp/core/scangate"
	mustnotcompile.RequireParams(t, pkg, "Decide", "Config", "mail.UnixMilli", "Input")
	mustnotcompile.RequireFields(t, pkg, "Config",
		"NoReplyLocalParts []string", "SmallBytes int64", "RecentAgeMillis int64", "LowVolume int64", "HighVolume int64")
	mustnotcompile.RequireFields(t, pkg, "Input",
		"Class sensitivity.SenderClass", "SubjectMasked bool", "ListID bool", "LocalPart string",
		"SizeBytes int64", "SentAt mail.UnixMilli", "SenderVolume int64", "PriorHits int64")
	mustnotcompile.RequireFields(t, pkg, "Verdict", "reason Reason")
	mustnotcompile.RequireFields(t, "github.com/ppat/mediated-mailbox-mcp/core/sensitivity", "SenderClass", "normal bool")
}
