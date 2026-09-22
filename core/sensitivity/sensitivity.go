// Package sensitivity holds the sensitivity-carrying types every component shares, sender class,
// content flags and scan state, the Sensitivity that combines them, and the Body a released message
// body travels in.
//
// Each type keeps its fields unexported, is built only through its constructor, and has the most
// restrictive state as its zero value (ADR-0042). A zero SenderClass is restricted, zero
// ContentFlags carry both flags, a zero ScanState is pending, and a zero Body holds no text. So a
// value nobody built denies a body rather than releasing one. Fixtures that must not compile against
// these types sit under testdata/mustnotcompile, one directory per case.
//
// A Body can be built only from a Sensitivity that releases a body, which is a normal sender, no
// content flag, and a scan state of scanned or skipped by the scan gate (ADR-0007). A body value
// carrying restricted sensitivity or a denying scan state cannot be constructed at all.
package sensitivity

import "errors"

// SenderClass is the per-sender axis. The zero value is restricted.
type SenderClass struct {
	normal bool
}

// NormalSender returns the normal sender class.
func NormalSender() SenderClass { return SenderClass{normal: true} }

// RestrictedSender returns the restricted sender class.
func RestrictedSender() SenderClass { return SenderClass{} }

// Restricted reports whether the class is restricted.
func (c SenderClass) Restricted() bool { return !c.normal }

// ContentFlags is the per-message axis, a detected one-time code and a detected login link. The zero
// value carries both flags.
type ContentFlags struct {
	noMFACode   bool
	noLoginLink bool
}

// Flags returns the content flags a scan detected.
func Flags(mfaCode, loginLink bool) ContentFlags {
	return ContentFlags{noMFACode: !mfaCode, noLoginLink: !loginLink}
}

// NoFlags returns content flags with neither flag set.
func NoFlags() ContentFlags { return Flags(false, false) }

// MFACode reports whether a one-time code was detected.
func (f ContentFlags) MFACode() bool { return !f.noMFACode }

// LoginLink reports whether a login link was detected.
func (f ContentFlags) LoginLink() bool { return !f.noLoginLink }

// Any reports whether either flag is set.
func (f ContentFlags) Any() bool { return f.MFACode() || f.LoginLink() }

// ScanState is a message's position relative to the content scanner (ADR-0007). The zero value is
// pending.
type ScanState struct {
	state scanState
}

type scanState uint8

const (
	pending scanState = iota
	scanned
	skippedGate
	skippedRestricted
)

// Pending returns the state of a message the scanner has not reached.
func Pending() ScanState { return ScanState{state: pending} }

// Scanned returns the state of a message the scanner has read.
func Scanned() ScanState { return ScanState{state: scanned} }

// SkippedGate returns the state of a message the scan gate chose not to scan.
func SkippedGate() ScanState { return ScanState{state: skippedGate} }

// SkippedRestricted returns the state of a message left unscanned because its sender is restricted.
func SkippedRestricted() ScanState { return ScanState{state: skippedRestricted} }

// String returns the state's name as the schema spells it (ADR-0016).
func (s ScanState) String() string {
	switch s.state {
	case scanned:
		return "scanned"
	case skippedGate:
		return "skipped_gate"
	case skippedRestricted:
		return "skipped_restricted"
	case pending:
		return "pending"
	default:
		return "pending"
	}
}

// Sensitivity combines a message's sender class, content flags and scan state. The zero value is a
// restricted sender with both flags, pending scan.
type Sensitivity struct {
	class SenderClass
	flags ContentFlags
	scan  ScanState
}

// ErrFlagsWithoutScan is returned for content flags on a message the scanner has not read. Flags come
// only from a scan.
var ErrFlagsWithoutScan = errors.New("sensitivity: content flags on a message that was not scanned")

// ErrNormalSkippedRestricted is returned for a normal sender left at skipped_restricted. The scan
// gate skips only restricted senders' mail that way, and a delisted sender's mail returns to pending
// (ADR-0037).
var ErrNormalSkippedRestricted = errors.New("sensitivity: a normal sender's message cannot be skipped as restricted")

// New returns the Sensitivity of a message, refusing a combination no message can be in. A caller
// treats a refusal as withholding the body, since a stored state it cannot build is an error state.
func New(class SenderClass, flags ContentFlags, scan ScanState) (Sensitivity, error) {
	if flags.Any() && scan.state != scanned {
		return Sensitivity{}, ErrFlagsWithoutScan
	}
	if !class.Restricted() && scan.state == skippedRestricted {
		return Sensitivity{}, ErrNormalSkippedRestricted
	}
	return Sensitivity{class: class, flags: flags, scan: scan}, nil
}

// Class returns the sender class.
func (s Sensitivity) Class() SenderClass { return s.class }

// Flags returns the content flags.
func (s Sensitivity) Flags() ContentFlags { return s.flags }

// Scan returns the scan state.
func (s Sensitivity) Scan() ScanState { return s.scan }

// ReleasesBody reports whether a message of this sensitivity may have its body released. It requires
// a normal sender, no content flag, and a scan state of scanned or skipped by the scan gate.
func (s Sensitivity) ReleasesBody() bool {
	if s.class.Restricted() || s.flags.Any() {
		return false
	}
	switch s.scan.state {
	case scanned, skippedGate:
		return true
	case pending, skippedRestricted:
		return false
	default:
		return false
	}
}

// Body is a message body released to a client. The zero value holds no text.
type Body struct {
	text string
}

// ErrBodyWithheld is returned when a body is built for a sensitivity that does not release one.
var ErrBodyWithheld = errors.New("sensitivity: the message's sensitivity withholds its body")

// NewBody returns a Body holding text, refusing unless s releases a body.
func NewBody(text string, s Sensitivity) (Body, error) {
	if !s.ReleasesBody() {
		return Body{}, ErrBodyWithheld
	}
	return Body{text: text}, nil
}

// Text returns the body's text.
func (b Body) Text() string { return b.text }
