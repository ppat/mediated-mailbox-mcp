package service

import (
	"encoding/base64"
	"encoding/json"
)

// A listing's cursor is opaque to the client and bound to the account and the filter of the call that
// returned it (ADR-0087). It holds the position of the last row the page returned, and the next page
// starts after that row. A cursor given with another account, another listing or another filter is
// refused, so a page never continues a different listing than the one it came from.

// position is what a cursor carries. Listing names the operation, Account and Filter the call it
// continues, and At and Key the last row returned, its time and its identity.
type position struct {
	Listing string `json:"l"`
	Account string `json:"a"`
	Filter  string `json:"f,omitempty"`
	At      string `json:"t"`
	Key     string `json:"k"`
}

// encodeCursor returns the cursor for p.
func encodeCursor(p position) string {
	text, err := json.Marshal(p)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(text)
}

// decodeCursor reads the cursor the client sent to the listing for account and filter. An empty
// cursor starts from the first row. A cursor that does not decode, or that another listing, account
// or filter returned, is refused with an ArgumentError.
func decodeCursor(cursor, listing, account, filter string) (position, bool, error) {
	if cursor == "" {
		return position{}, false, nil
	}
	text, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return position{}, false, Refuse("cursor is not a cursor this listing returned")
	}
	var p position
	if err := json.Unmarshal(text, &p); err != nil || p.At == "" {
		return position{}, false, Refuse("cursor is not a cursor this listing returned")
	}
	if p.Listing != listing || p.Account != account || p.Filter != filter {
		return position{}, false, Refuse("cursor was returned for another account, listing or filter")
	}
	return p, true, nil
}
