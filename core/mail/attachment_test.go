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

// ADR-0123's tables. The media type decides, its parameters dropped and its case ignored, the file
// name's extension decides only when the media type is application/octet-stream or missing, and
// anything neither names is other. A media type and a file name holding marker text (ADR-0044) give a
// word holding none of it, so a search of the stored types for marker text finds nothing.
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
		{"application/x-" + marker.Field("attachmenttype"), marker.Field("attachmentname") + ".pdf", "other"},
		{"application/octet-stream", marker.Field("attachmentname") + "." + marker.Field("attachmentextension"), "other"},
		{"application/octet-stream; name=" + marker.Field("attachmentparameter"), marker.Field("attachmentname") + ".PDF", "pdf"},
	}
	for _, c := range cases {
		got := mail.AttachmentTypeOf(c.mediaType, c.filename)
		if got != c.want {
			t.Errorf("AttachmentTypeOf(%q, %q) = %q, want %q", c.mediaType, c.filename, got, c.want)
		}
		if strings.Contains(string(got), marker.FieldPrefix) {
			t.Errorf("AttachmentTypeOf(%q, %q) = %q, which holds marker text", c.mediaType, c.filename, got)
		}
	}
}

// What a message's attachments set. The names keep the order given, the types are a sorted set, and
// the message has attachments exactly when it has names and types (ADR-0123).
func TestAMessagesAttachmentsSetItsNamesAndTypesTogether(t *testing.T) {
	var m mail.MessageMetadata
	m.SetAttachments([]mail.AttachmentPart{
		{MediaType: "application/octet-stream", Filename: "sheet.xlsx"},
		{MediaType: "image/png", Filename: "photo.png"},
		{MediaType: "image/jpeg", Filename: "scan.jpg"},
	})
	want := mail.MessageMetadata{
		HasAttachments:  true,
		AttachmentNames: []string{"sheet.xlsx", "photo.png", "scan.jpg"},
		AttachmentTypes: []mail.AttachmentType{"image", "spreadsheet"},
	}
	if diff := cmp.Diff(want, m, compare.Options); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
	m.SetAttachments(nil)
	if diff := cmp.Diff(mail.MessageMetadata{}, m, compare.Options); diff != "" {
		t.Errorf("after no attachments (-want +got):\n%s", diff)
	}
}
