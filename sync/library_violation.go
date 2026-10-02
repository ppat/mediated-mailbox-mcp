//go:build banproof

package main

// This file imports the HTML-to-Markdown converter on purpose. The component's list does not admit it,
// while the list for non-test code does, so only the component's list reports it. A tick converts a
// body through sanitize/markdown, the one package that may call the converter.
import (
	_ "github.com/JohannesKaufmann/html-to-markdown/v2/converter" // want depguard "list 'sync'"
)
