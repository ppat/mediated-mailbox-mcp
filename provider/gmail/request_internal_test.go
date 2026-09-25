package gmail

import (
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// sent is what a request puts on the wire.
type sent struct {
	Method string
	URL    string
	Body   string
	Units  int
}

func onWire(r request) sent {
	return sent{Method: r.Method, URL: r.URL(), Body: r.Body, Units: r.Units}
}

func must[T any](t *testing.T) func(T, error) T {
	return func(v T, err error) T {
		t.Helper()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return v
	}
}

// Each builder produces the request Gmail's reference page documents for its method, with every
// listing reaching into the trash and spam, and carries the units Gmail's quota page charges for it.
func TestRequests(t *testing.T) {
	cases := []struct {
		name string
		got  request
		want sent
	}{
		{"labels.list", labelsRequest(), sent{"GET", "https://gmail.googleapis.com/gmail/v1/users/me/labels", "", 1}},
		{
			"labels.create",
			must[request](t)(createLabelRequest(`reading/digest "weekly"`)),
			sent{"POST", "https://gmail.googleapis.com/gmail/v1/users/me/labels", `{"name":"reading/digest \"weekly\""}`, 5},
		},
		{
			"threads.list with search text, labels and a page",
			threadsRequest(search{Text: `from:"a@example.com" after:99`, LabelIDs: []string{"Label_1", "INBOX"}}, "0987", 50),
			sent{"GET", "https://gmail.googleapis.com/gmail/v1/users/me/threads?includeSpamTrash=true&labelIds=Label_1&labelIds=INBOX&maxResults=50&pageToken=0987&q=from%3A%22a%40example.com%22+after%3A99", "", 10},
		},
		{
			"threads.list for every thread",
			threadsRequest(search{}, "", 10),
			sent{"GET", "https://gmail.googleapis.com/gmail/v1/users/me/threads?includeSpamTrash=true&maxResults=10", "", 10},
		},
		{
			"messages.list",
			messagesRequest("12", 500),
			sent{"GET", "https://gmail.googleapis.com/gmail/v1/users/me/messages?includeSpamTrash=true&maxResults=500&pageToken=12", "", 5},
		},
		{
			"messages.get through the metadata mask",
			metadataRequest("18c2f0a1b2c3d4e5"),
			sent{"GET", "https://gmail.googleapis.com/gmail/v1/users/me/messages/18c2f0a1b2c3d4e5?" + url.Values{"fields": {metadataMask}, "format": {"full"}}.Encode(), "", 20},
		},
		{
			"messages.get in the full format",
			messageRequest("18c2f0a1b2c3d4e5", formatFull),
			sent{"GET", "https://gmail.googleapis.com/gmail/v1/users/me/messages/18c2f0a1b2c3d4e5?format=full", "", 20},
		},
		{
			"messages.get in the minimal format",
			messageRequest("18c2f0a1b2c3d4e5", formatMinimal),
			sent{"GET", "https://gmail.googleapis.com/gmail/v1/users/me/messages/18c2f0a1b2c3d4e5?format=minimal", "", 20},
		},
		{
			"threads.get",
			threadRequest("18c2f0a1b2c3d4e0"),
			sent{"GET", "https://gmail.googleapis.com/gmail/v1/users/me/threads/18c2f0a1b2c3d4e0?" + url.Values{"fields": {"id,historyId,messages(" + metadataMask + ")"}, "format": {"full"}}.Encode(), "", 40},
		},
		{
			"messages.modify",
			must[request](t)(modifyRequest("18c2f0a1b2c3d4e5", []string{"TRASH"}, []string{"INBOX"})),
			sent{"POST", "https://gmail.googleapis.com/gmail/v1/users/me/messages/18c2f0a1b2c3d4e5/modify", `{"addLabelIds":["TRASH"],"removeLabelIds":["INBOX"]}`, 5},
		},
		{
			"messages.modify removing only",
			must[request](t)(modifyRequest("18c2f0a1b2c3d4e5", nil, []string{"UNREAD"})),
			sent{"POST", "https://gmail.googleapis.com/gmail/v1/users/me/messages/18c2f0a1b2c3d4e5/modify", `{"removeLabelIds":["UNREAD"]}`, 5},
		},
		{"users.getProfile", profileRequest(), sent{"GET", "https://gmail.googleapis.com/gmail/v1/users/me/profile", "", 1}},
		{
			"history.list",
			historyRequest(9876543, 500),
			sent{"GET", "https://gmail.googleapis.com/gmail/v1/users/me/history?maxResults=500&startHistoryId=9876543", "", 2},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, onWire(c.got), compare.Options); diff != "" {
				t.Errorf("request (-want +got):\n%s", diff)
			}
		})
	}
}

// An identifier that could change the path it is written into names no message.
func TestValidID(t *testing.T) {
	for id, want := range map[string]bool{
		"18c2f0a1b2c3d4e5": true,
		"no-such-message":  true,
		"":                 false,
		"..":               false,
		"a/b":              false,
		"a?b":              false,
		"a%2Fb":            false,
		"a b":              false,
	} {
		if got := validID(id); got != want {
			t.Errorf("validID(%q) = %v, want %v", id, got, want)
		}
	}
}

