//go:build banproof

package deltasync

// This file imports the HTML-to-Markdown converter on purpose. A job kind converts a body only through
// content/markdown, the one package that may call the converter, and the list for non-test code
// admits the converter, so only the job kind's list reports it.
import (
	_ "github.com/JohannesKaufmann/html-to-markdown/v2/converter" // want depguard "list 'worker-sync'"
)
