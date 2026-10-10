package mail

import (
	"cmp"
	"slices"
	"strings"
)

// AttachmentType is a word of the closed vocabulary an attachment's type is told in (ADR-0123). It is
// shown in every sensitivity state, while the attachment's file name follows the body (ADR-0001), so
// it is never text the message carries. Its values are the constants below, and the mapping,
// AttachmentTypeOf, returns only those.
type AttachmentType string

// The vocabulary, every word an attachment's type can be.
const (
	AttachmentPDF          AttachmentType = "pdf"
	AttachmentImage        AttachmentType = "image"
	AttachmentAudio        AttachmentType = "audio"
	AttachmentVideo        AttachmentType = "video"
	AttachmentText         AttachmentType = "text"
	AttachmentCalendar     AttachmentType = "calendar"
	AttachmentContact      AttachmentType = "contact"
	AttachmentDocument     AttachmentType = "document"
	AttachmentSpreadsheet  AttachmentType = "spreadsheet"
	AttachmentPresentation AttachmentType = "presentation"
	AttachmentArchive      AttachmentType = "archive"
	AttachmentMessage      AttachmentType = "message"
	AttachmentSignature    AttachmentType = "signature"
	AttachmentOther        AttachmentType = "other"
)

// AttachmentPart is one attachment as a provider describes it, its media type as the part's
// Content-Type gives it and its file name. An adapter hands a message's attachments to
// SetAttachments, which keeps the names and the normalized media.
type AttachmentPart struct {
	MediaType string
	Filename  string
}

// AttachmentMedia is what the model keeps of an attachment to derive its type from, its media type
// and its file name's extension, each normalized by AttachmentMediaOf (ADR-0123). It is what ingest
// stores, and never what a client is served, since a client sees only the type the mapping derives.
type AttachmentMedia struct {
	// MediaType is lowercase, without parameters, a type and a subtype of RFC 6838's restricted-name
	// characters, each at most 127 long, or empty.
	MediaType string
	// Extension is the text after the file name's last dot, lowercase, one to 16 ASCII letters and
	// digits, or empty.
	Extension string
}

// The longest name RFC 6838 allows a type or a subtype, and the longest extension kept. No standard
// bounds an extension, and 16 is twice the longest one the mapping names (ADR-0123).
const (
	mediaNameMax = 127
	extensionMax = 16
)

// AttachmentMediaOf returns the normalized media of an attachment with the media type and file name
// given. A media type that is not a type and a subtype of RFC 6838's restricted names is empty, and
// so is an extension that is not one to 16 ASCII letters and digits, so only short, inert text the
// sender wrote is kept.
func AttachmentMediaOf(mediaType, filename string) AttachmentMedia {
	var out AttachmentMedia
	essence, _, _ := strings.Cut(mediaType, ";")
	essence = asciiLower(strings.TrimSpace(essence))
	if typ, sub, ok := strings.Cut(essence, "/"); ok && restrictedName(typ) && restrictedName(sub) {
		out.MediaType = essence
	}
	if dot := strings.LastIndexByte(filename, '.'); dot >= 0 {
		if ext := asciiLower(filename[dot+1:]); len(ext) <= extensionMax && strings.Trim(ext, "abcdefghijklmnopqrstuvwxyz0123456789") == "" {
			out.Extension = ext
		}
	}
	return out
}

// restrictedName reports whether s is an RFC 6838 restricted name in lower case, a letter or digit
// followed by up to 126 letters, digits and the characters ! # $ & - ^ _ . +.
func restrictedName(s string) bool {
	if s == "" || len(s) > mediaNameMax || !isLowerAlnum(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !isLowerAlnum(s[i]) && !strings.ContainsRune("!#$&-^_.+", rune(s[i])) {
			return false
		}
	}
	return true
}

