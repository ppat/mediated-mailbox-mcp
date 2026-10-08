// Package content is the root of the message content family, published as mediated-mailbox-content.
// The family holds the code that reads a message's content in memory and stores none of it, and it
// is a narrow, named exception to the rule that shared code is pure, because the conversion it holds
// calls an outside library. content/README.md argues its case.
//
// The mediator converts a body at serve time and the Backfill Job and delta sync convert it before the
// Content Scanner reads it, so each imports the one conversion in content/markdown and the scanner
// reads the Markdown a client would receive. Nothing belongs in this package itself.
package content
