package gmail

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// The listings a page token can belong to. A token names its listing, so a token one listing
// issued is refused by the other.
const (
	threadListing  = "t"
	messageListing = "m"
)

// tokenPrefix starts every page token and cursor the adapter issues. It carries a version, so a
// token of another shape can be told apart later.
const tokenPrefix = "gmail.1."

// pageToken wraps Gmail's page token for a listing. Gmail's empty token, the last page, stays empty.
func pageToken(listing, gmailToken string) mail.PageToken {
	if gmailToken == "" {
		return ""
	}
	return mail.PageToken(tokenPrefix + listing + "." + gmailToken)
}

// readPageToken returns the Gmail page token a port token carries for a listing. The empty token
// is the first page, and a token the adapter did not issue for that listing is ErrInvalid.
func readPageToken(listing string, page mail.PageToken) (string, error) {
	if page == "" {
		return "", nil
	}
	gmailToken, ok := strings.CutPrefix(string(page), tokenPrefix+listing+".")
	if !ok || gmailToken == "" {
		return "", fmt.Errorf("gmail: page token %q: %w", page, mail.ErrInvalid)
	}
	return gmailToken, nil
}

// cursorListing names a cursor among the adapter's tokens.
const cursorListing = "h"

// cursor wraps a Gmail history identifier.
func cursor(historyID uint64) mail.Cursor {
	return mail.Cursor(tokenPrefix + cursorListing + "." + strconv.FormatUint(historyID, 10))
}

// readCursor returns the history identifier a cursor carries. A cursor the adapter did not issue is
// a cursor gap.
func readCursor(c mail.Cursor) (uint64, error) {
	digits, ok := strings.CutPrefix(string(c), tokenPrefix+cursorListing+".")
	if ok {
		if id, err := strconv.ParseUint(digits, 10, 64); err == nil && id > 0 {
			return id, nil
		}
	}
	return 0, fmt.Errorf("gmail: cursor %q: %w", c, mail.ErrCursorGap)
}
