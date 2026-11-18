package markdown

import (
	"errors"
	"strconv"

	"github.com/ppat/mediated-mailbox-mcp/content/markdown/core"
)

// LiteralBody returns the text part of a body with no HTML part as core.Literal does, and refuses, with an
// error wrapping ErrUnconvertible and no Markdown, a text above MaxInputBytes, the bound the
// conversion holds an HTML part to (ADR-0131). The snippet and a filename are not body parts, and
// core.Literal renders them unbounded.
func LiteralBody(text string) (string, error) {
	if len(text) > MaxInputBytes {
		return "", errors.Join(ErrUnconvertible, errors.New("markdown: the text is "+strconv.Itoa(len(text))+
			" bytes, above the limit of "+strconv.Itoa(MaxInputBytes)))
	}
	return core.Literal(text), nil
}
