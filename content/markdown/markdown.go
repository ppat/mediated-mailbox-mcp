// Package markdown converts a message body's HTML to the clean Markdown every released body is
// served as, and that the Content Scanner reads (ADR-0036, ADR-0005). The mediator converts at serve
// time and the Backfill Job converts before scanning, so the scanner reads exactly the Markdown a
// client would receive.
//
// The conversion is github.com/JohannesKaufmann/html-to-markdown/v2, configured so that the output
// holds content and links and nothing else (ADR-0074).
//
//   - Images of every kind are dropped, remote and inline data: images alike, which also drops
//     tracking pixels.
//   - Elements that execute, embed, style or collect input are dropped with everything inside them.
//     The library's own defaults drop some of these, and this package drops every one of them
//     itself, so a change to those defaults drops nothing it relies on.
//   - A link keeps the form [label](target) only when its target is an absolute http or https URL
//     with a host, or a mailto URL of bare addresses and nothing else, so a label that disagrees
//     with its target stays visible. A mailto URL with a query, which can add recipients or fill
//     in a message, is not kept. Any other target, javascript:, data:, a relative path or one
//     that does not parse, is removed and the label stays as text.
//   - Headings are ATX, a line starting with # or ##, and bold is **, the forms the scanner reads.
//   - Code is written as ordinary escaped text, never as a code block or a code span, because the
//     library writes code unescaped and a fenced block inside a blockquote closes early.
//   - The library separates adjacent lists with an HTML comment by default. That is switched off.
//
// Convert returns an error wrapping ErrUnconvertible, and no Markdown, for a body it will not
// convert. A caller treats that as a denial. The error names the cause and carries none of the
// body's text.
package markdown

