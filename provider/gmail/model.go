package gmail

import (
	"cmp"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"mime"
	netmail "net/mail"
	"slices"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// The system label identifiers that map to flags rather than to labels.
const (
	unreadID  = "UNREAD"
	starredID = "STARRED"
)

// shownSystemLabels are the system labels the canonical model shows, by identifier. Every other
// system label is left out.
var shownSystemLabels = []string{mail.Inbox, mail.Trash, mail.Spam, "SENT", "DRAFT"}

// gmailLabel is a label as labels.list returns it.
type gmailLabel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// labelTable maps between Gmail's label identifiers and the paths of the labels the canonical model
// shows. hidden holds the system labels it leaves out.
type labelTable struct {
	pathByID map[string]string
	idByPath map[string]string
	hidden   map[string]bool
}

func newLabelTable(labels []gmailLabel) labelTable {
	t := labelTable{pathByID: map[string]string{}, idByPath: map[string]string{}, hidden: map[string]bool{}}
	for _, l := range labels {
		path := ""
		switch {
		case l.Type == "user":
			path = l.Name
		case slices.Contains(shownSystemLabels, l.ID):
			path = l.ID
		case l.ID == unreadID, l.ID == starredID:
			continue
		default:
			t.hidden[l.ID] = true
			continue
		}
		if path == "" {
			continue
		}
		t.pathByID[l.ID] = path
		t.idByPath[path] = l.ID
	}
	return t
}

// parseLabels reads a labels.list response.
func parseLabels(body []byte) (labelTable, error) {
	var raw struct {
		Labels []gmailLabel `json:"labels"`
	}
	if err := unmarshal(body, &raw, "the label list"); err != nil {
		return labelTable{}, err
	}
	return newLabelTable(raw.Labels), nil
}

// parseLabel reads a labels.create response as the path of the label it created.
func parseLabel(body []byte) (string, error) {
	var l gmailLabel
	if err := unmarshal(body, &l, "the created label"); err != nil {
		return "", err
	}
	if l.ID == "" || l.Name == "" {
		return "", fmt.Errorf("gmail: the created label has no identifier or name: %w", mail.ErrProvider)
	}
	return l.Name, nil
}

// canonicalLabels returns the labels the model shows, sorted by path.
func (t labelTable) canonicalLabels(account string) []mail.Label {
	paths := make([]string, 0, len(t.idByPath))
	for p := range t.idByPath {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	out := make([]mail.Label, len(paths))
	for i, p := range paths {
		out[i] = mail.Label{AccountID: account, Path: p}
	}
	return out
}

// gmailMessage is a message as messages.get returns it.
type gmailMessage struct {
	ID           string    `json:"id"`
	ThreadID     string    `json:"threadId"`
	LabelIDs     []string  `json:"labelIds"`
	Snippet      string    `json:"snippet"`
	InternalDate int64     `json:"internalDate,string"`
	SizeEstimate int64     `json:"sizeEstimate"`
	Payload      gmailPart `json:"payload"`
}

type gmailPart struct {
	MimeType string        `json:"mimeType"`
	Filename string        `json:"filename"`
	Headers  []gmailHeader `json:"headers"`
	Body     gmailBody     `json:"body"`
	Parts    []gmailPart   `json:"parts"`
}

type gmailHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type gmailBody struct {
	AttachmentID string `json:"attachmentId"`
	Data         string `json:"data"`
}

// parseMessage reads a messages.get response.
func parseMessage(body []byte) (gmailMessage, error) {
	var m gmailMessage
	if err := unmarshal(body, &m, "a message"); err != nil {
		return gmailMessage{}, err
	}
	if m.ID == "" || m.ThreadID == "" {
		return gmailMessage{}, fmt.Errorf("gmail: a message has no identifier or thread: %w", mail.ErrProvider)
	}
	return m, nil
}

// parseThread reads a threads.get response as its messages.
func parseThread(body []byte) ([]gmailMessage, error) {
	var raw struct {
		ID       string         `json:"id"`
		Messages []gmailMessage `json:"messages"`
	}
	if err := unmarshal(body, &raw, "a thread"); err != nil {
		return nil, err
	}
	for _, m := range raw.Messages {
		if m.ID == "" || m.ThreadID == "" {
			return nil, fmt.Errorf("gmail: a message of thread %q has no identifier or thread: %w", raw.ID, mail.ErrProvider)
		}
	}
	return raw.Messages, nil
}

// metadata maps a message read through the metadata mask to the canonical metadata.
func metadata(account string, labels labelTable, m gmailMessage) mail.MessageMetadata {
	out := mail.MessageMetadata{
		AccountID:   account,
		ID:          m.ID,
		ThreadID:    m.ThreadID,
		From:        address(header(m.Payload.Headers, "From")),
		To:          addresses(header(m.Payload.Headers, "To")),
		Cc:          addresses(header(m.Payload.Headers, "Cc")),
		Subject:     decodeHeader(header(m.Payload.Headers, "Subject")),
		Date:        mail.UnixMilli(m.InternalDate),
		Flags:       mail.Flags{Read: true},
		SizeBytes:   m.SizeEstimate,
		Snippet:     html.UnescapeString(m.Snippet),
		ListID:      listID(header(m.Payload.Headers, "List-Id")),
		AuthResults: authResults(m.Payload.Headers),
	}
	for _, id := range m.LabelIDs {
		switch id {
		case unreadID:
			out.Flags.Read = false
		case starredID:
			out.Flags.Starred = true
		default:
			if path, ok := labels.pathByID[id]; ok {
				out.Labels = append(out.Labels, path)
			}
		}
	}
	out.AttachmentNames = attachmentNames(m.Payload)
	out.HasAttachments = len(out.AttachmentNames) > 0
	return out
}

// header returns the value of the first header named name, ignoring case.
func header(headers []gmailHeader, name string) string {
	for _, h := range headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

// address parses a single address. A value that does not parse keeps its decoded text as the name
// and has no address, so it can never match a sender query by accident.
func address(value string) mail.Address {
	if strings.TrimSpace(value) == "" {
		return mail.Address{}
	}
	a, err := netmail.ParseAddress(value)
	if err != nil {
		return mail.Address{Name: decodeHeader(value)}
	}
	return mail.Address{Email: a.Address, Name: a.Name}
}

// addresses parses an address list. A list that does not parse maps to no addresses.
func addresses(value string) []mail.Address {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	list, err := netmail.ParseAddressList(value)
	if err != nil {
		return nil
	}
	out := make([]mail.Address, len(list))
	for i, a := range list {
		out[i] = mail.Address{Email: a.Address, Name: a.Name}
	}
	return out
}

// decodeHeader decodes RFC 2047 encoded words, keeping the value as it is when they do not decode.
func decodeHeader(value string) string {
	decoded, err := new(mime.WordDecoder).DecodeHeader(value)
	if err != nil {
		return value
	}
	return decoded
}

// listID returns the RFC 2919 list identifier, the text between the angle brackets.
func listID(value string) string {
	start, end := strings.LastIndexByte(value, '<'), strings.LastIndexByte(value, '>')
	if start < 0 || end < start {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(value[start+1 : end])
}

// authService is the authentication service Gmail names in the Authentication-Results header it
// adds on receipt.
const authService = "mx.google.com"

// authResults reads the first Authentication-Results header Gmail's own service added. A header
// naming another service may have been written by the sender, so it is never read.
func authResults(headers []gmailHeader) mail.AuthResults {
	for _, h := range headers {
		if !strings.EqualFold(h.Name, "Authentication-Results") {
			continue
		}
		fields := strings.Split(h.Value, ";")
		service := strings.Fields(fields[0])
		if len(service) == 0 || !strings.EqualFold(service[0], authService) {
			continue
		}
		var out mail.AuthResults
		for _, f := range fields[1:] {
			method, result := methodResult(f)
			switch {
			case method == "spf" && out.SPF == "":
				out.SPF = result
			case method == "dkim" && out.DKIM == "":
				out.DKIM = result
			case method == "dmarc" && out.DMARC == "":
				out.DMARC = result
			}
		}
		return out
	}
	return mail.AuthResults{}
}

// methodResult splits one result of an Authentication-Results header into its method, without any
// version, and its result keyword, both lower case.
func methodResult(field string) (string, string) {
	method, rest, ok := strings.Cut(strings.TrimSpace(field), "=")
	if !ok {
		return "", ""
	}
	method, _, _ = strings.Cut(strings.TrimSpace(method), "/")
	result := strings.Fields(rest)
	if len(result) == 0 {
		return "", ""
	}
	keyword, _, _ := strings.Cut(result[0], "(")
	return strings.ToLower(method), strings.ToLower(keyword)
}

// attachmentNames returns the file names of the message's parts, depth first.
func attachmentNames(p gmailPart) []string {
	var out []string
	if p.Filename != "" {
		out = append(out, p.Filename)
	}
	for _, c := range p.Parts {
		out = append(out, attachmentNames(c)...)
	}
	return out
}

// messageBody maps a message read in the full format to its body. It takes the first plain text
// part and the first HTML part that are not attachments, depth first, as Gmail returns their bytes.
func messageBody(account string, m gmailMessage) (mail.MessageBody, error) {
	out := mail.MessageBody{AccountID: account, MessageID: m.ID}
	var walk func(p gmailPart) error
	walk = func(p gmailPart) error {
		if p.Filename == "" {
			var slot *string
			switch strings.ToLower(p.MimeType) {
			case "text/plain":
				slot = &out.Text
			case "text/html":
				slot = &out.HTML
			}
			if slot != nil && *slot == "" {
				text, err := partData(p)
				if err != nil {
					return err
				}
				*slot = text
			}
		}
		for _, c := range p.Parts {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(m.Payload); err != nil {
		return mail.MessageBody{}, err
	}
	return out, nil
}

// partData decodes a part's body, which Gmail sends in URL-safe base64.
func partData(p gmailPart) (string, error) {
	if p.Body.Data == "" && p.Body.AttachmentID != "" {
		return "", fmt.Errorf("gmail: a %s body part is held apart from the message: %w", p.MimeType, mail.ErrProvider)
	}
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(p.Body.Data, "="))
	if err != nil {
		return "", fmt.Errorf("gmail: decoding a %s body part: %w", p.MimeType, errors.Join(mail.ErrProvider, err))
	}
	return string(b), nil
}

// threads groups messages into the threads threadIDs names, in that order. Each thread holds its
// messages oldest first, and a thread none of the messages belongs to is left out.
func threads(account string, threadIDs []string, messages []mail.MessageMetadata) []mail.ThreadMetadata {
	var out []mail.ThreadMetadata
	for _, id := range threadIDs {
		th := mail.ThreadMetadata{AccountID: account, ID: id}
		for _, m := range messages {
			if m.ThreadID == id {
				th.Messages = append(th.Messages, m)
			}
		}
		if len(th.Messages) == 0 {
			continue
		}
		slices.SortStableFunc(th.Messages, func(a, b mail.MessageMetadata) int {
			return cmp.Or(cmp.Compare(a.Date, b.Date), strings.Compare(a.ID, b.ID))
		})
		out = append(out, th)
	}
	return out
}

// listing is one page of messages.list or threads.list, as identifiers.
type listing struct {
	Messages []messageRef
	Threads  []string
	Next     string
}

// messageRef is a message as a listing names it.
type messageRef struct {
	ID       string
	ThreadID string
}

// parseListing reads a messages.list or threads.list response.
func parseListing(body []byte) (listing, error) {
	var raw struct {
		Messages []struct {
			ID       string `json:"id"`
			ThreadID string `json:"threadId"`
		} `json:"messages"`
		Threads []struct {
			ID string `json:"id"`
		} `json:"threads"`
		NextPageToken string `json:"nextPageToken"`
	}
	if err := unmarshal(body, &raw, "a listing"); err != nil {
		return listing{}, err
	}
	out := listing{Next: raw.NextPageToken}
	for _, m := range raw.Messages {
		out.Messages = append(out.Messages, messageRef{ID: m.ID, ThreadID: m.ThreadID})
	}
	for _, th := range raw.Threads {
		out.Threads = append(out.Threads, th.ID)
	}
	return out, nil
}

// parseProfile reads the mailbox's current history identifier from a profile response.
func parseProfile(body []byte) (uint64, error) {
	var raw struct {
		HistoryID uint64 `json:"historyId,string"`
	}
	if err := unmarshal(body, &raw, "the profile"); err != nil {
		return 0, err
	}
	if raw.HistoryID == 0 {
		return 0, fmt.Errorf("gmail: the profile has no history identifier: %w", mail.ErrProvider)
	}
	return raw.HistoryID, nil
}

// gmailHistory is one page of history.list.
type gmailHistory struct {
	History []struct {
		ID              uint64          `json:"id,string"`
		MessagesAdded   []historyChange `json:"messagesAdded"`
		MessagesDeleted []historyChange `json:"messagesDeleted"`
		LabelsAdded     []historyChange `json:"labelsAdded"`
		LabelsRemoved   []historyChange `json:"labelsRemoved"`
	} `json:"history"`
	NextPageToken string `json:"nextPageToken"`
	HistoryID     uint64 `json:"historyId,string"`
}

type historyChange struct {
	Message struct {
		ID string `json:"id"`
	} `json:"message"`
	LabelIDs []string `json:"labelIds"`
}

// parseHistory reads a history.list response.
func parseHistory(body []byte) (gmailHistory, error) {
	var h gmailHistory
	if err := unmarshal(body, &h, "the history"); err != nil {
		return gmailHistory{}, err
	}
	if h.HistoryID == 0 {
		return gmailHistory{}, fmt.Errorf("gmail: the history has no history identifier: %w", mail.ErrProvider)
	}
	return h, nil
}

type changeKind uint8

const (
	added changeKind = 1 << iota
	modified
	removed
)

// changes maps one page of history to a change set, netted as the port requires. A label change
// counts only when it touches a label or flag the model shows. When Gmail has more pages, the next
// cursor is the last record on this one, so asking again continues from there. Otherwise it is the
// mailbox's current history identifier.
func changes(account string, labels labelTable, h gmailHistory) mail.ChangeSet {
	var order []string
	kinds := map[string]changeKind{}
	note := func(id string, k changeKind) {
		if _, ok := kinds[id]; !ok {
			order = append(order, id)
		}
		kinds[id] |= k
	}
	shown := func(ids []string) bool {
		return slices.ContainsFunc(ids, func(id string) bool { return !labels.hidden[id] })
	}
	for _, r := range h.History {
		for _, c := range r.MessagesAdded {
			note(c.Message.ID, added)
		}
		for _, c := range r.MessagesDeleted {
			note(c.Message.ID, removed)
		}
		for _, c := range slices.Concat(r.LabelsAdded, r.LabelsRemoved) {
			if shown(c.LabelIDs) {
				note(c.Message.ID, modified)
			}
		}
	}
	next := h.HistoryID
	if h.NextPageToken != "" && len(h.History) > 0 {
		next = h.History[len(h.History)-1].ID
	}
	out := mail.ChangeSet{AccountID: account, Next: cursor(next)}
	for _, id := range order {
		switch k := kinds[id]; {
		case k&removed != 0:
			out.Removed = append(out.Removed, id)
		case k&added != 0:
			out.Added = append(out.Added, id)
		default:
			out.Modified = append(out.Modified, id)
		}
	}
	return out
}

func unmarshal(body []byte, v any, what string) error {
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("gmail: reading %s: %w", what, errors.Join(mail.ErrProvider, err))
	}
	return nil
}
