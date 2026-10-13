package core_test

import (
	"bytes"
	"encoding/base64"
	"mime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/text/encoding/htmlindex"

	"github.com/ppat/mediated-mailbox-mcp/provider/gmail/core"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// labels is the adapter's own label table, as is the parser decide passes, so each decision is
// tested against the lookups the adapter makes.
func labels() core.Labels { return core.Labels{Get: htmlindex.Get, Name: htmlindex.Name} }

// inline is a part of mediaType carrying text inline, as Gmail sends it.
func inline(mediaType, filename, text string) core.Part {
	return core.Part{MimeType: mediaType, Filename: filename, Body: core.Body{Size: int64(len(text)), Data: encode([]byte(text))}}
}

// apart is a part of mediaType that Gmail holds apart, declaring size and carrying headers.
func apart(mediaType string, size int64, headers ...core.Header) core.Part {
	return core.Part{MimeType: mediaType, Headers: headers, Body: core.Body{AttachmentID: "ANGjdJ9-x_1", Size: size}}
}

func multipart(mediaType string, parts ...core.Part) core.Part {
	return core.Part{MimeType: mediaType, Parts: parts}
}

func encode(data []byte) string { return base64.RawURLEncoding.EncodeToString(data) }

// outcome is a decision with the text an Inline decision carries.
type outcome struct {
	Decision core.Decision
	Text     string
}

func decide(payload core.Part) outcome {
	d, text := core.MessageBody(payload, mime.ParseMediaType)
	return outcome{d, text}
}

// Each message's rendered part, chosen as ADR-0131 states and returned in its slot alone.
func TestMessageBodyChoosesTheRenderedPart(t *testing.T) {
	inlined := func(slot core.Slot, text string) outcome {
		return outcome{core.Decision{Act: core.Inline, Slot: slot}, text}
	}
	for _, c := range []struct {
		name    string
		payload core.Part
		want    outcome
	}{
		{"a single HTML part", inline("text/html", "", "<b>"), inlined(core.HTML, "<b>")},
		{"a single text part", inline("text/plain", "", "plain"), inlined(core.Text, "plain")},
		{
			"an HTML attachment beside a text part",
			multipart("multipart/mixed", inline("text/plain", "", "plain"), inline("text/html", "page.html", "<i>")),
			inlined(core.Text, "plain"),
		},
		{
			"the first HTML part, depth first",
			multipart("multipart/mixed",
				multipart("multipart/alternative", inline("text/plain", "", "plain"), inline("text/html", "", "<p>one</p>")),
				inline("text/html", "", "<p>two</p>")),
			inlined(core.HTML, "<p>one</p>"),
		},
		{
			"an empty HTML part beside a text part",
			multipart("multipart/alternative", inline("text/plain", "", "plain"), inline("text/html", "", "")),
			inlined(core.Text, "plain"),
		},
		{
			"an empty HTML part before one that holds content",
			multipart("multipart/mixed", inline("text/html", "", ""), inline("text/html", "", "<p>two</p>")),
			inlined(core.HTML, "<p>two</p>"),
		},
		{"no text and no HTML part", inline("image/png", "", "png"), outcome{Decision: core.Decision{Act: core.Empty}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, decide(c.payload), compare.Options); diff != "" {
				t.Errorf("MessageBody (-want +got):\n%s", diff)
			}
		})
	}
}

// A rendered part held apart within the bound is fetched, with the charset its Content-Type
// declares, and one whose declared size passes the bound is refused with nothing to fetch, as is an
// inline part past the bound. 524288 bytes is the 512 KiB the operator set, written out here rather
// than read from the code (ADR-0131).
func TestMessageBodyOfAPartHeldApartOrPastTheBound(t *testing.T) {
	latin := core.Header{Name: "content-type", Value: `text/html; charset="ISO-8859-1"`}
	alternative := func(size int64) core.Part {
		return multipart("multipart/alternative", inline("text/plain", "", "plain"), apart("text/html", size, latin))
	}
	want := outcome{Decision: core.Decision{Act: core.Fetch, Slot: core.HTML, Attachment: "ANGjdJ9-x_1", Charset: "ISO-8859-1"}}
	if diff := cmp.Diff(want, decide(alternative(524288)), compare.Options); diff != "" {
		t.Errorf("a part held apart at the bound (-want +got):\n%s", diff)
	}
	for name, c := range map[string]struct {
		payload core.Part
		slot    core.Slot
	}{
		"held apart past the bound": {alternative(524289), core.HTML},
		"inline past the bound":     {inline("text/plain", "", strings.Repeat("a", 524289)), core.Text},
	} {
		t.Run(name, func(t *testing.T) {
			want := outcome{Decision: core.Decision{Act: core.Stop, Slot: c.slot, Problem: core.PastBound}}
			if diff := cmp.Diff(want, decide(c.payload), compare.Options); diff != "" {
				t.Errorf("MessageBody (-want +got):\n%s", diff)
			}
		})
	}
}