import (
	"errors"
	"net/mail"
	"net/url"
	"strconv"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// MaxInputBytes is the largest body Convert accepts. It is about five times the size senders design
// HTML mail to stay under, and it bounds a conversion's memory, which grows to about 150 times its
// input.
const MaxInputBytes = 512 << 10

// ErrUnconvertible is wrapped by every error Convert returns. A body that cannot be converted is
// never released and never scanned as anything but a denial.
var ErrUnconvertible = errors.New("markdown: the body cannot be converted")

// removed lists the elements dropped with all their content. Images and the elements that carry
// them, elements that run code or embed another document, forms and their controls, and the
// document head.
var removed = []string{
	"#comment", "applet", "area", "audio", "base", "button", "canvas", "datalist", "embed", "fieldset",
	"form", "frame", "frameset", "head", "iframe", "img", "input", "link", "map", "math", "meta",
	"noembed", "noframes", "noscript", "object", "optgroup", "option", "output", "param", "picture",
	"portal", "script", "select", "source", "style", "svg", "template", "textarea", "title", "track",
	"video",
}

// Convert returns body, a message body's HTML, as clean Markdown.
func Convert(body string) (string, error) {
	if len(body) > MaxInputBytes {
		return "", errors.Join(ErrUnconvertible, errors.New("markdown: the body is "+strconv.Itoa(len(body))+
			" bytes, above the limit of "+strconv.Itoa(MaxInputBytes)))
	}
	return guarded(func() (string, error) {
		// The document is parsed here rather than by the library, so a parse error is told apart from
		// a conversion error. x/net/html refuses a document nesting more than 512 open elements.
		doc, err := html.Parse(strings.NewReader(body))
		if err != nil {
			return "", err
		}
		out, err := newConverter().ConvertNode(doc)
		if err != nil {
			return "", err
		}
		return string(out), nil
	})
}

// guarded runs convert and turns a failure or a panic into an error wrapping ErrUnconvertible, with
// no output. The library has panicked on crafted input before, and a panic value can carry body
// text, so the value is never included.
func guarded(convert func() (string, error)) (out string, err error) {
	defer func() {
		if recover() != nil {
			out, err = "", errors.Join(ErrUnconvertible, errors.New("markdown: the converter panicked"))
		}
	}()
	out, err = convert()
	if err != nil {
		return "", errors.Join(ErrUnconvertible, err)
	}
	return out, nil
}

func newConverter() *converter.Converter {
	c := converter.NewConverter(converter.WithPlugins(
		base.NewBasePlugin(),
		commonmark.NewCommonmarkPlugin(
			commonmark.WithHeadingStyle(commonmark.HeadingStyleATX),
			commonmark.WithStrongDelimiter("**"),
			commonmark.WithListEndComment(false),
			commonmark.WithLinkEmptyHrefBehavior(commonmark.LinkBehaviorSkip),
			commonmark.WithLinkEmptyContentBehavior(commonmark.LinkBehaviorSkip),
		),
		table.NewTablePlugin(),
	))
	for _, tag := range removed {
		c.Register.TagType(tag, converter.TagTypeRemove, converter.PriorityEarly)
	}
	c.Register.TagType("td", converter.TagTypeBlock, converter.PriorityEarly)
	c.Register.TagType("th", converter.TagTypeBlock, converter.PriorityEarly)
	c.Register.PreRenderer(dropUnsafeLinkTargets, converter.PriorityEarly)
	c.Register.PreRenderer(flattenCode, converter.PriorityEarly)
	return c
}

// codeElements are the elements the library writes as code, a pre as a fenced block and the rest as
// inline code spans.
var codeElements = map[string]bool{"pre": true, "code": true, "var": true, "samp": true, "kbd": true, "tt": true}

// flattenCode turns every code element into ordinary text, so its content is escaped like any other
// text. Code is the one place the library writes text without escaping it, which leaves markup
// written as text in the body inert only while the fence around it holds. Inside a blockquote the
// library prefixes only the first line of a fenced block, so the fence closes early and the markup on
// the later lines becomes live HTML. A pre becomes a block whose lines are separated by line breaks,
// and every other code element becomes an inline span. The pass also removes the two private
// characters the library uses as markers of its own, so text cannot pose as one.
func flattenCode(_ converter.Context, doc *html.Node) {
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		switch {
		case n.Type == html.ElementNode && codeElements[n.Data]:
			pre := n.Data == "pre"
			n.Data, n.DataAtom, n.Attr = "span", atom.Span, nil
			if pre {
				n.Data, n.DataAtom = "div", atom.Div
				breakLines(n)
			}
		case n.Type == html.TextNode:
			n.Data = strings.Map(dropMarker, n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
}

// breakLines replaces every line break in the text under n with a br element.
func breakLines(n *html.Node) {
	for child := n.FirstChild; child != nil; {
		next := child.NextSibling
		if child.Type == html.TextNode && strings.ContainsAny(child.Data, "\r\n") {
			lines := strings.Split(strings.ReplaceAll(child.Data, "\r\n", "\n"), "\n")
			for i, line := range lines {
				if i > 0 {
					n.InsertBefore(&html.Node{Type: html.ElementNode, Data: "br", DataAtom: atom.Br}, child)
				}
				if line != "" {
					n.InsertBefore(&html.Node{Type: html.TextNode, Data: line}, child)
				}
			}
			n.RemoveChild(child)
		} else if child.Type == html.ElementNode {
			breakLines(child)
		}
		child = next
	}
}

// dropMarker removes the library's escaping marker, U+0007, and its code block line break marker,
// U+F002, from body text.
func dropMarker(r rune) rune {
	if r == '\a' || r == '\uf002' {
		return -1
	}
	return r
}

// dropUnsafeLinkTargets empties every link target kept refuses, so the link renders as its label
// alone.
func dropUnsafeLinkTargets(_ converter.Context, doc *html.Node) {
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for i, a := range n.Attr {
				if a.Namespace == "" && a.Key == "href" && !kept(a.Val) {
					n.Attr[i].Val = ""
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
}

// kept reports whether a link keeps target, an absolute http or https URL with a host, or a mailto
// URL of bare addresses and nothing else, with no query and no fragment.
func kept(target string) bool {
	u, err := url.Parse(strings.TrimSpace(target))
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return u.Host != ""
	case "mailto":
		return u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && addresses(u.Opaque)
	default:
		return false
	}
}

// addresses reports whether to, the recipients of a mailto URL, names at least one recipient and
// each is a bare address such as someone@shop.example. A mailto URL with no recipient, or with a
// display name or anything else the mail address syntax refuses, carries no address to keep.
func addresses(to string) bool {
	if to == "" {
		return false
	}
	for _, r := range strings.Split(to, ",") {
		addr, err := url.PathUnescape(r)
		if err != nil {
			return false
		}
		if a, err := mail.ParseAddress(addr); err != nil || a.Name != "" || a.Address != addr {
			return false
		}
	}
	return true
}
