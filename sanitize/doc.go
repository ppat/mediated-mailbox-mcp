// Package sanitize is the body sanitization library, published as mediated-mailbox-sanitize. It is a
// narrow, named exception to the rule that shared code is pure, because the conversion it holds calls
// an outside library.
//
// The mediator sanitizes a body at serve time and the Backfill Job sanitizes it before the Content
// Scanner reads it, so both import the one conversion here and the scanner reads the Markdown a client
// would receive. The conversion sits under markdown. Nothing belongs in this package itself.
package sanitize
