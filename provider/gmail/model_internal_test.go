package gmail

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// labelsBody is a labels.list response in the shape Google's reference documents, holding every
// kind of label an account has.
const labelsBody = `{
  "labels": [
    {"id": "CHAT", "name": "CHAT", "messageListVisibility": "hide", "labelListVisibility": "labelHide", "type": "system"},
    {"id": "SENT", "name": "SENT", "type": "system"},
    {"id": "INBOX", "name": "INBOX", "type": "system"},
    {"id": "IMPORTANT", "name": "IMPORTANT", "messageListVisibility": "hide", "labelListVisibility": "labelShow", "type": "system"},
    {"id": "TRASH", "name": "TRASH", "type": "system"},
    {"id": "DRAFT", "name": "DRAFT", "type": "system"},
    {"id": "SPAM", "name": "SPAM", "type": "system"},
    {"id": "CATEGORY_FORUMS", "name": "CATEGORY_FORUMS", "type": "system"},
    {"id": "CATEGORY_UPDATES", "name": "CATEGORY_UPDATES", "type": "system"},
    {"id": "CATEGORY_PERSONAL", "name": "CATEGORY_PERSONAL", "type": "system"},
    {"id": "CATEGORY_PROMOTIONS", "name": "CATEGORY_PROMOTIONS", "type": "system"},
    {"id": "CATEGORY_SOCIAL", "name": "CATEGORY_SOCIAL", "type": "system"},
    {"id": "STARRED", "name": "STARRED", "type": "system"},
    {"id": "UNREAD", "name": "UNREAD", "type": "system"},
    {"id": "Label_12", "name": "finance", "messageListVisibility": "show", "labelListVisibility": "labelShow", "type": "user"},
    {"id": "Label_13", "name": "reading/digest", "type": "user"},
    {"id": "Label_14", "name": "reading", "type": "user"}
  ]
}`

func accountLabels(t *testing.T) labelTable {
	t.Helper()
	labels, err := parseLabels([]byte(labelsBody))
	if err != nil {
		t.Fatalf("parseLabels: %v", err)
	}
	return labels
}

// The model shows user labels by name and five system labels by identifier. The flags' labels
// and the labels Gmail assigns itself are not labels in the model.
func TestLabels(t *testing.T) {
	labels := accountLabels(t)
	want := []mail.Label{
		{AccountID: "a", Path: "DRAFT"},
		{AccountID: "a", Path: "INBOX"},
		{AccountID: "a", Path: "SENT"},
		{AccountID: "a", Path: "SPAM"},
		{AccountID: "a", Path: "TRASH"},
		{AccountID: "a", Path: "finance"},
		{AccountID: "a", Path: "reading"},
		{AccountID: "a", Path: "reading/digest"},
	}
	if diff := cmp.Diff(want, labels.canonicalLabels("a"), compare.Options); diff != "" {
		t.Errorf("canonical labels (-want +got):\n%s", diff)
	}
	hidden := map[string]bool{
		"CHAT": true, "IMPORTANT": true, "CATEGORY_FORUMS": true, "CATEGORY_UPDATES": true,
		"CATEGORY_PERSONAL": true, "CATEGORY_PROMOTIONS": true, "CATEGORY_SOCIAL": true,
	}
	if diff := cmp.Diff(hidden, labels.hidden, compare.Options); diff != "" {
		t.Errorf("hidden labels (-want +got):\n%s", diff)
	}
}

func TestParseLabel(t *testing.T) {
	got, err := parseLabel([]byte(`{"id": "Label_20", "name": "project/phase", "messageListVisibility": "show", "labelListVisibility": "labelShow", "type": "user"}`))
	if err != nil {
		t.Fatalf("parseLabel: %v", err)
	}
	if got != "project/phase" {
		t.Errorf("parseLabel = %q, want %q", got, "project/phase")
	}
}

