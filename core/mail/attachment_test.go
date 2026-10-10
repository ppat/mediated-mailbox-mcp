package mail_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
)

// vocabulary is ADR-0123's closed vocabulary, written out here rather than read from the package, so
// a mapping that returned a word outside it fails.
var vocabulary = map[mail.AttachmentType]bool{
	"pdf": true, "image": true, "audio": true, "video": true, "text": true, "calendar": true, "contact": true,
	"document": true, "spreadsheet": true, "presentation": true, "archive": true, "message": true,
	"signature": true, "other": true,
}

// ADR-0123's tables, over the media the model keeps of each attachment. The media type decides, its
// parameters dropped and its case ignored, the extension decides only when the media type is
// application/octet-stream or missing, and anything neither names is other. A media type and a file
// name holding marker text (ADR-0044) give a word holding none of it, so a search of the served types
// for marker text finds nothing.
func TestAnAttachmentsTypeIsAWordOfTheVocabulary(t *testing.T) {
	cases := []struct {
		mediaType, filename string
		want                mail.AttachmentType
	}{
		{"application/pdf", "statement.bin", "pdf"},
		{`Application/PDF; name="statement.pdf"`, "statement.pdf", "pdf"},
		{"image/png", "photo.pdf", "image"},
		{"IMAGE/HEIC", "", "image"},
		{"audio/mpeg", "voicemail.mp3", "audio"},
		{"video/mp4", "clip.mp4", "video"},
		{"text/plain", "notes.txt", "text"},
		{"text/html", "page.html", "text"},
		{"text/calendar; method=REQUEST", "invite.ics", "calendar"},
		{"application/ics", "invite.ics", "calendar"},
		{"text/vcard", "card.vcf", "contact"},
		{"text/x-vcard", "card.vcf", "contact"},
		{"application/pkcs7-signature", "smime.p7s", "signature"},
		{"application/x-pkcs7-signature", "smime.p7s", "signature"},
		{"application/pgp-signature", "signature.asc", "signature"},
		{"text/csv", "rows.csv", "spreadsheet"},
		{"application/msword", "letter.doc", "document"},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "letter.docx", "document"},
		{"application/vnd.ms-word.document.macroEnabled.12", "letter.docm", "document"},
		{"application/vnd.oasis.opendocument.text", "letter.odt", "document"},
		{"application/rtf", "letter.rtf", "document"},
		{"application/vnd.ms-excel", "sheet.xls", "spreadsheet"},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "sheet.xlsx", "spreadsheet"},
		{"application/vnd.ms-excel.sheet.macroEnabled.12", "sheet.xlsm", "spreadsheet"},
		{"application/vnd.ms-powerpoint", "deck.ppt", "presentation"},
		{"application/vnd.openxmlformats-officedocument.presentationml.presentation", "deck.pptx", "presentation"},
		{"application/vnd.oasis.opendocument.presentation", "deck.odp", "presentation"},
		{"application/zip", "files.zip", "archive"},
		{"application/x-7z-compressed", "files.7z", "archive"},
		{"message/rfc822", "forwarded.eml", "message"},
		{"application/x-419283", "code.419283", "other"},
		{"application/json", "data.pdf", "other"},
		{"application/octet-stream", "statement.PDF", "pdf"},
		{"application/octet-stream; name=sheet.xlsx", "sheet.xlsx", "spreadsheet"},
		{"", "photo.JPEG", "image"},
		{"application/octet-stream", "archive.tar.gz", "archive"},
		{"application/octet-stream", "invite.ics", "calendar"},
		{"application/octet-stream", "forwarded.eml", "message"},
		{"application/octet-stream", "code.419283", "other"},
		{"application/octet-stream", "no-extension", "other"},
		{"application/octet-stream", "trailing.", "other"},
		{"", "", "other"},
		{"pdf", "statement.pdf", "pdf"},
		{"application/x-" + marker.Field("attachmenttype"), marker.Field("attachmentname") + ".pdf", "other"},
		{"application/octet-stream", marker.Field("attachmentname") + "." + marker.Field("attachmentextension"), "other"},
		{"application/octet-stream; name=" + marker.Field("attachmentparameter"), marker.Field("attachmentname") + ".PDF", "pdf"},
	}
	for _, c := range cases {
		got := mail.AttachmentTypeOf(mail.AttachmentMediaOf(c.mediaType, c.filename))
		if got != c.want {
			t.Errorf("AttachmentTypeOf(%q, %q) = %q, want %q", c.mediaType, c.filename, got, c.want)
		}
		if strings.Contains(string(got), marker.FieldPrefix) {
			t.Errorf("AttachmentTypeOf(%q, %q) = %q, which holds marker text", c.mediaType, c.filename, got)
		}
	}
}

