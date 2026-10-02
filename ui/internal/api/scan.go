package api

import (
	"regexp"
	"strings"
)

// The scan the policy's tests run over the entry template and the built bundle (ADR-0062, with the
// proof split ADR-0064 records). It sits beside the policy it guards so a mutation patch can break
// it, and the binary never calls it.

var (
	// scriptElement is a script element. One without a src attribute is inline.
	scriptElement = regexp.MustCompile(`(?is)<script\b([^>]*)>`)
	// srcAttribute is a script element's src attribute.
	srcAttribute = regexp.MustCompile(`(?i)\bsrc\s*=`)
	// eventHandler is an inline event handler attribute, which the policy would also block.
	eventHandler = regexp.MustCompile(`(?i)<[a-z][^>]*\son[a-z]+\s*=`)
	// absolute is a URL that names an origin, with a scheme or scheme-relative.
	absolute = regexp.MustCompile(`(?i)\b(?:https?|wss?|ftp):\/\/[^\s"'<>)]+|["'(\s]\/\/[a-z0-9.-]+\.[a-z]{2,}`)
	// stylesheetURL is a url() or an @import in a stylesheet.
	stylesheetURL = regexp.MustCompile(`(?i)url\(\s*["']?([^"')]*)|@import\s+["']([^"']*)`)
	// fontFace is an @font-face rule's block, and inlineURL a url() holding a data: URI. They read the
	// minified stylesheet bun writes, and hand-written CSS can slip past them, with a comment between
	// @font-face and its block, a closing brace inside a quoted family name, or an escape in url. The
	// build's own test, which refuses any data: in the built stylesheet, is the stricter guard there.
	fontFace  = regexp.MustCompile(`(?is)@font-face\s*\{[^}]*\}`)
	inlineURL = regexp.MustCompile(`(?i)url\(\s*["']?data:`)
)

// allowedAbsolute are the absolute strings a bundle may carry because they can never become a request,
// each certified by a person with the reason (ADR-0064). Each is matched whole, so no other URL on the
// same origin passes. The operator certified the first four on 2026-09-30, and the seven OAuth client
// setup carries are listed for the operator's certification.
func allowedAbsolute() map[string]string {
	return map[string]string{
		"http://www.w3.org/2000/svg": "Preact's renderer passes the SVG namespace to " +
			"document.createElementNS, which names a namespace and fetches nothing",
		"http://www.w3.org/1998/Math/MathML": "Preact's renderer passes the MathML namespace to " +
			"document.createElementNS, which names a namespace and fetches nothing",
		"http://www.w3.org/1999/xhtml": "Preact's renderer passes the XHTML namespace to " +
			"document.createElementNS as the default, which names a namespace and fetches nothing",
		"https://github.com/preactjs/preact-iso#locationprovider": "preact-iso writes it into the " +
			"message of the error it throws when a hook runs outside its location provider, and never " +
			"requests it",
		// OAuth client setup's console pages, each the address of a link the operator follows or a
		// window the operator's click opens, a navigation the policy does not govern and the page
		// never fetches (docs/UI.md section 8.11).
		"https://console.cloud.google.com/projectcreate":                     consolePage,
		"https://console.cloud.google.com/apis/library/gmail.googleapis.com": consolePage,
		"https://console.cloud.google.com/auth/overview":                     consolePage,
		"https://console.cloud.google.com/auth/scopes":                       consolePage,
		"https://console.cloud.google.com/auth/audience":                     consolePage,
		"https://console.cloud.google.com/auth/clients":                      consolePage,
		"https://www.googleapis.com/auth/gmail.modify": "the Gmail scope's name, shown as the value " +
			"the operator enters at OAuth client setup's step 4, which names a scope and fetches nothing",
	}
}

// consolePage is why a Google Cloud console page may sit in the bundle.
const consolePage = "a Google Cloud console page OAuth client setup links to, which the operator " +
	"opens and the page never fetches"

// NeedsLooserPolicy returns what in one shipped file the policy would have to be loosened for, each finding named
// by its kind.
func NeedsLooserPolicy(name, text string) []string {
	var findings []string
	for _, m := range scriptElement.FindAllStringSubmatch(text, -1) {
		if !srcAttribute.MatchString(m[1]) {
			findings = append(findings, "inline script")
		}
	}
	if eventHandler.MatchString(text) {
		findings = append(findings, "inline event handler")
	}
	if strings.Contains(strings.ToLower(text), "javascript:") {
		findings = append(findings, "script URL")
	}
	for _, m := range absolute.FindAllString(text, -1) {
		m = strings.TrimLeft(m, `"'( `+"\t\n")
		if _, ok := allowedAbsolute()[m]; !ok {
			findings = append(findings, "another origin: "+m)
		}
	}
	if strings.HasSuffix(name, ".css") {
		// The policy's img-src admits data: and its font-src does not, so a font inlined as a data: URI is
		// blocked while an inlined image loads (docs/UI.md section 14.3).
		for _, block := range fontFace.FindAllString(text, -1) {
			if inlineURL.MatchString(block) {
				findings = append(findings, "font inlined as a data: URI")
			}
		}
		for _, m := range stylesheetURL.FindAllStringSubmatch(text, -1) {
			u := m[1] + m[2]
			if !strings.HasPrefix(u, "/") || strings.HasPrefix(u, "//") {
				if !strings.HasPrefix(u, "data:") {
					findings = append(findings, "stylesheet resource off the UI's origin: "+u)
				}
			}
		}
	}
	return findings
}