// metadataBody is a messages.get response in the metadata format, in the shape Google's reference
// documents, for a message in a thread of two.
const metadataBody = `{
  "id": "18c2f0a1b2c3d4e5",
  "threadId": "18c2f0a1b2c3d4e0",
  "labelIds": ["IMPORTANT", "CATEGORY_UPDATES", "INBOX", "Label_12", "STARRED", "Label_99"],
  "snippet": "Your statement is ready &amp; it&#39;s &lt;attached&gt;",
  "sizeEstimate": 48213,
  "historyId": "9876543",
  "internalDate": "1725181200250",
  "payload": {
    "partId": "",
    "mimeType": "multipart/mixed",
    "filename": "",
    "headers": [
      {"name": "Authentication-Results", "value": "evil.example; spf=pass; dkim=pass; dmarc=pass"},
      {"name": "From", "value": "=?UTF-8?Q?B=C3=A4nk?= <alerts@bank.example>"},
      {"name": "To", "value": "You <you@example.com>, other@example.com"},
      {"name": "Cc", "value": "\"Doe, Jane\" <jane@example.com>"},
      {"name": "Subject", "value": "=?UTF-8?Q?Statement_f=C3=BCr_September?="},
      {"name": "List-Id", "value": "Bank alerts <alerts.bank.example>"},
      {"name": "Authentication-Results", "value": "mx.google.com; dkim=pass header.i=@bank.example header.s=s1 header.b=abc; spf=softfail (google.com: domain of transitioning alerts@bank.example) smtp.mailfrom=alerts@bank.example; dmarc=fail (p=NONE sp=NONE dis=NONE) header.from=bank.example"}
    ],
    "body": {"size": 0},
    "parts": [
      {"partId": "0", "mimeType": "text/plain", "filename": "", "body": {"size": 10}},
      {"partId": "1", "mimeType": "application/pdf", "filename": "statement.pdf", "body": {"attachmentId": "ANGjdJ8", "size": 40000}}
    ]
  }
}`

func TestMetadata(t *testing.T) {
	m, err := parseMessage([]byte(metadataBody))
	if err != nil {
		t.Fatalf("parseMessage: %v", err)
	}
	want := mail.MessageMetadata{
		AccountID:       "a",
		ID:              "18c2f0a1b2c3d4e5",
		ThreadID:        "18c2f0a1b2c3d4e0",
		From:            mail.Address{Email: "alerts@bank.example", Name: "Bänk"},
		To:              []mail.Address{{Email: "you@example.com", Name: "You"}, {Email: "other@example.com"}},
		Cc:              []mail.Address{{Email: "jane@example.com", Name: "Doe, Jane"}},
		Subject:         "Statement für September",
		Date:            1725181200250,
		Labels:          []string{mail.Inbox, "finance"},
		Flags:           mail.Flags{Read: true, Starred: true},
		SizeBytes:       48213,
		HasAttachments:  true,
		AttachmentNames: []string{"statement.pdf"},
		Snippet:         "Your statement is ready & it's <attached>",
		ListID:          "alerts.bank.example",
		AuthResults:     mail.AuthResults{SPF: "softfail", DKIM: "pass", DMARC: "fail"},
	}
	if diff := cmp.Diff(want, metadata("a", accountLabels(t), m), compare.Options); diff != "" {
		t.Errorf("metadata (-want +got):\n%s", diff)
	}
}

// A message with the fewest headers, unread, whose sender does not parse.
func TestMetadataOfABareMessage(t *testing.T) {
	m, err := parseMessage([]byte(`{
  "id": "18c2f0a1b2c3d4e6", "threadId": "18c2f0a1b2c3d4e6",
  "labelIds": ["UNREAD", "TRASH"],
  "snippet": "", "sizeEstimate": 120, "internalDate": "1725181200000",
  "payload": {"mimeType": "text/plain", "filename": "", "headers": [
    {"name": "from", "value": "not an address"},
    {"name": "Subject", "value": "plain"},
    {"name": "List-Id", "value": "bare.list.example"}
  ], "body": {"size": 5}}
}`))
	if err != nil {
		t.Fatalf("parseMessage: %v", err)
	}
	want := mail.MessageMetadata{
		AccountID: "a",
		ID:        "18c2f0a1b2c3d4e6",
		ThreadID:  "18c2f0a1b2c3d4e6",
		From:      mail.Address{Name: "not an address"},
		Subject:   "plain",
		Date:      1725181200000,
		Labels:    []string{mail.Trash},
		SizeBytes: 120,
		ListID:    "bare.list.example",
	}
	if diff := cmp.Diff(want, metadata("a", accountLabels(t), m), compare.Options); diff != "" {
		t.Errorf("metadata (-want +got):\n%s", diff)
	}
}

