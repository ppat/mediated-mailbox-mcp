// Package sensitivity holds the sensitivity-carrying types every component shares, sender class,
// content flags and scan state, and the Sensitivity that combines them. It holds the names the index
// stores for the content flags and the scan state, which the index's writers write and its readers
// read through it, and the sender class's name, which the client surface and the UI show, since the
// index stores no class (ADR-0016, ADR-0136).
//
// Each type keeps its fields unexported, is built only through its constructor, and has the most
// restrictive state as its zero value (ADR-0042). A zero SenderClass is restricted, zero
// ContentFlags carry both flags, and a zero ScanState is pending. So a value nobody built denies a
// body rather than releasing one, and so does a stored name the schema does not name. Fixtures that
// must not compile against these types sit under testdata/mustnotcompile, one directory per case.
//
// A Sensitivity releases a body only for a normal sender, no content flag, and a scan state of
// scanned or skipped by the scan gate (ADR-0093). The body a client receives is the API server's
// release step's, which builds one only for a Sensitivity that releases it.
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

// String returns the class's name as the client surface and the UI spell it, normal or restricted.
func (c SenderClass) String() string {
	if c.normal {
		return "normal"
	}
	return "restricted"
}

// SenderClassNamed returns the class name spells and true, or the restricted class and false for a
// name that is neither normal nor restricted.
func SenderClassNamed(name string) (SenderClass, bool) {
	switch name {
	case "normal":
		return NormalSender(), true
	case "restricted":
		return RestrictedSender(), true
	default:
		return RestrictedSender(), false
	}
}

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

// Names returns the names the index stores for the flags set, mfa_code before login_link, and an
// empty list for none (ADR-0016).
func (f ContentFlags) Names() []string {
	names := []string{}
	if f.MFACode() {
		names = append(names, "mfa_code")
	}
	if f.LoginLink() {
		names = append(names, "login_link")
	}
	return names
}

// FlagsNamed returns the content flags the stored names carry. A name the schema does not name reads
// as both flags, the most restrictive state, so an unreadable row is never released (ADR-0042).
func FlagsNamed(names []string) ContentFlags {
	mfa, login := false, false
	for _, n := range names {
		switch n {
		case "mfa_code":
			mfa = true
		case "login_link":
			login = true
		default:
			return Flags(true, true)
		}
	}
	return Flags(mfa, login)
}

// ScanState is a message's position relative to the content scanner (ADR-0093). The zero value is
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
	refused
)

// Pending returns the state of a message the scanner has not reached.
func Pending() ScanState { return ScanState{state: pending} }

// Scanned returns the state of a message the scanner has read.
func Scanned() ScanState { return ScanState{state: scanned} }

// SkippedGate returns the state of a message the scan gate chose not to scan.
func SkippedGate() ScanState { return ScanState{state: skippedGate} }

// SkippedRestricted returns the state of a message left unscanned because its sender is restricted.
func SkippedRestricted() ScanState { return ScanState{state: skippedRestricted} }

// Refused returns the state of a message whose body was refused for its size or by its conversion, so
// it is never scanned or released while the bound it was refused under holds (ADR-0132).
func Refused() ScanState { return ScanState{state: refused} }

// String returns the state's name as the schema spells it (ADR-0016).
func (s ScanState) String() string {
	switch s.state {
	case scanned:
		return "scanned"
	case skippedGate:
		return "skipped_gate"
	case skippedRestricted:
		return "skipped_restricted"
	case refused:
		return "refused"
	case pending:
		return "pending"
	default:
		return "pending"
	}
}

// ScanStateNamed returns the scan state the stored name spells, the inverse of String. A name the
// schema does not name reads as pending, which the gate never releases (ADR-0042).
func ScanStateNamed(name string) ScanState {
	switch name {
	case "scanned":
		return Scanned()
	case "skipped_gate":
		return SkippedGate()
	case "skipped_restricted":
		return SkippedRestricted()
	case "refused":
		return Refused()
	default:
		return Pending()
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
// at the next delisting comparison (ADR-0037), so until then this refusal withholds the body.
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
	case pending, skippedRestricted, refused:
		return false
	default:
		return false
	}
}
