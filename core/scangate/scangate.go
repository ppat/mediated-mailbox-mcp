// Package scangate holds the Scan Gate's predicate, which decides whether a message body is worth
// scanning (ADR-0007). It decides from pass-1 statistics and message metadata the caller passes in as
// values, under thresholds the caller also passes in, and returns its decision as a value
// (ADR-0040). Fetching the body and running the scanner's tiers follow a decision to scan and are
// the shell's.
//
// The rules are ADR-0007's, applied in the order it lists them, and the first that matches gives
// the decision and its reason. Every decision carries its reason, scans as well as skips. A
// restricted sender is the first rule, so no other input can turn a restricted sender's message into
// a scan (ADR-0008). The one other skip is the high-volume rule, and a message no rule names is
// scanned.
//
// Fail closed. A zero Input is a restricted sender and is skipped as restricted. A Config with a
// threshold below 1, or a high-volume mark below the low-volume one, decides nothing, and neither
// does a Verdict nobody built. A message an undecided verdict reaches stays pending, so its body
// stays denied.
package scangate

import (
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
)

// Config holds the thresholds the rules compare against. They are tuned from the recorded skip
// rates, so the caller passes them in (ADR-0007).
type Config struct {
	// NoReplyLocalParts are the local parts that ask for a scan on a message with no List-Id. They
	// match whole and in any letter case.
	NoReplyLocalParts []string
	// SmallBytes and RecentAgeMillis bound the recent small message rule. A message below SmallBytes
	// in size and younger than RecentAgeMillis milliseconds, with no List-Id, is scanned.
	SmallBytes      int64
	RecentAgeMillis int64
	// LowVolume is the sender volume below which a sender's messages are all scanned.
	LowVolume int64
	// HighVolume is the sender volume above which the high-volume rule may skip a message.
	HighVolume int64
}

// DefaultConfig returns ADR-0007's thresholds.
func DefaultConfig() Config {
	return Config{
		NoReplyLocalParts: []string{"noreply", "no-reply", "security", "accounts", "verify", "auth", "support"},
		SmallBytes:        30 * 1024,
		RecentAgeMillis:   24 * 60 * 60 * 1000,
		LowVolume:         20,
		HighVolume:        500,
	}
}

// valid reports whether c can decide. A zero Config cannot, so thresholds nobody set skip nothing.
func (c Config) valid() bool {
	return c.SmallBytes >= 1 && c.RecentAgeMillis >= 1 && c.LowVolume >= 1 && c.HighVolume >= c.LowVolume
}

// Input is what the predicate decides from, for one message.
type Input struct {
	// Class is the sender's class under the policy in force.
	Class sensitivity.SenderClass
	// SubjectMasked reports whether pass 1 masked the message's subject for any reason, the whole
	// subject masked when the scanner could not decide included (ADR-0003, ADR-0007).
	SubjectMasked bool
	// ListID reports whether the message carries its own List-Id header.
	ListID bool
	// LocalPart is the local part of the sender's address.
	LocalPart string
	// SizeBytes is the message's size.
	SizeBytes int64
	// SentAt is when the message was sent.
	SentAt mail.UnixMilli
	// SenderVolume is the sender's message count from pass 1's sender aggregates.
	SenderVolume int64
	// PriorHits is the number of the sender's messages in which the scanner found a content flag.
	PriorHits int64
}

// Reason is the rule that decided a message. The zero value is no decision.
type Reason uint8

const (
	// Undecided is a verdict nobody built or one made under a Config that cannot decide. The
	// message stays pending.
	Undecided Reason = iota
	// Restricted is a restricted sender, skipped and never scanned (ADR-0008).
	Restricted
	// SubjectSignal is a subject pass 1 masked, scanned.
	SubjectSignal
	// NoReplyLocalPart is a message with no List-Id from one of the configured local parts, scanned.
	NoReplyLocalPart
	// RecentSmall is a small, recent message with no List-Id, scanned.
	RecentSmall
	// LowVolume is a message from a sender below the low-volume mark, scanned.
	LowVolume
	// PriorHit is a message from a sender with a prior scan hit, scanned.
	PriorHit
	// HighVolumeNoHits is a message carrying a List-Id from a sender above the high-volume mark with
	// no prior scan hit, skipped.
	HighVolumeNoHits
	// Default is a message no other rule names, scanned.
	Default
)

// String returns the reason as ADR-0007 names it, which is how the decision table stores it.
func (r Reason) String() string {
	switch r {
	case Restricted:
		return "restricted"
	case SubjectSignal:
		return "subject_signal"
	case NoReplyLocalPart:
		return "noreply_local_part"
	case RecentSmall:
		return "recent_small"
	case LowVolume:
		return "low_volume"
	case PriorHit:
		return "prior_hit"
	case HighVolumeNoHits:
		return "high_volume_no_hits"
	case Default:
		return "default"
	case Undecided:
		return "undecided"
	default:
		return "undecided"
	}
}

// Verdict is the gate's decision on one message. The zero value decides nothing.
type Verdict struct {
	reason Reason
}

// Reason returns the rule that decided the message.
func (v Verdict) Reason() Reason { return v.reason }

// Scans reports whether the shell fetches and scans the body.
func (v Verdict) Scans() bool {
	switch v.reason {
	case SubjectSignal, NoReplyLocalPart, RecentSmall, LowVolume, PriorHit, Default:
		return true
	case Undecided, Restricted, HighVolumeNoHits:
		return false
	default:
		return false
	}
}

// Decided reports whether the verdict is a decision the shell records.
func (v Verdict) Decided() bool { return v.reason != Undecided }

// State returns the scan state the shell records for the message. A skip records its skip state. A
// message to be scanned stays pending until its scan verdict is stored, and so does a message an
// undecided verdict reaches.
func (v Verdict) State() sensitivity.ScanState {
	switch v.reason {
	case Restricted:
		return sensitivity.SkippedRestricted()
	case HighVolumeNoHits:
		return sensitivity.SkippedGate()
	case Undecided, SubjectSignal, NoReplyLocalPart, RecentSmall, LowVolume, PriorHit, Default:
		return sensitivity.Pending()
	default:
		return sensitivity.Pending()
	}
}

// Decide returns the gate's decision on the message in, under the thresholds c, with the message's
// age measured at now.
func Decide(c Config, now mail.UnixMilli, in Input) Verdict {
	switch {
	case in.Class.Restricted():
		return Verdict{reason: Restricted}
	case !c.valid():
		return Verdict{}
	case in.SubjectMasked:
		return Verdict{reason: SubjectSignal}
	case !in.ListID && noReply(c, in.LocalPart):
		return Verdict{reason: NoReplyLocalPart}
	case !in.ListID && in.SizeBytes < c.SmallBytes && int64(now-in.SentAt) < c.RecentAgeMillis:
		return Verdict{reason: RecentSmall}
	case in.SenderVolume < c.LowVolume:
		return Verdict{reason: LowVolume}
	case in.PriorHits > 0:
		return Verdict{reason: PriorHit}
	case in.SenderVolume > c.HighVolume && in.PriorHits == 0 && in.ListID:
		return Verdict{reason: HighVolumeNoHits}
	default:
		return Verdict{reason: Default}
	}
}

func noReply(c Config, localPart string) bool {
	for _, p := range c.NoReplyLocalParts {
		if strings.EqualFold(p, localPart) {
			return true
		}
	}
	return false
}
