package mail

import (
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
// SetAttachments, and the model keeps the names and the types and never the media types.
type AttachmentPart struct {
	MediaType string
	Filename  string
}

// SetAttachments sets whether the message has attachments, their names in the order given, and their
// types as a set, each word once and sorted, all from the one list of the message's attachments, so
// the three always describe the same parts (ADR-0123).
func (m *MessageMetadata) SetAttachments(parts []AttachmentPart) {
	m.HasAttachments = len(parts) > 0
	m.AttachmentNames, m.AttachmentTypes = nil, nil
	for _, p := range parts {
		m.AttachmentNames = append(m.AttachmentNames, p.Filename)
		m.AttachmentTypes = append(m.AttachmentTypes, AttachmentTypeOf(p.MediaType, p.Filename))
	}
	slices.Sort(m.AttachmentTypes)
	m.AttachmentTypes = slices.Compact(m.AttachmentTypes)
}

// AttachmentTypeOf returns the type of an attachment with the media type and file name given. The
// media type decides, its parameters dropped and its case ignored, and the file name's last
// extension decides only when the media type is application/octet-stream or missing, which say
// nothing of the content (RFC 2046). Anything neither names is AttachmentOther (ADR-0123).
func AttachmentTypeOf(mediaType, filename string) AttachmentType {
	essence, _, _ := strings.Cut(mediaType, ";")
	essence = strings.ToLower(strings.TrimSpace(essence))
	if essence == "" || essence == "application/octet-stream" {
		return byExtension(filename)
	}
	return byMediaType(essence)
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

// byExtension maps a file name's last extension, its case ignored, to its word.
func byExtension(filename string) AttachmentType {
	dot := strings.LastIndexByte(filename, '.')
	if dot < 0 {
		return AttachmentOther
	}
	switch strings.ToLower(filename[dot+1:]) {
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
