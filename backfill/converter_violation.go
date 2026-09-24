//go:build banproof

package main

// This file imports the HTML-to-Markdown converter on purpose. The Backfill Job converts only through
// sanitize/markdown, and the list for non-test code admits the converter, so only the component's list
// reports it.
import (
	_ "github.com/JohannesKaufmann/html-to-markdown/v2/converter" // want depguard "list 'backfill'"
)
