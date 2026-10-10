package mail_test

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

// part is one attachment as the sender writes it, drawn on its own (ADR-0069). The media type and the
// extension are each a known one, a generic one or text the sender made up, in any case, with
// parameters or without.
type part struct {
	MediaType, Filename string
}

// The media types and extensions a draw picks from beside made-up text. Each sits in one of ADR-0123's
// tables, so the draw reaches every way the mapping decides.
var (
	knownMediaTypes = []string{
		"application/pdf", "image/png", "audio/mpeg", "video/mp4", "text/plain", "text/calendar", "text/vcard",
		"application/pkcs7-signature", "application/msword", "application/vnd.ms-excel.sheet.macroEnabled.12",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation", "application/zip",
		"message/rfc822",
	}
	knownExtensions = []string{"pdf", "jpg", "mp3", "mov", "txt", "ics", "vcf", "p7s", "docx", "csv", "pptx", "gz", "eml"}
)

// drawParts draws a message's attachments, none to four.
func drawParts(t *rapid.T) []part {
	return rapid.SliceOfN(rapid.Custom(func(t *rapid.T) part {
		var mediaType string
		switch rapid.SampledFrom([]string{"known", "generic", "made up"}).Draw(t, "media type kind") {
		case "known":
			mediaType = rapid.SampledFrom(knownMediaTypes).Draw(t, "media type")
		case "generic":
			mediaType = rapid.SampledFrom([]string{"application/octet-stream", ""}).Draw(t, "media type")
		default:
			mediaType = rapid.String().Draw(t, "media type")
		}
		if rapid.Bool().Draw(t, "upper case") {
			mediaType = strings.ToUpper(mediaType)
		}
		if rapid.Bool().Draw(t, "parameters") {
			mediaType += "; name=" + rapid.String().Draw(t, "parameter")
		}
		filename := rapid.String().Draw(t, "stem")
		switch rapid.SampledFrom([]string{"known", "made up", "none"}).Draw(t, "extension kind") {
		case "known":
			filename += "." + rapid.SampledFrom(knownExtensions).Draw(t, "extension")
		case "made up":
			filename += "." + rapid.String().Draw(t, "extension")
		}
		return part{MediaType: mediaType, Filename: filename}
	}), 0, 4).Draw(t, "parts")
}

// The stored form ADR-0123 states, written out here rather than read from the package. A media type
// is empty or a lowercase type and subtype of RFC 6838's restricted names, each at most 127 long, and
// an extension is empty or one to 16 lowercase ASCII letters and digits.
var (
	storedMediaType = regexp.MustCompile(`^([a-z0-9][a-z0-9!#$&^_.+-]{0,126}/[a-z0-9][a-z0-9!#$&^_.+-]{0,126})?$`)
	storedExtension = regexp.MustCompile(`^[a-z0-9]{0,16}$`)
)

// A postcondition over every message's attachments, whatever the sender wrote in their media types
// and file names. Every type served is a word of the vocabulary, each word once and sorted, every
// medium kept is in the stored form, each pair once and sorted, and the message has media and types
// exactly when it has attachment names, one name for each part in the order given (ADR-0123). A
// mapping passing on a raw media type or extension fails, so does a normalization keeping text the
// stored form refuses, and so does a set computed from other parts than the names.
func TestEveryAttachmentTypeIsAWordOfTheVocabulary(t *testing.T) {
	property.Check(t, drawParts, func(t rapid.TB, parts []part) {
		in := make([]mail.AttachmentPart, len(parts))
		names := make([]string, len(parts))
		for i, p := range parts {
			in[i] = mail.AttachmentPart{MediaType: p.MediaType, Filename: p.Filename}
			names[i] = p.Filename
		}
		var m mail.MessageMetadata
		m.SetAttachments(in)
		for _, a := range m.AttachmentMedia {
			if !storedMediaType.MatchString(a.MediaType) || !storedExtension.MatchString(a.Extension) {
				t.Fatalf("%+v: the medium %+v is not in the stored form", parts, a)
			}
		}
		if !slices.IsSortedFunc(m.AttachmentMedia, compareMedia) || len(slices.Compact(slices.Clone(m.AttachmentMedia))) != len(m.AttachmentMedia) {
			t.Fatalf("%+v: the media %+v are not each pair once and sorted", parts, m.AttachmentMedia)
		}
		types := mail.AttachmentTypes(m.AttachmentMedia)
		for _, w := range types {
			if !vocabulary[w] {
				t.Fatalf("%+v: the type %q is not a word of the vocabulary", parts, w)
			}
		}
		if !slices.IsSorted(types) || len(slices.Compact(slices.Clone(types))) != len(types) {
			t.Fatalf("%+v: the types %q are not each word once and sorted", parts, types)
		}
		has := len(parts) > 0
		if m.HasAttachments != has || (len(m.AttachmentMedia) > 0) != has || (len(types) > 0) != has || !slices.Equal(m.AttachmentNames, names) {
			t.Fatalf("%+v: has attachments %v, names %q, media %+v, types %q", parts, m.HasAttachments, m.AttachmentNames, m.AttachmentMedia, types)
		}
		// The mapping returns only the vocabulary's words over media that were never normalized too,
		// such as a stored row a writer bypassing the normalization could leave.
		for _, p := range parts {
			if w := mail.AttachmentTypeOf(mail.AttachmentMedia{MediaType: p.MediaType, Extension: p.Filename}); !vocabulary[w] {
				t.Fatalf("%+v: the type %q of raw media is not a word of the vocabulary", p, w)
			}
		}
	})
}

// compareMedia orders media by media type, then extension, as the model sorts them.
func compareMedia(a, b mail.AttachmentMedia) int {
	return cmp.Or(strings.Compare(a.MediaType, b.MediaType), strings.Compare(a.Extension, b.Extension))
}

// attachmentKind names what a drawn message exercises, by how its first attachment's type is decided.
func attachmentKind(parts []part) string {
	if len(parts) == 0 {
		return "no attachments"
	}
	essence, _, _ := strings.Cut(parts[0].MediaType, ";")
	switch strings.ToLower(strings.TrimSpace(essence)) {
	case "application/octet-stream", "":
		return "the extension decides"
	}
	for _, k := range knownMediaTypes {
		if strings.EqualFold(strings.TrimSpace(essence), k) {
			return "a known media type decides"
		}
	}
	return "a made-up media type"
}

// The generator report for the property above. Each minimum catches a kind the generator stops
// producing. Across thirty seeds at 200 cases no kind fell below 14 percent.
func TestEveryAttachmentTypeIsAWordOfTheVocabularyMix(t *testing.T) {
	property.Report(t, drawParts, attachmentKind, map[string]float64{
		"no attachments":             0.05,
		"the extension decides":      0.1,
		"a known media type decides": 0.1,
		"a made-up media type":       0.1,
	})
}