func isLowerAlnum(c byte) bool { return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' }

// asciiLower lowercases the ASCII letters of s and leaves every other byte as it is, so a letter
// outside ASCII never becomes one inside it.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// SetAttachments sets whether the message has attachments, their names in the order given, and their
// normalized media as a set, each pair once and sorted, all from the one list of the message's
// attachments, so the three always describe the same parts (ADR-0123).
func (m *MessageMetadata) SetAttachments(parts []AttachmentPart) {
	m.HasAttachments = len(parts) > 0
	m.AttachmentNames, m.AttachmentMedia = nil, nil
	for _, p := range parts {
		m.AttachmentNames = append(m.AttachmentNames, p.Filename)
		m.AttachmentMedia = append(m.AttachmentMedia, AttachmentMediaOf(p.MediaType, p.Filename))
	}
	slices.SortFunc(m.AttachmentMedia, compareMedia)
	m.AttachmentMedia = slices.Compact(m.AttachmentMedia)
}

func compareMedia(a, b AttachmentMedia) int {
	return cmp.Or(strings.Compare(a.MediaType, b.MediaType), strings.Compare(a.Extension, b.Extension))
}

// AttachmentTypes returns the types of a message's attachments from their media, each word once and
// sorted, which is how every client is served them (ADR-0123).
func AttachmentTypes(media []AttachmentMedia) []AttachmentType {
	out := make([]AttachmentType, 0, len(media))
	for _, m := range media {
		out = append(out, AttachmentTypeOf(m))
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// AttachmentTypeOf returns the type of an attachment with the media given. The media type decides,
// and the extension decides only when the media type is application/octet-stream or empty, which say
// nothing of the content (RFC 2046). Anything neither names is AttachmentOther (ADR-0123). It reads
// the media as normalized, and returns only the vocabulary's words whatever it is given.
func AttachmentTypeOf(m AttachmentMedia) AttachmentType {
	if m.MediaType == "" || m.MediaType == "application/octet-stream" {
		return byExtension(m.Extension)
	}
	return byMediaType(m.MediaType)
}

// byMediaType maps a media type, lowercase and without parameters, to its word.
func byMediaType(t string) AttachmentType {
	switch t {
	case "application/pdf":
		return AttachmentPDF
	case "text/calendar", "application/ics":
		return AttachmentCalendar
	case "text/vcard", "text/x-vcard", "text/directory":
		return AttachmentContact
	case "application/pkcs7-signature", "application/x-pkcs7-signature", "application/pgp-signature":
		return AttachmentSignature
	case "text/csv", "text/tab-separated-values":
		return AttachmentSpreadsheet
	case "application/msword", "application/rtf", "text/rtf", "application/vnd.oasis.opendocument.text",
		"application/vnd.apple.pages":
		return AttachmentDocument
	case "application/vnd.ms-excel", "application/vnd.oasis.opendocument.spreadsheet", "application/vnd.apple.numbers":
		return AttachmentSpreadsheet
	case "application/vnd.ms-powerpoint", "application/vnd.oasis.opendocument.presentation", "application/vnd.apple.keynote":
		return AttachmentPresentation
	case "application/zip", "application/x-zip-compressed", "application/gzip", "application/x-gzip", "application/x-tar",
		"application/x-bzip2", "application/x-7z-compressed", "application/vnd.rar", "application/x-rar-compressed":
		return AttachmentArchive
	}
	switch {
	case hasAnyPrefix(t, "application/vnd.openxmlformats-officedocument.wordprocessingml.", "application/vnd.ms-word."):
		return AttachmentDocument
	case hasAnyPrefix(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.", "application/vnd.ms-excel."):
		return AttachmentSpreadsheet
	case hasAnyPrefix(t, "application/vnd.openxmlformats-officedocument.presentationml.", "application/vnd.ms-powerpoint."):
		return AttachmentPresentation
	case strings.HasPrefix(t, "image/"):
		return AttachmentImage
	case strings.HasPrefix(t, "audio/"):
		return AttachmentAudio
	case strings.HasPrefix(t, "video/"):
		return AttachmentVideo
	case strings.HasPrefix(t, "message/"):
		return AttachmentMessage
	case strings.HasPrefix(t, "text/"):
		return AttachmentText
	}
	return AttachmentOther
}

// byExtension maps an extension, lowercase, to its word.
func byExtension(extension string) AttachmentType {
	switch extension {
	case "pdf":
		return AttachmentPDF
	case "jpg", "jpeg", "png", "gif", "bmp", "tif", "tiff", "webp", "heic", "heif", "svg":
		return AttachmentImage
	case "mp3", "m4a", "wav", "ogg", "oga", "flac", "aac", "amr":
		return AttachmentAudio
	case "mp4", "m4v", "mov", "avi", "mkv", "webm", "3gp":
		return AttachmentVideo
	case "txt", "log", "md":
		return AttachmentText
	case "ics", "vcs":
		return AttachmentCalendar
	case "vcf":
		return AttachmentContact
	case "p7s", "sig":
		return AttachmentSignature
	case "doc", "docx", "docm", "dot", "dotx", "odt", "rtf", "pages":
		return AttachmentDocument
	case "xls", "xlsx", "xlsm", "xlt", "xltx", "ods", "csv", "tsv", "numbers":
		return AttachmentSpreadsheet
	case "ppt", "pptx", "pptm", "pps", "ppsx", "odp", "key":
		return AttachmentPresentation
	case "zip", "gz", "tgz", "tar", "bz2", "xz", "7z", "rar":
		return AttachmentArchive
	case "eml", "msg":
		return AttachmentMessage
	}
	return AttachmentOther
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	return slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(s, p) })
}
