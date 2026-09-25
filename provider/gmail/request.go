package gmail

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// apiBase is the root every Gmail request's path is relative to, the account the access token
// belongs to.
const apiBase = "https://gmail.googleapis.com/gmail/v1/users/me/"

// request is one call to Gmail's API. Path is relative to apiBase, and Body is a JSON document, empty
// for a request without one. Units is what Gmail charges for the method, from the rate profile's
// prices, which the shell counts when it sends the request and holds each port call to.
type request struct {
	Method string
	Path   string
	Query  url.Values
	Body   string
	Units  int
}

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

func labelsRequest() request { return request{Method: "GET", Path: "labels", Units: unitsLabelsList} }

func createLabelRequest(path string) (request, error) {
	body, err := encode(struct {
		Name string `json:"name"`
	}{path})
	if err != nil {
		return request{}, err
	}
	return request{Method: "POST", Path: "labels", Body: body, Units: unitsLabelsCreate}, nil
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
	return request{Method: "GET", Path: "threads", Query: q, Units: unitsThreadsList}
}

// messagesRequest lists every message in the mailbox, trash and spam included.
func messagesRequest(pageToken string, size int) request {
	q := url.Values{"includeSpamTrash": {"true"}, "maxResults": {strconv.Itoa(size)}}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	return request{Method: "GET", Path: "messages", Query: q, Units: unitsMessagesList}
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
	return request{Method: "GET", Path: "messages/" + url.PathEscape(id), Query: q, Units: unitsMessagesGet}
}

// messageRequest reads the message id names in a format. The caller has checked id with validID.
func messageRequest(id, format string) request {
	q := url.Values{"format": {format}}
	return request{Method: "GET", Path: "messages/" + url.PathEscape(id), Query: q, Units: unitsMessagesGet}
}

// threadRequest reads the thread id names, every message of it through the metadata mask. The
// caller has checked id with validID.
func threadRequest(id string) request {
	q := url.Values{"format": {formatFull}, "fields": {"id,historyId,messages(" + metadataFields() + ")"}}
	return request{Method: "GET", Path: "threads/" + url.PathEscape(id), Query: q, Units: unitsThreadsGet}
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
	return request{Method: "POST", Path: "messages/" + url.PathEscape(id) + "/modify", Body: body, Units: unitsMessageModify}, nil
}

func profileRequest() request { return request{Method: "GET", Path: "profile", Units: unitsGetProfile} }

// historyRequest lists the mailbox's history records after start.
func historyRequest(start uint64, size int) request {
	q := url.Values{"startHistoryId": {strconv.FormatUint(start, 10)}, "maxResults": {strconv.Itoa(size)}}
	return request{Method: "GET", Path: "history", Query: q, Units: unitsHistoryList}
}

func encode(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("gmail: encoding a request body: %w", err)
	}
	return string(b), nil
}