func TestParseMessageRefusesAnUnusableBody(t *testing.T) {
	for name, body := range map[string]string{
		"not JSON":      `<html>`,
		"no identifier": `{"threadId": "t"}`,
		"no thread":     `{"id": "m"}`,
		"a bad date":    `{"id": "m", "threadId": "t", "internalDate": "yesterday"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseMessage([]byte(body)); !errors.Is(err, mail.ErrProvider) {
				t.Errorf("parseMessage returned %v, want an error wrapping %v", err, mail.ErrProvider)
			}
		})
	}
}

// fullBody is a messages.get response in the full format, a mixed message holding an alternative
// part and a text attachment.
const fullBody = `{
  "id": "18c2f0a1b2c3d4e5", "threadId": "18c2f0a1b2c3d4e0", "labelIds": ["INBOX"],
  "payload": {"mimeType": "multipart/mixed", "filename": "", "headers": [], "body": {"size": 0}, "parts": [
    {"mimeType": "multipart/alternative", "filename": "", "body": {"size": 0}, "parts": [
      {"mimeType": "text/plain", "filename": "", "body": {"size": 21, "data": "SGVsbG8sIHdvcmxkIQpMaW5lIHR3bz8-"}},
      {"mimeType": "text/html", "filename": "", "body": {"size": 22, "data": "PHA-SGVsbG8sIHdvcmxkITwvcD4="}}
    ]},
    {"mimeType": "text/plain", "filename": "notes.txt", "body": {"size": 5, "data": "bm90ZXM"}}
  ]}
}`

func TestMessageBody(t *testing.T) {
	m, err := parseMessage([]byte(fullBody))
	if err != nil {
		t.Fatalf("parseMessage: %v", err)
	}
	got, err := messageBody("a", m)
	if err != nil {
		t.Fatalf("messageBody: %v", err)
	}
	want := mail.MessageBody{AccountID: "a", MessageID: "18c2f0a1b2c3d4e5", Text: "Hello, world!\nLine two?>", HTML: "<p>Hello, world!</p>"}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("messageBody (-want +got):\n%s", diff)
	}
}

func TestMessageBodyOfASinglePartMessage(t *testing.T) {
	m, err := parseMessage([]byte(`{"id": "m1", "threadId": "t1", "payload": {"mimeType": "text/html", "filename": "", "body": {"size": 4, "data": "PGI-"}}}`))
	if err != nil {
		t.Fatalf("parseMessage: %v", err)
	}
	got, err := messageBody("a", m)
	if err != nil {
		t.Fatalf("messageBody: %v", err)
	}
	if diff := cmp.Diff(mail.MessageBody{AccountID: "a", MessageID: "m1", HTML: "<b>"}, got, compare.Options); diff != "" {
		t.Errorf("messageBody (-want +got):\n%s", diff)
	}
}

func TestMessageBodyRefusesAPartItCannotRead(t *testing.T) {
	for name, part := range map[string]string{
		"held apart": `{"mimeType": "text/plain", "filename": "", "body": {"attachmentId": "ANGjdJ9", "size": 900000}}`,
		"not base64": `{"mimeType": "text/plain", "filename": "", "body": {"size": 3, "data": "***"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			m, err := parseMessage([]byte(`{"id": "m1", "threadId": "t1", "payload": ` + part + `}`))
			if err != nil {
				t.Fatalf("parseMessage: %v", err)
			}
			if _, err := messageBody("a", m); !errors.Is(err, mail.ErrProvider) {
				t.Errorf("messageBody returned %v, want an error wrapping %v", err, mail.ErrProvider)
			}
		})
	}
}

// Messages are grouped into the threads named, in that order, each oldest first, and a thread with
// no message left is left out.
func TestThreads(t *testing.T) {
	msg := func(thread, id string, date mail.UnixMilli) mail.MessageMetadata {
		return mail.MessageMetadata{ID: id, ThreadID: thread, Date: date}
	}
	got := threads("a", []string{"t2", "t1", "gone"}, []mail.MessageMetadata{
		msg("t1", "m3", 30), msg("t2", "m2", 20), msg("t1", "m1", 10), msg("t1", "m0", 30),
	})
	want := []mail.ThreadMetadata{
		{AccountID: "a", ID: "t2", Messages: []mail.MessageMetadata{msg("t2", "m2", 20)}},
		{AccountID: "a", ID: "t1", Messages: []mail.MessageMetadata{msg("t1", "m1", 10), msg("t1", "m0", 30), msg("t1", "m3", 30)}},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("threads (-want +got):\n%s", diff)
	}
}

