package mail

import "errors"

// The errors the port speaks. An operation's error wraps one of them, so a caller tells them apart
// with errors.Is.
var (
	// ErrNotFound is a message, thread or label the account does not have.
	ErrNotFound = errors.New("mail: no such message, thread or label")
	// ErrInvalid is a request the port refuses as malformed, such as an invalid query, an op no
	// constructor built, or a page token the implementation did not issue.
	ErrInvalid = errors.New("mail: the request is malformed")
	// ErrCursorGap is a cursor the provider cannot calculate changes from, because it expired or was
	// never valid. The caller re-enumerates a bounded window and alerts (ADR-0018).
	ErrCursorGap = errors.New("mail: the provider cannot calculate changes from the cursor")
	// ErrThrottled is a request the provider refused for exceeding its rate. An error wrapping it is
	// a ThrottleError carrying the provider's signal.
	ErrThrottled = errors.New("mail: the provider throttled the request")
	// ErrProvider is a request the provider failed on its side.
	ErrProvider = errors.New("mail: the provider failed the request")
	// ErrAuthentication is a credential the provider refused.
	ErrAuthentication = errors.New("mail: the provider refused the credential")
	// ErrRefused is a body the port will not return, its rendered part larger than MaxBodyBytes or in
	// a form the implementation cannot decode. The provider did nothing wrong, and asking again
	// refuses again while the message and MaxBodyBytes stay as they are (ADR-0131).
	ErrRefused = errors.New("mail: the body is refused, too large or not decodable")
	// ErrHeldApart is a rendered part the provider holds apart from the message, which a second
	// GetMessageBody call names. An error wrapping it is a HeldApartError naming the part.
	ErrHeldApart = errors.New("mail: the body's rendered part is held apart from the message")
)

// BodyPart names one rendered part of a message for GetMessageBody. It is opaque, and only the
// implementation that issued it can read it. The empty part asks the implementation to choose the
// rendered part.
type BodyPart string

// HeldApartError is GetMessageBody's answer when the rendered part is held apart from the message
// and within MaxBodyBytes. Part names it for the second call.
type HeldApartError struct {
	Part BodyPart
}

func (e HeldApartError) Error() string { return ErrHeldApart.Error() }

func (e HeldApartError) Unwrap() error { return ErrHeldApart }

// NextBodyPart decides, after a GetMessageBody call that named asked returned err, the part the next
// call names and whether one is due (ADR-0131). A call that chose the rendered part and was told it
// is held apart is followed by one naming that part. Nothing else is followed, a part that names
// itself held apart again included, so a body takes at most two calls.
func NextBodyPart(asked BodyPart, err error) (BodyPart, bool) {
	var apart HeldApartError
	if asked != "" || !errors.As(err, &apart) || apart.Part == "" {
		return "", false
	}
	return apart.Part, true
}

// MaxBodyBytes is the largest rendered part, in bytes of UTF-8 text, that the port returns and the
// system releases or scans (ADR-0131). The conversion's input limit and the literal form's are this
// value, so the three cannot drift. The live run measured Gmail holding parts apart once a message's
// parts pass between 1,000,000 and 1,048,619 bytes in all, the large parts first, so any raise of
// this value first runs the gmail-contract workflow on its branch, since only a real Gmail answer
// shows how many bodies would then take the second call (ADR-0131).
const MaxBodyBytes = 512 << 10
