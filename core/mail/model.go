// Package mail holds the canonical mail model, the Provider Port interface the adapters implement,
// the canonical query, and the rate profile types adapters declare with the hard-cap fraction every
// call is sized within (ADR-0010, ADR-0023, ADR-0024).
//
// Everything above the port speaks these types, and only an adapter knows a provider's own shapes.
// The package is a pure core, so it holds types, constructors and the interfaces and nothing that
// performs input or output. The pure-core import list (ADR-0071) admits neither the context package
// nor the time package, because time reads the environment and the zone files, and that shapes two
// things.
//
//   - The port is generic in the value each call carries for cancellation, because the context
//     package is not on the list. Adapters and their callers use Port[context.Context].
//   - An instant is a UnixMilli, milliseconds since the Unix epoch, because the time package is not
//     on the list either. An integer count from the epoch has no offset, so every instant the model
//     holds is UTC by construction (ADR-0033).
//
// Metadata and the body never share a type. No type a metadata operation returns has a field for the
// full body, and the body arrives only through GetMessageBody, the one operation the gate guards
// (ADR-0010). The metadata does carry two fields derived from the body, the snippet and the
// attachment names, which the gate withholds with it (ADR-0001).
package mail

// UnixMilli is an instant as milliseconds since the Unix epoch, in UTC.
type UnixMilli int64

// The labels the disposal verbs manage. A message in the inbox carries Inbox, and one in the trash
// or marked as spam carries Trash or Spam. Adapters translate their provider's own names and roles
// to these.
const (
	Inbox = "INBOX"
	Trash = "TRASH"
	Spam  = "SPAM"
)

// Address is an email address with its display name, which is empty when the message gives none.
type Address struct {
	Email string
	Name  string
}

// Flags are a message's states that are not labels.
type Flags struct {
	Read    bool
	Starred bool
}

// AuthResults are the sender authentication results a message carries, each an RFC 8601 result
// keyword such as pass or fail, and empty when the message carries none. Classification does not
// read them (ADR-0004).
type AuthResults struct {
	SPF   string
	DKIM  string
	DMARC string
}

// MessageMetadata is everything about a message except its body.
type MessageMetadata struct {
	AccountID string
	ID        string
	ThreadID  string
	From      Address
	To        []Address
	Cc        []Address
	Subject   string
	Date      UnixMilli
	// Labels are label paths, the labels the disposal verbs manage included.
	Labels         []string
	Flags          Flags
	SizeBytes      int64
	HasAttachments bool
	// AttachmentNames are derived from the body, so the gate withholds them with it (ADR-0001).
	AttachmentNames []string
	// Snippet is the provider's preview of the body, empty when the provider gives none. It is body
	// text, so the gate withholds it with the body (ADR-0001).
	Snippet string
	// ListID is the RFC 2919 list identifier without its angle brackets, empty when there is none.
	ListID      string
	AuthResults AuthResults
}

// ThreadMetadata is a thread and the metadata of every message in it, oldest first.
type ThreadMetadata struct {
	AccountID string
	ID        string
	Messages  []MessageMetadata
}

// MessageBody is a message's body as the provider holds it, before any sanitization. A part the
// message does not have is empty, so a body with no HTML part is never converted as HTML (ADR-0036).
type MessageBody struct {
	AccountID string
	MessageID string
	Text      string
	HTML      string
}

// Label is a label an account has. Its path joins the levels of a nested label with a slash.
type Label struct {
	AccountID string
	Path      string
}

// PageToken marks where a paged listing continues. The empty token asks for the first page, and a
// page whose Next is empty is the last.
type PageToken string

// Page is one page of a paged listing.
type Page[T any] struct {
	Items []T
	Next  PageToken
}

// Cursor marks a point in a mailbox's change history. It is opaque, and only the implementation that
// issued it can read it.
type Cursor string

// ChangeSet is what changed in a mailbox after a cursor, as message identifiers. Each identifier sits
// in one list at most. A message added and then removed is only removed, and one added and then
// changed is only added. Next is the cursor to ask from next time.
type ChangeSet struct {
	AccountID string
	Added     []string
	Modified  []string
	Removed   []string
	Next      Cursor
}