// What a message's attachments set. The names keep the order given, the media are a sorted set of
// pairs, the types derived from them a sorted set of words, and the message has attachments exactly
// when it has names and media (ADR-0123).
func TestAMessagesAttachmentsSetItsNamesAndMediaTogether(t *testing.T) {
	var m mail.MessageMetadata
	m.SetAttachments([]mail.AttachmentPart{
		{MediaType: "application/octet-stream", Filename: "sheet.xlsx"},
		{MediaType: "image/png", Filename: "photo.png"},
		{MediaType: "Image/JPEG; name=scan.jpg", Filename: "scan.JPG"},
		{MediaType: "image/png", Filename: "copy.png"},
	})
	want := mail.MessageMetadata{
		HasAttachments:  true,
		AttachmentNames: []string{"sheet.xlsx", "photo.png", "scan.JPG", "copy.png"},
		AttachmentMedia: []mail.AttachmentMedia{
			{MediaType: "application/octet-stream", Extension: "xlsx"},
			{MediaType: "image/jpeg", Extension: "jpg"},
			{MediaType: "image/png", Extension: "png"},
		},
	}
	if diff := cmp.Diff(want, m, compare.Options); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]mail.AttachmentType{"image", "spreadsheet"}, mail.AttachmentTypes(m.AttachmentMedia), compare.Options); diff != "" {
		t.Errorf("the types (-want +got):\n%s", diff)
	}
	m.SetAttachments(nil)
	if diff := cmp.Diff(mail.MessageMetadata{}, m, compare.Options); diff != "" {
		t.Errorf("after no attachments (-want +got):\n%s", diff)
	}
}

// ADR-0123's normalization. A media type keeps only its lowercase type and subtype, and only when
// each is an RFC 6838 restricted name of at most 127 characters, and an extension only when it is
// one to 16 ASCII letters and digits, so what is stored is short and inert.
func TestAnAttachmentsMediaIsKeptNormalized(t *testing.T) {
	long := strings.Repeat("a", 127)
	cases := []struct {
		mediaType, filename string
		want                mail.AttachmentMedia
	}{
		{"application/pdf", "statement.pdf", mail.AttachmentMedia{MediaType: "application/pdf", Extension: "pdf"}},
		{` Application/PDF ; name="statement.pdf"`, "Statement.PDF", mail.AttachmentMedia{MediaType: "application/pdf", Extension: "pdf"}},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "a.b.XLSX", mail.AttachmentMedia{MediaType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", Extension: "xlsx"}},
		{"image/svg+xml", "logo.svg", mail.AttachmentMedia{MediaType: "image/svg+xml", Extension: "svg"}},
		{"text/" + long, "notes.txt", mail.AttachmentMedia{MediaType: "text/" + long, Extension: "txt"}},
		{"text/" + long + "a", "notes.txt", mail.AttachmentMedia{Extension: "txt"}},
		{"pdf", "statement", mail.AttachmentMedia{}},
		{"application/", "a.", mail.AttachmentMedia{}},
		{"application/x pdf", "a.p d f", mail.AttachmentMedia{}},
		{"application/-pdf", "a.pdf ", mail.AttachmentMedia{}},
		{"application/p\u00e9", "r\u00e9sum\u00e9.p\u00e9", mail.AttachmentMedia{}},
		{"application/\u212apdf", "a.\u212a", mail.AttachmentMedia{}},
		{"", "a.abcdefghijklmnop", mail.AttachmentMedia{Extension: "abcdefghijklmnop"}},
		{"", "a.abcdefghijklmnopq", mail.AttachmentMedia{}},
		{"", "code.419283", mail.AttachmentMedia{Extension: "419283"}},
	}
	for _, c := range cases {
		if got := mail.AttachmentMediaOf(c.mediaType, c.filename); got != c.want {
			t.Errorf("AttachmentMediaOf(%q, %q) = %+v, want %+v", c.mediaType, c.filename, got, c.want)
		}
	}
}