func TestParseListing(t *testing.T) {
	cases := []struct {
		name string
		body string
		want listing
	}{
		{
			"messages.list",
			`{"messages": [{"id": "m2", "threadId": "t1"}, {"id": "m1", "threadId": "t1"}], "nextPageToken": "0342", "resultSizeEstimate": 2}`,
			listing{Messages: []messageRef{{ID: "m2", ThreadID: "t1"}, {ID: "m1", ThreadID: "t1"}}, Next: "0342"},
		},
		{
			"threads.list",
			`{"threads": [{"id": "t1", "snippet": "hi", "historyId": "5"}, {"id": "t2", "snippet": "", "historyId": "6"}], "resultSizeEstimate": 2}`,
			listing{Threads: []string{"t1", "t2"}},
		},
		{"an empty mailbox", `{"resultSizeEstimate": 0}`, listing{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseListing([]byte(c.body))
			if err != nil {
				t.Fatalf("parseListing: %v", err)
			}
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("parseListing (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseProfile(t *testing.T) {
	got, err := parseProfile([]byte(`{"emailAddress": "you@example.com", "messagesTotal": 7, "threadsTotal": 6, "historyId": "9876543"}`))
	if err != nil {
		t.Fatalf("parseProfile: %v", err)
	}
	if got != 9876543 {
		t.Errorf("parseProfile = %d, want 9876543", got)
	}
	if _, err := parseProfile([]byte(`{"emailAddress": "you@example.com"}`)); !errors.Is(err, mail.ErrProvider) {
		t.Errorf("parseProfile without a history identifier returned %v, want an error wrapping %v", err, mail.ErrProvider)
	}
}

// historyBody is a history.list response in the shape Google's reference documents.
const historyBody = `{
  "history": [
    {"id": "101", "messages": [{"id": "m1", "threadId": "t1"}], "messagesAdded": [{"message": {"id": "m1", "threadId": "t1", "labelIds": ["INBOX", "UNREAD"]}}]},
    {"id": "102", "messages": [{"id": "m2", "threadId": "t2"}], "labelsAdded": [{"message": {"id": "m2", "threadId": "t2", "labelIds": ["INBOX", "IMPORTANT"]}, "labelIds": ["IMPORTANT"]}]},
    {"id": "103", "messages": [{"id": "m3", "threadId": "t3"}], "labelsRemoved": [{"message": {"id": "m3", "threadId": "t3", "labelIds": []}, "labelIds": ["UNREAD"]}]},
    {"id": "104", "messages": [{"id": "m1", "threadId": "t1"}], "labelsAdded": [{"message": {"id": "m1", "threadId": "t1", "labelIds": ["INBOX", "Label_12"]}, "labelIds": ["Label_12"]}]},
    {"id": "105", "messages": [{"id": "m4", "threadId": "t4"}], "messagesAdded": [{"message": {"id": "m4", "threadId": "t4", "labelIds": ["INBOX"]}}]},
    {"id": "106", "messages": [{"id": "m4", "threadId": "t4"}], "messagesDeleted": [{"message": {"id": "m4", "threadId": "t4"}}]},
    {"id": "107", "messages": [{"id": "m5", "threadId": "t5"}], "labelsAdded": [{"message": {"id": "m5", "threadId": "t5", "labelIds": ["STARRED"]}, "labelIds": ["STARRED", "CATEGORY_SOCIAL"]}]},
    {"id": "108", "messages": [{"id": "m6", "threadId": "t6"}], "labelsRemoved": [{"message": {"id": "m6", "threadId": "t6", "labelIds": []}, "labelIds": ["Label_77"]}]}
  ],
  "historyId": "112"
}`

// A page of history nets to one entry per message. An add and a change is an add, anything
// deleted is removed, and a change only to labels the model leaves out is no change. A label
// that is no longer listed counts, since it may have been one the model showed.
func TestChanges(t *testing.T) {
	h, err := parseHistory([]byte(historyBody))
	if err != nil {
		t.Fatalf("parseHistory: %v", err)
	}
	want := mail.ChangeSet{
		AccountID: "a",
		Added:     []string{"m1"},
		Modified:  []string{"m3", "m5", "m6"},
		Removed:   []string{"m4"},
		Next:      "gmail.1.h.112",
	}
	if diff := cmp.Diff(want, changes("a", accountLabels(t), h), compare.Options); diff != "" {
		t.Errorf("changes (-want +got):\n%s", diff)
	}
}

// When Gmail has more pages, the next cursor is the last record read, so asking again continues
// from there. An empty history leaves the cursor at the mailbox's current identifier.
func TestChangesNextCursor(t *testing.T) {
	cases := []struct {
		name string
		body string
		want mail.ChangeSet
	}{
		{
			"more pages",
			`{"history": [{"id": "201", "messagesAdded": [{"message": {"id": "m1"}}]}, {"id": "205", "messagesAdded": [{"message": {"id": "m2"}}]}], "nextPageToken": "abc", "historyId": "300"}`,
			mail.ChangeSet{AccountID: "a", Added: []string{"m1", "m2"}, Next: "gmail.1.h.205"},
		},
		{"no activity", `{"historyId": "300"}`, mail.ChangeSet{AccountID: "a", Next: "gmail.1.h.300"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, err := parseHistory([]byte(c.body))
			if err != nil {
				t.Fatalf("parseHistory: %v", err)
			}
			if diff := cmp.Diff(c.want, changes("a", accountLabels(t), h), compare.Options); diff != "" {
				t.Errorf("changes (-want +got):\n%s", diff)
			}
		})
	}
}
