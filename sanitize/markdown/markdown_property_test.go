package markdown_test

import (
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/sanitize/markdown"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

// part is one element of a generated body. Kind picks the element's shape and Choice the link or
// image target or the dropped element's name. Nest lists the containers the element sits in, from the
// innermost out, each a blockquote, a bulleted list item or a numbered list item.
type part struct {
	Kind   int
	Choice int
	Words  string
	Nest   []int
}

// args are the parts of one generated body, each drawn on its own (ADR-0069). Every body opens with a
// paragraph carrying the shown marker, so the output is never empty for a correct conversion.
type args struct {
	Lead  string
	Parts []part
}

// targets are the link and image targets a part draws from. The first five are targets a link keeps.
var targets = []string{
	"https://shop.example/a?b=c", "http://shop.example/x", "HTTPS://Shop.Example/Y",
	"mailto:someone@shop.example", "  MAILTO:someone@shop.example ", "mailto:someone@shop.example?subject=Hi",
	"mailto:someone@shop.example?cc=other@evil.example",
	"javascript:alert(1)", "JaVaScRiPt:alert(1)", " javascript:alert(1)", "java\tscript:alert(1)",
	"data:text/html;base64,PHNjcmlwdD4=", "data:image/png;base64,iVBORw0KGgo=", "vbscript:msgbox(1)",
	"mailto:?subject=Hi", "file:///etc/passwd", "cid:logo123", "/account", "//evil.example/x", "",
}

func keeps(choice int) bool { return choice < 5 }

// droppedElements are elements a part wraps the dropped marker in.
var droppedElements = []string{
	"script", "style", "noscript", "iframe", "object", "svg", "math", "template", "form", "button",
	"select", "textarea", "video", "audio", "canvas", "title",
}

const (
	kindParagraph = iota
	kindHeading
	kindBold
	kindLink
	kindImage
	kindDropped
	kindMarkupAsText
	kindLayoutTable
	kindComment
	kindPreformatted
	kindInlineCode
	kinds
)

// Containers a part can sit in.
const (
	nestQuote = iota
	nestBullet
	nestNumbered
	nests
)

// markupLines is text that spells markup, entity-escaped as a sender writes it, over several lines.
const markupLines = "\n&lt;script&gt;alert(1)&lt;/script&gt;\n&lt;img src=https://t.example/p.gif&gt;\n"

func draw(t *rapid.T) args {
	words := rapid.StringMatching(`[a-z]{1,8}( [a-z]{1,8}){0,3}`)
	return args{
		Lead: words.Draw(t, "lead"),
		Parts: rapid.SliceOfN(rapid.Custom(func(t *rapid.T) part {
			return part{
				Kind:   rapid.IntRange(0, kinds-1).Draw(t, "kind"),
				Choice: rapid.IntRange(0, len(targets)-1).Draw(t, "choice"),
				Words:  words.Draw(t, "words"),
				Nest:   rapid.SliceOfN(rapid.IntRange(0, nests-1), 0, 3).Draw(t, "nest"),
			}
		}), 0, 12).Draw(t, "parts"),
	}
}

func html(a args) string {
	shown, dropped := marker.Body("shown"), marker.Body("dropped")
	var b strings.Builder
	b.WriteString("<html><body><p>" + shown + " " + a.Lead + "</p>")
	for _, p := range a.Parts {
		for _, n := range slices.Backward(p.Nest) {
			b.WriteString([...]string{"<blockquote>", "<ul><li>", "<ol><li>"}[n])
		}
		switch p.Kind {
		case kindParagraph:
			b.WriteString("<p>" + shown + " " + p.Words + "</p>")
		case kindHeading:
			level := string(rune('1' + p.Choice%2))
			b.WriteString("<h" + level + ">" + shown + " " + p.Words + "</h" + level + ">")
		case kindBold:
			b.WriteString("<p><b>" + shown + "</b> " + p.Words + "</p>")
		case kindLink:
			b.WriteString(`<p><a href="` + targets[p.Choice] + `">` + shown + " " + p.Words + "</a></p>")
		case kindImage:
			b.WriteString(`<img src="` + targets[p.Choice] + `" alt="` + dropped + `">`)
		case kindDropped:
			el := droppedElements[p.Choice%len(droppedElements)]
			b.WriteString("<" + el + ">" + dropped + " " + p.Words + "</" + el + ">")
		case kindMarkupAsText:
			b.WriteString("<p>&lt;script&gt;" + p.Words + "&lt;/script&gt; [x](javascript:alert(1)) ![y](https://t.example/p.gif) &lt;img src=x&gt;</p>")
		case kindLayoutTable:
			b.WriteString(`<table role="presentation"><tr><td>` + shown + "</td><td>" + p.Words + "</td></tr></table>")
		case kindComment:
			b.WriteString("<!-- " + dropped + " -->")
		case kindPreformatted:
			b.WriteString("<pre><code>" + shown + markupLines + p.Words + "</code></pre>")
		case kindInlineCode:
			b.WriteString("<p><code>&lt;script&gt;" + p.Words + "&lt;/script&gt;</code> " + shown + "</p>")
		}
		for _, n := range p.Nest {
			b.WriteString([...]string{"</blockquote>", "</li></ul>", "</li></ol>"}[n])
		}
	}
	b.WriteString("</body></html>")
	return b.String()
}

// A postcondition over every generated body, of ADR-0036's rule that a released body is clean
// Markdown holding content and links and nothing else. A separate CommonMark parser reads the output
// as any Markdown reader would. It must find no image and no HTML, every link it finds must target an
// http or https URL with a host or a mailto URL of bare addresses and nothing else, and it must find one link for every
// kept link the body held, so a conversion that drops every link fails. The shown marker must survive
// and the dropped marker must not, so a conversion that returns nothing fails too.
func TestTheConvertedBodysForm(t *testing.T) {
	property.Check(t, draw, func(t rapid.TB, a args) {
		in := html(a)
		out, err := markdown.Convert(in)
		if err != nil {
			t.Fatalf("%+v: Convert returned %v", a, err)
		}
		if !strings.Contains(out, marker.Body("shown")) || strings.Contains(out, marker.Body("dropped")) {
			t.Fatalf("%+v: the output lost the shown text or kept the dropped text:\n%s", a, out)
		}
		src := []byte(out)
		links := 0
		err = ast.Walk(goldmark.New().Parser().Parse(text.NewReader(src)), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}
			switch n := n.(type) {
			case *ast.Image, *ast.HTMLBlock, *ast.RawHTML:
				t.Fatalf("%+v: the output holds a %s:\n%s", a, n.Kind(), out)
			case *ast.Link:
				links++
				if !keptTarget(string(n.Destination)) {
					t.Fatalf("%+v: the output links to %q:\n%s", a, n.Destination, out)
				}
			case *ast.AutoLink:
				t.Fatalf("%+v: the output holds an autolink to %q:\n%s", a, n.URL(src), out)
			}
			return ast.WalkContinue, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if want := keptLinks(a); links != want {
			t.Fatalf("%+v: the output holds %d links, want %d:\n%s", a, links, want, out)
		}
	})
}

// keptTarget reports whether a link the parser found targets an http or https URL with a host, or a
// mailto URL naming an address and carrying no query.
func keptTarget(dest string) bool {
	u, err := url.Parse(dest)
	switch {
	case err != nil:
		return false
	case u.Scheme == "http" || u.Scheme == "https":
		return u.Host != ""
	case u.Scheme == "mailto":
		return strings.Contains(u.Opaque, "@") && u.RawQuery == "" && !u.ForceQuery
	default:
		return false
	}
}

func keptLinks(a args) int {
	n := 0
	for _, p := range a.Parts {
		if p.Kind == kindLink && keeps(p.Choice) {
			n++
		}
	}
	return n
}

func linkMix(a args) string {
	var kepts, others bool
	for _, p := range a.Parts {
		if p.Kind == kindLink {
			kepts = kepts || keeps(p.Choice)
			others = others || !keeps(p.Choice)
		}
	}
	switch {
	case kepts && others:
		return "kept and other links"
	case kepts:
		return "kept links only"
	case others:
		return "other links only"
	default:
		return "no links"
	}
}

// The generator report for the property above. Across thirty seeds at 200 cases the rarest kind, kept
// and other links together, never fell below 2.5 percent.
func TestTheConvertedBodysFormMix(t *testing.T) {
	property.Report(t, draw, linkMix, map[string]float64{
		"kept and other links": 0.01,
		"kept links only":      0.01,
		"other links only":     0.01,
		"no links":             0.01,
	})
}

func nesting(a args) string {
	var quotedPre, listedPre, nested bool
	for _, p := range a.Parts {
		if len(p.Nest) == 0 {
			continue
		}
		nested = true
		if p.Kind == kindPreformatted {
			quotedPre = quotedPre || slices.Contains(p.Nest, nestQuote)
			listedPre = listedPre || slices.Contains(p.Nest, nestBullet) || slices.Contains(p.Nest, nestNumbered)
		}
	}
	switch {
	case quotedPre:
		return "preformatted text in a blockquote"
	case listedPre:
		return "preformatted text in a list only"
	case nested:
		return "other nesting"
	default:
		return "no nesting"
	}
}

// The generator report of the same property's nesting, which must reach preformatted text inside a
// blockquote, the shape in which a fenced block would close early. Across thirty seeds at 200 cases the
// rarest kind, preformatted text in a list only, never fell below 2 percent.
func TestTheConvertedBodysFormNesting(t *testing.T) {
	property.Report(t, draw, nesting, map[string]float64{
		"preformatted text in a blockquote": 0.01,
		"preformatted text in a list only":  0.01,
		"other nesting":                     0.01,
		"no nesting":                        0.01,
	})
}
