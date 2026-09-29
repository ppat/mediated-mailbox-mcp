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
)

// allowedAbsolute are the absolute strings a bundle may carry because they can never become a request,
// each certified by a person with the reason (ADR-0064). The bundle holds none yet.
func allowedAbsolute() map[string]string {
	return map[string]string{}
}

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
