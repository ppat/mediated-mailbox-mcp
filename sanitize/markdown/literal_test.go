package markdown_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/sanitize/markdown"
)

// The released form of a plain text, its snippet and a filename, written out (ADR-0100). The fence is
// one backtick longer than the longest run in the text, and at least three.
func TestLiteralForms(t *testing.T) {
	for in, want := range map[string]string{
		"":                                  "```\n```",
		"hello":                             "```\nhello\n```",
		"line\n":                            "```\nline\n```",
		"<img src=https://t.example/p.gif>": "```\n<img src=https://t.example/p.gif>\n```",
		"a ``` b":                           "````\na ``` b\n````",
		"`````":                             "``````\n`````\n``````",
		"report_final.pdf":                  "```\nreport_final.pdf\n```",
	} {
		if got := markdown.Literal(in); got != want {
			t.Errorf("Literal(%q) = %q, want %q", in, got, want)
		}
	}
}
