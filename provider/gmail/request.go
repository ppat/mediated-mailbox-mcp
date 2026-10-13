package gmail

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/ppat/mediated-mailbox-mcp/provider/gmail/core"
)

// apiBase is the root every Gmail request's path is relative to, the account the access token
// belongs to.
const apiBase = "https://gmail.googleapis.com/gmail/v1/users/me/"

// request is one call to Gmail's API. Path is relative to apiBase, and Body is a JSON document, empty
// for a request without one. Units is what Gmail charges for the method, from the rate profile's
// prices, which the shell counts when it sends the request and holds each port call to. Endpoint is
// the Gmail method the request calls, without its users. prefix, which labels the request's latency,
// since the path holds identifiers and cannot (ADR-0125).
type request struct {
	Method   string
	Path     string
	Query    url.Values
	Body     string
	Units    int
	Endpoint string
	// Cap is the most of the answer the request reads. Every builder names one, and a request with
	// none is refused before it is sent (adapter.go).
	Cap readCap
}

// readCap is how much of an answer a request reads and what an answer past it is. Bytes of zero is
// no cap at all, which only a request no builder made carries.
type readCap struct {
	Bytes int
	// Refuses is true for a body read, whose answer past its cap is the body decision's PastCap, the
	// port's refusal (ADR-0131), and false for every other request, whose answer past maxResponse is
	// the provider's failure.
	Refuses bool
}

// bodyReadCap is the cap the body decision gives a read of a body, at n bytes.
func bodyReadCap(n int) readCap { return readCap{Bytes: n, Refuses: core.PastCap.Refused()} }

// anyReadCap is every other request's cap, maxResponse.
func anyReadCap() readCap { return readCap{Bytes: maxResponse} }

// URL returns the address the request is sent to.
func (r request) URL() string {
	if len(r.Query) == 0 {
		return apiBase + r.Path
	}
	return apiBase + r.Path + "?" + r.Query.Encode()
}

