package markdown_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/marker"
	"github.com/ppat/mediated-mailbox-mcp/sanitize/markdown"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// outcome is what a test can observe of a conversion.
type outcome struct {
	Markdown      string
	Unconvertible bool
}

func convert(html string) outcome {
	out, err := markdown.Convert(html)
	return outcome{Markdown: out, Unconvertible: errors.Is(err, markdown.ErrUnconvertible)}
}

func converted(md string) outcome { return outcome{Markdown: md} }

var unconvertible = outcome{Unconvertible: true}

// The paths production never exercises, tested first (ADR-0042). A body too deeply nested to parse
// and a body over the size limit are refused with ErrUnconvertible and no Markdown, and the error
// carries none of the body's text.
func TestFailClosed(t *testing.T) {
	shown := marker.Body("shown")
	cases := []struct {
		name, html string
	}{
		{"600 nested elements", strings.Repeat("<div>", 600) + shown + strings.Repeat("</div>", 600)},
		{"600 nested tables", strings.Repeat("<table><tr><td>", 600) + shown + strings.Repeat("</td></tr></table>", 600)},
		{"a body one byte over the size limit", sized(limit + 1)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(unconvertible, convert(c.html), compare.Options); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
			if _, err := markdown.Convert(c.html); err == nil || strings.Contains(err.Error(), marker.BodyPrefix) {
				t.Errorf("Convert returned %v, want an error carrying no body text", err)
			}
		})
	}
}

// limit is the size limit the operator set, 512 KiB, written out here rather than read from the
// package.
const limit = 524288

// sized returns an HTML body of exactly n bytes holding the shown marker.
func sized(n int) string {
	body := "<p>" + marker.Body("shown") + "</p><p>"
	return body + strings.Repeat("a", n-len(body)-len("</p>")) + "</p>"
}

// A body exactly at the size limit converts.
func TestABodyAtTheSizeLimitConverts(t *testing.T) {
	out, err := markdown.Convert(sized(limit))
	if err != nil || !strings.Contains(out, marker.Body("shown")) {
		t.Errorf("Convert of a body at the limit returned %v and Markdown without the shown marker", err)
	}
}

