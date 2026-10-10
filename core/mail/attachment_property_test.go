package mail_test

import (
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

// A postcondition over every message's attachments, whatever the sender wrote in their media types
// and file names. Every type is a word of the vocabulary, each word once and sorted, and the message
// has types exactly when it has attachment names, one name for each part in the order given
// (ADR-0123). A mapping passing on a raw media type or extension fails, and so does a set computed
// from other parts than the names.
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
		for _, w := range m.AttachmentTypes {
			if !vocabulary[w] {
				t.Fatalf("%+v: the type %q is not a word of the vocabulary", parts, w)
			}
		}
		if !slices.IsSorted(m.AttachmentTypes) || len(slices.Compact(slices.Clone(m.AttachmentTypes))) != len(m.AttachmentTypes) {
			t.Fatalf("%+v: the types %q are not each word once and sorted", parts, m.AttachmentTypes)
		}
		if m.HasAttachments != (len(parts) > 0) || (len(m.AttachmentTypes) > 0) != (len(parts) > 0) || !slices.Equal(m.AttachmentNames, names) {
			t.Fatalf("%+v: has attachments %v, names %q, types %q", parts, m.HasAttachments, m.AttachmentNames, m.AttachmentTypes)
		}
	})
}

// kind names what a drawn message exercises, by how its first attachment's type is decided.
func attachmentKind(parts []part) string {
	if len(parts) == 0 {
		return "no attachments"
	}
	essence, _, _ := strings.Cut(parts[0].MediaType, ";")
	switch strings.ToLower(strings.TrimSpace(essence)) {
	case "application/octet-stream", "":
		return "the file name decides"
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
		"the file name decides":      0.1,
		"a known media type decides": 0.1,
		"a made-up media type":       0.1,
	})
}
