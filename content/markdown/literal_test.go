package markdown_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/content/markdown"
	"github.com/ppat/mediated-mailbox-mcp/content/markdown/core"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
)

// A text part at the size limit is rendered as Literal renders it, and one a byte over is refused
// with no Markdown, as an HTML part over the limit is (ADR-0131). The limit is the 512 KiB the
// operator set, written out in markdown_test.go rather than read from the package.
func TestLiteralBodyHoldsTheSizeLimit(t *testing.T) {
	at := marker.Body("shown") + strings.Repeat("a", limit-len(marker.Body("shown")))
	got, err := markdown.LiteralBody(at)
	if err != nil || got != core.Literal(at) {
		t.Errorf("LiteralBody of a text at the limit returned an error %v or other Markdown than Literal's", err)
	}
	got, err = markdown.LiteralBody(at + "a")
	if !errors.Is(err, markdown.ErrUnconvertible) || got != "" {
		t.Errorf("LiteralBody of a text a byte over the limit returned %d bytes and %v, want none and an error wrapping %v",
			len(got), err, markdown.ErrUnconvertible)
	}
	if err != nil && strings.Contains(err.Error(), marker.BodyPrefix) {
		t.Errorf("LiteralBody's refusal carries body text: %v", err)
	}
	// The snippet and a filename are metadata, which Literal renders whatever their size.
	if past := core.Literal(at + "a"); !strings.Contains(past, at+"a") {
		t.Errorf("Literal of a text past the limit did not render it whole")
	}
}

// A body with an HTML part renders as its conversion, and one without as its text in the literal
// form, each refused above the size limit (ADR-0131).
func TestRenderTakesTheReleasedForm(t *testing.T) {
	html := "<p>" + marker.Body("shown") + "</p>"
	converted, err := markdown.Convert(html)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	cases := []struct {
		name    string
		body    mail.MessageBody
		want    string
		refused bool
	}{
		{"an HTML part", mail.MessageBody{HTML: html}, converted, false},
		{"both parts", mail.MessageBody{HTML: html, Text: marker.Body("hidden")}, converted, false},
		{"a text part", mail.MessageBody{Text: marker.Body("shown")}, core.Literal(marker.Body("shown")), false},
		{"no part", mail.MessageBody{}, core.Literal(""), false},
		{"an HTML part over the limit", mail.MessageBody{HTML: "<p>" + strings.Repeat("a", limit) + "</p>"}, "", true},
		{"a text part over the limit", mail.MessageBody{Text: strings.Repeat("a", limit+1)}, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := markdown.Render(c.body)
			if got != c.want || errors.Is(err, markdown.ErrUnconvertible) != c.refused {
				t.Errorf("Render returned %d bytes and %v, want %d bytes, refused %v", len(got), err, len(c.want), c.refused)
			}
		})
	}
}