// A part held apart with no Content-Type, or one naming no charset, is fetched with no charset,
// which is read as us-ascii. One whose Content-Type is present and does not parse is refused, since
// the charset it names cannot be read. Go's parser refuses RFC 2045's comment form, a parameter
// given twice and an empty parameter, all forms real mail carries (ADR-0134).
func TestMessageBodyOfAPartHeldApartReadsItsContentType(t *testing.T) {
	contentType := func(value string) []core.Header { return []core.Header{{Name: "Content-Type", Value: value}} }
	for name, headers := range map[string][]core.Header{
		"no Content-Type":                nil,
		"a Content-Type with no charset": contentType("text/html"),
	} {
		t.Run(name, func(t *testing.T) {
			want := outcome{Decision: core.Decision{Act: core.Fetch, Slot: core.HTML, Attachment: "ANGjdJ9-x_1"}}
			if diff := cmp.Diff(want, decide(apart("text/html", 10, headers...)), compare.Options); diff != "" {
				t.Errorf("MessageBody (-want +got):\n%s", diff)
			}
		})
	}
	for name, value := range map[string]string{
		"a comment":               `text/html; charset=us-ascii (Plain text)`,
		"a parameter given twice": `text/html; charset=utf-8; charset=iso-8859-1`,
		"an empty parameter":      `text/html;; charset=utf-8`,
	} {
		t.Run(name, func(t *testing.T) {
			want := outcome{Decision: core.Decision{Act: core.Stop, Slot: core.HTML, Problem: core.ContentTypeUnparsed}}
			if diff := cmp.Diff(want, decide(apart("text/html", 10, contentType(value)...)), compare.Options); diff != "" {
				t.Errorf("MessageBody (-want +got):\n%s", diff)
			}
		})
	}
}

// Data that is not base64 is the provider's failure, never a refusal.
func TestMessageBodyOfDataThatIsNotBase64(t *testing.T) {
	part := core.Part{MimeType: "text/plain", Body: core.Body{Size: 3, Data: "***"}}
	want := outcome{Decision: core.Decision{Act: core.Stop, Slot: core.Text, Problem: core.NotBase64}}
	if diff := cmp.Diff(want, decide(part), compare.Options); diff != "" {
		t.Errorf("MessageBody (-want +got):\n%s", diff)
	}
	if core.NotBase64.Refused() {
		t.Error("data that is not base64 refuses the body for good")
	}
}

// A part held apart is decoded from the charset it declared to UTF-8, refused when the decoded text
// or the data passes the bound, whatever size the part declared, when its charset cannot be decoded,
// or when it declares UTF-16 or x-user-defined, which would be read as other text than it holds
// (ADR-0131, ADR-0134).
func TestPartText(t *testing.T) {
	if got, problem := core.PartText(encode([]byte("caf\xe9 \x80")), "iso-8859-1", labels()); got != "café €" || problem != core.None {
		t.Errorf("a Latin-1 part decoded to %q with %v, want café € with none", got, problem)
	}
	if got, problem := core.PartText(encode([]byte("<p>ok</p>")), "utf-8", labels()); got != "<p>ok</p>" || problem != core.None {
		t.Errorf("a UTF-8 part decoded to %q with %v", got, problem)
	}
	if got, problem := core.PartText(encode([]byte("plain")), "", labels()); got != "plain" || problem != core.None {
		t.Errorf("a part declaring no charset decoded to %q with %v", got, problem)
	}
	for name, c := range map[string]struct {
		charset string
		data    []byte
		want    core.Problem
	}{
		"data past the bound":                   {"utf-8", []byte(strings.Repeat("a", 524289)), core.PastBound},
		"decoded text past the bound":           {"iso-8859-1", bytes.Repeat([]byte{0xe9}, 300000), core.PastBound},
		"a charset the standard does not name":  {"x-unknown", []byte("a"), core.CharsetRefused},
		"a charset that decodes to replacement": {"iso-2022-kr", []byte("a"), core.CharsetRefused},
		"a UTF-16 part":                         {"utf-16", []byte{0xfe, 0xff, 0x00, 0x68, 0x00, 0x69}, core.CharsetRefused},
		"a UTF-16BE part":                       {"utf-16be", []byte{0x00, 0x68, 0x00, 0x69}, core.CharsetRefused},
		"a user-defined part":                   {"x-user-defined", []byte("caf\xe9"), core.CharsetRefused},
	} {
		t.Run(name, func(t *testing.T) {
			if _, problem := core.PartText(encode(c.data), c.charset, labels()); problem != c.want || !problem.Refused() {
				t.Errorf("PartText returned %v, want %v, a refusal", problem, c.want)
			}
		})
	}
}

// Text fills the slot its decision names, and no other.
func TestFill(t *testing.T) {
	if got := core.Fill("a", "m1", core.HTML, "<p>"); got.HTML != "<p>" || got.Text != "" || got.AccountID != "a" || got.MessageID != "m1" {
		t.Errorf("an HTML part filled %+v", got)
	}
	if got := core.Fill("a", "m1", core.Text, "plain"); got.Text != "plain" || got.HTML != "" {
		t.Errorf("a text part filled %+v", got)
	}
}

// A decision names the part and why, and holds none of the part's data, so the body's text travels
// only as an Inline decision's second result (ADR-0040).
func TestADecisionHoldsNoContent(t *testing.T) {
	mustnotcompile.RequireFields(t, "github.com/ppat/mediated-mailbox-mcp/provider/gmail/core", "Decision",
		"Act Act", "Slot Slot", "Problem Problem", "Attachment string", "Charset string")
}

// Each body read's cap is four thirds, for base64, of what its answer carries at the bound, two parts
// with 256 KiB of headers and part tree for the message read and one part with 64 KiB for the part
// read, and an answer past either refuses the body for good (ADR-0131). The bound is 524288 bytes, so
// the caps are written out here rather than read from the code.
func TestTheReadCapsFollowTheBound(t *testing.T) {
	if core.MessageReadCap != 1660245 || core.PartReadCap != 764586 {
		t.Errorf("the read caps are %d and %d, want 1660245 and 764586", core.MessageReadCap, core.PartReadCap)
	}
	if !core.PastCap.Refused() {
		t.Error("an answer past its read cap does not refuse the body for good")
	}
}
