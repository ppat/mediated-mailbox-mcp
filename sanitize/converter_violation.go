//go:build banproof

package sanitize

// This file imports the HTML-to-Markdown converter on purpose from outside the conversion subsection.
// The library's own list admits the converter, so only the list for the rest of the library reports it.
import (
	_ "github.com/JohannesKaufmann/html-to-markdown/v2/converter" // want depguard "list 'sanitize-without-converter'"
)
