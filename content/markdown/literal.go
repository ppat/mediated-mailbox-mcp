package markdown

import "strings"

// Literal returns text, message text that has no HTML form, as one CommonMark fenced code block that
// shows it exactly (ADR-0100). That is the text part of a body with no HTML part, a snippet, or an
// attachment's filename. The fence is a run of backticks longer than the longest run of backticks in
// the text, and at least three, with no info string, so no line of the text can close the block and a
// Markdown reader sees no link, image, heading or raw HTML in it. The text inside the block is the
// text as it arrived, so a URL in it is unchanged.
func Literal(text string) string {
	longest, run := 0, 0
	for _, r := range text {
		if r != '`' {
			run = 0
			continue
		}
		run++
		longest = max(longest, run)
	}
	fence := strings.Repeat("`", max(3, longest+1))
	var b strings.Builder
	b.Grow(len(text) + 2*len(fence) + 2)
	b.WriteString(fence)
	b.WriteByte('\n')
	b.WriteString(text)
	if text != "" && !strings.HasSuffix(text, "\n") {
		b.WriteByte('\n')
	}
	b.WriteString(fence)
	return b.String()
}