// metadataMask is the metadata mask written out, six levels of parts deep.
const metadataMask = "id,threadId,labelIds,snippet,historyId,internalDate,sizeEstimate,payload(mimeType,filename,headers," +
	"parts(mimeType,filename,parts(mimeType,filename,parts(mimeType,filename,parts(mimeType,filename,parts(mimeType,filename,parts(mimeType,filename)))))))"

// maskField is one field a fields mask selects, with the fields it selects beneath it. A field with
// no sub-selection returns everything beneath it.
type maskField struct {
	name     string
	selected []maskField
}

// parseMask reads a fields mask in Google's partial-response syntax, fields separated by commas,
// each optionally followed by a parenthesised sub-selection.
func parseMask(t *testing.T, mask string) []maskField {
	t.Helper()
	fields, rest := parseMaskList(t, mask)
	if rest != "" {
		t.Fatalf("the mask %q has %q left after its fields", mask, rest)
	}
	return fields
}

func parseMaskList(t *testing.T, s string) ([]maskField, string) {
	t.Helper()
	var out []maskField
	for {
		end := strings.IndexAny(s, ",()")
		if end < 0 {
			end = len(s)
		}
		f := maskField{name: s[:end]}
		if f.name == "" {
			t.Fatalf("the mask holds an empty field name before %q", s)
		}
		s = s[end:]
		if strings.HasPrefix(s, "(") {
			f.selected, s = parseMaskList(t, s[1:])
			if !strings.HasPrefix(s, ")") {
				t.Fatalf("a sub-selection of %q is not closed", f.name)
			}
			s = s[1:]
		}
		out = append(out, f)
		if !strings.HasPrefix(s, ",") {
			return out, s
		}
		s = s[1:]
	}
}

// The fields a metadata read may select, as Google's reference names them. Each leaf holds no body
// content. Each object holds some beneath it, since a message part's body is part of the object,
// so an object is selected only with a sub-selection naming what it may return.
var (
	maskLeaves  = []string{"id", "threadId", "labelIds", "snippet", "historyId", "internalDate", "sizeEstimate", "mimeType", "filename", "headers"}
	maskObjects = []string{"messages", "payload", "parts"}
)

// checkMask returns the names of every field fields selects at any depth, and a problem for each
// field that is not an allowed leaf without a sub-selection or an allowed object with one.
func checkMask(fields []maskField, path string) (names, problems []string) {
	for _, f := range fields {
		at := path + f.name
		switch {
		case slices.Contains(maskLeaves, f.name) && f.selected == nil:
		case slices.Contains(maskObjects, f.name) && f.selected != nil:
			n, p := checkMask(f.selected, at+"/")
			names, problems = append(names, n...), append(problems, p...)
		case slices.Contains(maskObjects, f.name):
			problems = append(problems, "the mask selects "+at+" whole, and everything beneath it, a body included, comes back")
		default:
			problems = append(problems, "the mask selects "+at+", which is not a field a metadata read may return")
		}
		names = append(names, f.name)
	}
	return names, problems
}

// The metadata reads select only fields that hold no body content, so Google returns none on any
// metadata path, and the body fetch stays the port's one body path (ADR-0010). The mask is read
// structurally, since an object selected without a sub-selection returns everything beneath it
// while naming no body field. A mask that selects anything off the lists, selects an object whole,
// or a read that loses the mask, fails here.
func TestMetadataReadsNameNoBody(t *testing.T) {
	for name, r := range map[string]request{
		"messages.get": metadataRequest("18c2f0a1b2c3d4e5"),
		"threads.get":  threadRequest("18c2f0a1b2c3d4e0"),
	} {
		t.Run(name, func(t *testing.T) {
			if got := r.Query.Get("format"); got != formatFull {
				t.Errorf("the read's format is %q, want %q under its mask", got, formatFull)
			}
			fields := r.Query["fields"]
			if len(fields) != 1 {
				t.Fatalf("the read carries %d field masks, want one", len(fields))
			}
			names, problems := checkMask(parseMask(t, fields[0]), "")
			for _, p := range problems {
				t.Error(p)
			}
			if !slices.Contains(names, "filename") {
				t.Errorf("the read's mask names no part's file name, so no attachment is seen")
			}
		})
	}
}

// The structural check refuses the masks that would return a body.
func TestTheMaskCheckRefusesABodyPath(t *testing.T) {
	for name, mask := range map[string]string{
		"a body field":           "id,payload(mimeType,body)",
		"a whole payload":        "id,payload",
		"a whole innermost part": "id,payload(mimeType,parts(mimeType,parts))",
		"a wildcard":             "*",
		"a raw message":          "id,raw",
	} {
		t.Run(name, func(t *testing.T) {
			if _, problems := checkMask(parseMask(t, mask), ""); len(problems) == 0 {
				t.Errorf("the check accepted %q", mask)
			}
		})
	}
}
