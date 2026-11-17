package gmail

import (
	"errors"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

func TestPageTokens(t *testing.T) {
	if got := pageToken(threadListing, ""); got != "" {
		t.Errorf("the last page's token is %q, want empty", got)
	}
	issued := pageToken(threadListing, "09876")
	if issued != "gmail.1.t.09876" {
		t.Errorf("pageToken = %q", issued)
	}
	if got, err := readPageToken(threadListing, issued); err != nil || got != "09876" {
		t.Errorf("readPageToken(%q) = %q, %v, want Gmail's token back", issued, got, err)
	}
	if got, err := readPageToken(messageListing, ""); err != nil || got != "" {
		t.Errorf("readPageToken of the first page = %q, %v", got, err)
	}
}

// A token the adapter did not issue for a listing is refused before any call is made.
func TestReadPageTokenRefusesAForeignToken(t *testing.T) {
	for _, page := range []mail.PageToken{"not-a-page-token", "09876", "gmail.1.m.09876", "gmail.1.t.", "gmail.2.t.09876"} {
		if _, err := readPageToken(threadListing, page); !errors.Is(err, mail.ErrInvalid) {
			t.Errorf("readPageToken(%q) returned %v, want an error wrapping %v", page, err, mail.ErrInvalid)
		}
	}
}

func TestCursors(t *testing.T) {
	c := cursor(9876543)
	if c != "gmail.1.h.9876543" {
		t.Errorf("cursor = %q", c)
	}
	if got, err := readCursor(c); err != nil || got != 9876543 {
		t.Errorf("readCursor(%q) = %d, %v", c, got, err)
	}
	for _, foreign := range []mail.Cursor{"not-a-cursor", "", "9876543", "gmail.1.h.", "gmail.1.h.0", "gmail.1.h.-4", "gmail.1.t.12", "gmail.1.h.12x"} {
		if _, err := readCursor(foreign); !errors.Is(err, mail.ErrCursorGap) {
			t.Errorf("readCursor(%q) returned %v, want an error wrapping %v", foreign, err, mail.ErrCursorGap)
		}
	}
}
