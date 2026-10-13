package markdown

import "github.com/ppat/mediated-mailbox-mcp/core/mail"

// Render returns body as the Markdown it is released and scanned as. A body with an HTML part is
// that part's conversion, and one without is its text part in the literal form, so the scan reads
// exactly what a client would receive (ADR-0036, ADR-0100, ADR-0131). It returns an error wrapping
// ErrUnconvertible, and no Markdown, for a body either form refuses, above MaxInputBytes included.
func Render(body mail.MessageBody) (string, error) {
	if body.HTML != "" {
		return Convert(body.HTML)
	}
	return LiteralBody(body.Text)
}
