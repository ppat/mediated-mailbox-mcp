// Package marker holds the designed marker text that synthetic fixtures and generated mail values
// carry (ADR-0044).
//
// It sits in the shared pure library rather than in testsupport because the mediator's readiness
// probe builds its known-sensitive fixture from it in non-test code, and non-test code never imports
// testsupport. The import list for non-test code enforces that.
//
// Body markers and field markers begin with different prefixes, and neither prefix contains the
// other. A search of any output for BodyPrefix finds leaked body text and nothing else, although
// the field markers in subjects and display names are visible by design. A field marker shaped like
// markup lets a rendering test assert the marker arrived as text and nothing else.
//
// A marker built from one tag contains the marker built from any tag that is a prefix of it, so
// Body("bank") is found inside Body("banknote"). Callers that need to tell markers of one kind apart
// choose tags where none begins another.
//
// A tag is lowercase letters only. A run of digits would read as a one-time code that subject
// masking replaces, and a marker that masking can remove is no longer searchable. A hyphen would let
// a tag spell the other kind's prefix, so a body marker could carry a field prefix or the reverse.
package marker

// BodyPrefix begins every body marker.
const BodyPrefix = "mmbodymarker-"

// FieldPrefix begins every field marker.
const FieldPrefix = "mmfieldmarker-"

// Body returns the body marker for tag. It panics when tag is anything but lowercase letters.
func Body(tag string) string {
	return BodyPrefix + checked(tag)
}

// Field returns the field marker for tag, for metadata fields such as subjects and display names.
// It panics when tag is anything but lowercase letters.
func Field(tag string) string {
	return FieldPrefix + checked(tag)
}

// MarkupField returns the field marker for tag inside a script element. A surface showing it inert
// shows the whole string, angle brackets included, as text. The inner marker alone is no evidence,
// because the text content of a parsed script element still contains it. It panics when tag is
// anything but lowercase letters.
func MarkupField(tag string) string {
	return "<script>" + Field(tag) + "</script>"
}

func checked(tag string) string {
	if tag == "" {
		panic("marker: empty tag")
	}
	for _, r := range tag {
		if r < 'a' || r > 'z' {
			panic("marker: tag " + tag + " is not lowercase letters only")
		}
	}
	return tag
}