// validID reports whether id can name a Gmail message or thread. Gmail's identifiers are
// hexadecimal, and the check admits the URL-safe characters around them, so an identifier can
// never change the path it is written into.
func validID(id string) bool {
	if id == "" {
		return false
	}
	for _, c := range id {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

func labelsRequest() request {
	return request{Method: "GET", Path: "labels", Units: unitsLabelsList, Endpoint: "labels.list", Cap: anyReadCap()}
}

func createLabelRequest(path string) (request, error) {
	body, err := encode(struct {
		Name string `json:"name"`
	}{path})
	if err != nil {
		return request{}, err
	}
	return request{Method: "POST", Path: "labels", Body: body, Units: unitsLabelsCreate, Endpoint: "labels.create", Cap: anyReadCap()}, nil
}

// threadsRequest lists the threads a compiled query selects, trash and spam included.
func threadsRequest(s search, pageToken string, size int) request {
	q := url.Values{"includeSpamTrash": {"true"}, "maxResults": {strconv.Itoa(size)}}
	if s.Text != "" {
		q.Set("q", s.Text)
	}
	if len(s.LabelIDs) > 0 {
		q["labelIds"] = append([]string(nil), s.LabelIDs...)
	}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	return request{Method: "GET", Path: "threads", Query: q, Units: unitsThreadsList, Endpoint: "threads.list", Cap: anyReadCap()}
}

// messagesRequest lists every message in the mailbox, trash and spam included.
func messagesRequest(pageToken string, size int) request {
	q := url.Values{"includeSpamTrash": {"true"}, "maxResults": {strconv.Itoa(size)}}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	return request{Method: "GET", Path: "messages", Query: q, Units: unitsMessagesList, Endpoint: "messages.list", Cap: anyReadCap()}
}

// The formats a message is read in. Full holds the body, and minimal holds the identifiers and the
// labels.
const (
	formatFull    = "full"
	formatMinimal = "minimal"
)

// partsDepth is how many levels of MIME parts a metadata read names. An attachment nested deeper is
// not seen (ADR-0010).
const partsDepth = 6

// metadataFields is the field mask a metadata read sends with the full format (ADR-0010). Google
// returns only the fields a mask names, so the mask naming no body field is what keeps body content
// off the metadata path, which only the body fetch may carry. It names the identifiers, the labels,
// the dates, the history identifier, the snippet and size the model maps, the message's headers,
// and each part's type and file name to partsDepth levels. Gmail's metadata format keeps the body
// out as well but returns no parts, so no attachment names.
func metadataFields() string {
	parts := ""
	for range partsDepth {
		inner := "mimeType,filename"
		if parts != "" {
			inner += "," + parts
		}
		parts = "parts(" + inner + ")"
	}
	return "id,threadId,labelIds,snippet,historyId,internalDate,sizeEstimate,payload(mimeType,filename,headers," + parts + ")"
}

// metadataRequest reads the message id names through the metadata mask. The caller has checked id
// with validID.
func metadataRequest(id string) request {
	q := url.Values{"format": {formatFull}, "fields": {metadataFields()}}
	return request{Method: "GET", Path: "messages/" + url.PathEscape(id), Query: q, Units: unitsMessagesGet, Endpoint: "messages.get", Cap: anyReadCap()}
}

// messageRequest reads the message id names in a format. The caller has checked id with validID.
func messageRequest(id, format string) request {
	q := url.Values{"format": {format}}
	return request{Method: "GET", Path: "messages/" + url.PathEscape(id), Query: q, Units: unitsMessagesGet, Endpoint: "messages.get", Cap: anyReadCap()}
}

// threadRequest reads the thread id names, every message of it through the metadata mask. The
// caller has checked id with validID.
func threadRequest(id string) request {
	q := url.Values{"format": {formatFull}, "fields": {"id,historyId,messages(" + metadataFields() + ")"}}
	return request{Method: "GET", Path: "threads/" + url.PathEscape(id), Query: q, Units: unitsThreadsGet, Endpoint: "threads.get", Cap: anyReadCap()}
}

// bodyFields is the field mask a body read sends with the full format. It names the identifiers and
// the whole part tree, which the rendered part is chosen from, and drops the labels, the snippet, the
// history identifier, the date and the size estimate, which the metadata read supplies. It keeps the
// headers, since a part held apart comes in the charset its Content-Type declares, which for a
// single-part message is among the message's own headers (ADR-0131).
const bodyFields = "id,threadId,payload"

// bodyRequest reads the message id names for its rendered part. The caller has checked id with
// validID.
func bodyRequest(id string) request {
	q := url.Values{"format": {formatFull}, "fields": {bodyFields}}
	return request{
		Method: "GET", Path: "messages/" + url.PathEscape(id), Query: q, Units: unitsMessagesGet, Endpoint: "messages.get",
		Cap: bodyReadCap(core.MessageReadCap),
	}
}

// partRequest reads the part held apart from message id that Gmail's reference attachment names.
// The caller has checked both with validID.
func partRequest(id, attachment string) request {
	return request{
		Method: "GET", Path: "messages/" + url.PathEscape(id) + "/attachments/" + url.PathEscape(attachment),
		Units: unitsAttachmentsGet, Endpoint: "messages.attachments.get", Cap: bodyReadCap(core.PartReadCap),
	}
}

// modifyRequest adds and removes label identifiers on a message, or on every message of a thread.
// The caller has checked id with validID.
func modifyRequest(id string, add, remove []string) (request, error) {
	body, err := encode(struct {
		Add    []string `json:"addLabelIds,omitempty"`
		Remove []string `json:"removeLabelIds,omitempty"`
	}{add, remove})
	if err != nil {
		return request{}, err
	}
	return request{Method: "POST", Path: "messages/" + url.PathEscape(id) + "/modify", Body: body, Units: unitsMessageModify, Endpoint: "messages.modify", Cap: anyReadCap()}, nil
}

func profileRequest() request {
	return request{Method: "GET", Path: "profile", Units: unitsGetProfile, Endpoint: "getProfile", Cap: anyReadCap()}
}

// historyRequest lists the mailbox's history records after start.
func historyRequest(start uint64, size int) request {
	q := url.Values{"startHistoryId": {strconv.FormatUint(start, 10)}, "maxResults": {strconv.Itoa(size)}}
	return request{Method: "GET", Path: "history", Query: q, Units: unitsHistoryList, Endpoint: "history.list", Cap: anyReadCap()}
}

func encode(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("gmail: encoding a request body: %w", err)
	}
	return string(b), nil
}