// The S3 part of docs/VERIFICATIONS.md's row for serving a body with raw HTML, a link whose label
// disagrees with its target, and a remote image (ADR-0036). The client's Markdown holds the content,
// the link in [label](target) form so the disagreement shows, and no image and no markup.
func TestRawHTMLALinkAndARemoteImage(t *testing.T) {
	html := `<html><head><title>` + marker.Body("dropped") + `</title><style>p{color:red}</style></head><body>` +
		`<h1>Statement</h1><p>` + marker.Body("shown") + ` is <b>due</b>.</p>` +
		`<p><a href="https://evil.example/steal">https://bank.example/login</a></p>` +
		`<img src="https://track.example/open.gif?id=42" alt="` + marker.Body("dropped") + `">` +
		`<script>` + marker.Body("dropped") + `</script></body></html>`
	want := "# Statement\n\n" + marker.Body("shown") + " is **due**.\n\n[https://bank.example/login](https://evil.example/steal)"
	if diff := cmp.Diff(converted(want), convert(html), compare.Options); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

// Every image is dropped, remote, inline and embedded alike, together with its alternative text, and a
// link holding only an image goes with it.
func TestImagesAreDropped(t *testing.T) {
	dropped := marker.Body("dropped")
	cases := []struct {
		name, html string
	}{
		{"a remote image", `<p>kept</p><img src="https://track.example/p.gif" alt="` + dropped + `">`},
		{"an inline data: image", `<p>kept</p><img src="data:image/png;base64,iVBORw0KGgo=" alt="` + dropped + `">`},
		{"an embedded cid: image", `<p>kept</p><img src="cid:logo123" alt="` + dropped + `">`},
		{"a picture with sources", `<picture><source srcset="https://i.example/a.webp"><img src="https://i.example/a.png" alt="` + dropped + `"></picture><p>kept</p>`},
		{"a link holding only an image", `<a href="https://shop.example/"><img src="https://i.example/a.png" alt="` + dropped + `"></a><p>kept</p>`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(converted("kept"), convert(c.html), compare.Options); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

// Elements that execute, embed another document, style, or collect input are dropped with everything
// inside them. The check is on the output, which keeps the text beside each element and none of the
// text inside it.
func TestActiveContentIsDropped(t *testing.T) {
	dropped := marker.Body("dropped")
	cases := []struct {
		name, html string
	}{
		{"a script", `<script>` + dropped + `</script>`},
		{"a style", `<style>` + dropped + `{}</style>`},
		{"a noscript fallback", `<noscript>` + dropped + `</noscript>`},
		{"an iframe", `<iframe src="https://x.example/">` + dropped + `</iframe>`},
		{"an iframe with an HTML data: source", `<iframe src="data:text/html,&lt;h1&gt;` + dropped + `&lt;/h1&gt;"></iframe>`},
		{"an object and its fallback", `<object data="https://x.example/a.swf">` + dropped + `</object>`},
		{"an embed", `<embed src="https://x.example/a.swf" title="` + dropped + `">`},
		{"an svg and its text", `<svg><text>` + dropped + `</text><a href="https://x.example/">` + dropped + `</a></svg>`},
		{"a math element", `<math><mi>` + dropped + `</mi></math>`},
		{"a template", `<template><p>` + dropped + `</p></template>`},
		{"a form and its controls", `<form action="https://x.example/"><label>` + dropped + `</label><button>` + dropped + `</button></form>`},
		{"a select and its options", `<select><option>` + dropped + `</option></select>`},
		{"a text area", `<textarea>` + dropped + `</textarea>`},
		{"a video and its fallback", `<video src="https://v.example/a.mp4">` + dropped + `</video>`},
		{"an audio and its fallback", `<audio src="https://v.example/a.mp3">` + dropped + `</audio>`},
		{"a canvas and its fallback", `<canvas>` + dropped + `</canvas>`},
		{"an image map", `<map><area href="https://x.example/" alt="` + dropped + `"></map>`},
		{"a comment", `<!-- ` + dropped + ` -->`},
		{"a title in the head", `<head><title>` + dropped + `</title></head>`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			html := "<p>before</p>" + c.html + "<p>after</p>"
			if diff := cmp.Diff(converted("before\n\nafter"), convert(html), compare.Options); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

// A link keeps [label](target) only for an absolute http or https target with a host, or a mailto
// target of bare addresses and nothing else. Any other target is removed and the label stays as
// text, so nothing in the output can run or open a local resource.
func TestLinkTargets(t *testing.T) {
	cases := []struct {
		name, href, want string
	}{
		{"https", "https://shop.example/a?b=c", "[label](https://shop.example/a?b=c)"},
		{"http", "http://shop.example/a", "[label](http://shop.example/a)"},
		{"a scheme in capitals", "HTTPS://Shop.Example/A", "[label](https://Shop.Example/A)"},
		{"surrounding space", "  https://shop.example/a  ", "[label](https://shop.example/a)"},
		{"a space in the path", "https://shop.example/a b", "[label](https://shop.example/a%20b)"},
		{"a target trying to close the link", "https://ok.example/a)](https://evil.example/b", "[label](https://ok.example/a%29%5D%28https://evil.example/b)"},
		{"javascript", "javascript:alert(document.domain)", "label"},
		{"javascript in mixed case", "JaVaScRiPt:alert(1)", "label"},
		{"javascript after space", " javascript:alert(1)", "label"},
		{"javascript split by a tab", "java\tscript:alert(1)", "label"},
		{"javascript split by an encoded newline", "java&#10;script:alert(1)", "label"},
		{"an HTML data: target", "data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==", "label"},
		{"vbscript", "vbscript:msgbox(1)", "label"},
		{"a local file", "file:///etc/passwd", "label"},
		{"mailto", "mailto:someone@shop.example", "[label](mailto:someone@shop.example)"},
		{"mailto in capitals with surrounding space", "  MAILTO:someone@shop.example ", "[label](mailto:someone@shop.example)"},
		{"mailto with two recipients", "mailto:a@shop.example,b@shop.example", "[label](mailto:a@shop.example,b@shop.example)"},
		{"mailto with a subject", "mailto:a@shop.example?subject=Hi", "label"},
		{"mailto with a copy recipient and a body", "mailto:a@shop.example?cc=evil@evil.example&body=secret", "label"},
		{"mailto with a blind copy recipient", "mailto:a@shop.example?bcc=evil@evil.example", "label"},
		{"mailto with an empty query", "mailto:a@shop.example?", "label"},
		{"mailto with a fragment", "mailto:a@shop.example#x", "label"},
		{"mailto with no recipient", "mailto:?subject=Hi", "label"},
		{"mailto with a display name", "mailto:Someone%20%3Csomeone@shop.example%3E", "label"},
		{"mailto with no domain", "mailto:someone", "label"},
		{"mailto with an empty recipient", "mailto:a@shop.example,", "label"},
		{"a relative path", "/account/settings", "label"},
		{"a target with no scheme", "//evil.example/steal", "label"},
		{"an http target with no host", "https:evil", "label"},
		{"an empty target", "", "label"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			html := `<p><a href="` + c.href + `">label</a></p>`
			if diff := cmp.Diff(converted(c.want), convert(html), compare.Options); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

// The structure the Content Scanner reads (ADR-0005). Headings are ATX, bold is **, a data table
// becomes | rows, and the cells of a layout table stand on lines of their own rather than running
// together, so a code in a cell is a run the scanner can find.
func TestStructureTheScannerReads(t *testing.T) {
	cases := []struct {
		name, html, want string
	}{
		{"headings", `<h1>First</h1><h2>Second</h2><h3>Third</h3>`, "# First\n\n## Second\n\n### Third"},
		{"bold", `<p><b>b</b> and <strong>strong</strong></p>`, "**b** and **strong**"},
		{
			"a data table", `<table><tr><th>Item</th><th>Price</th></tr><tr><td>Lamp</td><td>42.00</td></tr></table>`,
			"| Item | Price |\n|------|-------|\n| Lamp | 42.00 |",
		},
		{"a layout table", `<table role="presentation"><tr><td>Your code</td><td>482913</td></tr></table>`, "Your code\n\n482913"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(converted(c.want), convert(c.html), compare.Options); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

// The output carries no markup. Markup written as text stays escaped, an unknown element keeps only
// its text, and two adjacent lists are not separated by an HTML comment. Code, preformatted text
// included, is written as escaped text line by line, so no fence exists to close early inside a
// blockquote or a list, and text cannot pose as the library's own marker characters.
func TestNoMarkupInTheOutput(t *testing.T) {
	cases := []struct {
		name, html, want string
	}{
		{
			"markup written as text", `<p>&lt;script&gt;alert(1)&lt;/script&gt; &lt;img src=x onerror=alert(1)&gt;</p>`,
			"&lt;script&gt;alert(1)&lt;/script&gt; &lt;img src=x onerror=alert(1)&gt;",
		},
		{"an unknown element", `<p>a <x-custom onclick="alert(1)">custom</x-custom> element</p>`, "a custom element"},
		{"two adjacent lists", `<ol><li>one</li></ol><ol><li>two</li></ol>`, "1. one\n\n1. two"},
		{
			"preformatted markup in a blockquote", "<blockquote><pre>a\n&lt;img src=https://t.example/p.gif&gt;</pre></blockquote>",
			"> a  \n> &lt;img src=https://t.example/p.gif&gt;",
		},
		{
			"a code block in a blockquote", "<blockquote><pre><code class=\"language-go\">line1\n&lt;script&gt;alert(1)&lt;/script&gt;</code></pre></blockquote>",
			"> line1  \n> &lt;script&gt;alert(1)&lt;/script&gt;",
		},
		{
			"preformatted markup in a blockquote in a list", "<ol><li><blockquote><pre>x\n&lt;img src=https://t.example/p.gif&gt;</pre></blockquote></li></ol>",
			"1. > x  \n   > &lt;img src=https://t.example/p.gif&gt;",
		},
		{"inline code", "<p>Run <code>&lt;script&gt;</code> now</p>", "Run &lt;script&gt; now"},
		{"preformatted lines", "<pre>Your code\n\n  482913\n</pre>", "Your code\n\n482913"},
		{"the library's own marker characters in text", "<p>a\uf002&lt;script&gt;\u0007b</p>", "a&lt;script&gt;b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(converted(c.want), convert(c.html), compare.Options); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}
