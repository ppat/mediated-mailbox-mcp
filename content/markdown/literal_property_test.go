package markdown_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/content/markdown"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

// pieces are what a generated plain text is made of, each something a Markdown reader would act on
// in plain text, beside ordinary words. A run of backticks or tildes could close a fence, and one
// alone on its line closes a fence no longer than it, a tag is raw HTML, the bracket forms are a link
// and a remote image, and the rest are block structure.
var pieces = []string{
	"`", "``", "```", "````", "\n```\n", "\n````\n", "\n`````", "~~~", "<script>alert(1)</script>", "<img src=https://t.example/p.gif>",
	"[the bank](https://evil.example/login)", "![](https://t.example/p.gif)", "<https://t.example/auto>",
	"# heading", "> quoted", "- item", "1. item", "    indented", "\t", "\n", "\r\n", "\r", " ", "&amp;",
	"https://shop.example/a_b?c=d_e", "\\", "*", "_", "|", "<!-- comment -->", "\x00",
}

// plain is one generated text, its pieces drawn on their own (ADR-0069).
type plain struct {
	Pieces []int
	Words  []string
}

func drawPlain(t *rapid.T) plain {
	return plain{
		Pieces: rapid.SliceOfN(rapid.IntRange(0, len(pieces)-1), 0, 24).Draw(t, "pieces"),
		Words:  rapid.SliceOfN(rapid.StringMatching(`[a-z]{0,6}`), 0, 24).Draw(t, "words"),
	}
}

// textOf joins a generated text's pieces, each followed by a word, after the marker text, so the text
// is never empty for the property and the marker shows where the text ends up.
func textOf(p plain) string {
	var b strings.Builder
	b.WriteString(marker.Body("plain"))
	for i, n := range p.Pieces {
		b.WriteString(pieces[n])
		if i < len(p.Words) {
			b.WriteString(p.Words[i])
		}
	}
	return b.String()
}

// A postcondition over every generated text, of ADR-0100's rule that message text with no HTML form
// is released as one fenced code block that shows it exactly. A separate CommonMark parser reads the
// released form as any Markdown reader would. The document must be one fenced code block with no
// info string, so it holds no link, image, heading or raw HTML, and the block's lines must be the
// text itself, ending in a line break whether or not the text did.
func TestLiteralTextIsOneCodeBlockShowingIt(t *testing.T) {
	property.Check(t, drawPlain, func(t rapid.TB, p plain) {
		in := textOf(p)
		out := markdown.Literal(in)
		src := []byte(out)
		doc := goldmark.New().Parser().Parse(text.NewReader(src))
		block, ok := doc.FirstChild().(*ast.FencedCodeBlock)
		if doc.ChildCount() != 1 || !ok {
			t.Fatalf("%q was released as %d blocks, the first a %s, not one fenced code block:\n%s", in, doc.ChildCount(), doc.FirstChild().Kind(), out)
		}
		if block.Info != nil {
			t.Fatalf("%q was released with the info string %q:\n%s", in, block.Info.Value(src), out)
		}
		var shown strings.Builder
		lines := block.Lines()
		for i := range lines.Len() {
			seg := lines.At(i)
			shown.Write(seg.Value(src))
		}
		want := in
		if !strings.HasSuffix(want, "\n") {
			want += "\n"
		}
		if shown.String() != want {
			t.Fatalf("the code block shows %q, want %q:\n%s", shown.String(), want, out)
		}
	})
}

// closingRun matches a line holding nothing but a run of three or more backticks, which closes a
// fence of three backticks, and of its own length.
var closingRun = regexp.MustCompile("(?m)^```+[ \\t]*\\r?$")

// fenceMix classifies a generated text by whether a line of it is only a run of three or more
// backticks, the text a fence of three would not survive, or it holds such a run among other text.
func fenceMix(p plain) string {
	text := textOf(p)
	switch {
	case closingRun.MatchString(text):
		return "a line that is only a run of three or more backticks"
	case strings.Contains(text, "```"):
		return "a run of three or more backticks among other text"
	default:
		return "shorter runs only"
	}
}

// The generator report for the property above. A fence of three backticks fails only on a line that
// is only such a run, so that kind is required at a share the gating count reaches many times over.
func TestLiteralTextIsOneCodeBlockShowingItMix(t *testing.T) {
	property.Report(t, drawPlain, fenceMix, map[string]float64{
		"a line that is only a run of three or more backticks": 0.10,
		"a run of three or more backticks among other text":    0.02,
		"shorter runs only": 0.02,
	})
}
