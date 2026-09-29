package service

import (
	"strings"
	"time"
)

// Every timestamp crossing the client surface is ISO 8601 in UTC with the Z suffix. One leaving it is
// written in UTC whatever location it was read in, and one entering it with any other offset is
// refused rather than converted (ADR-0033).

// utcLayout writes a timestamp in UTC with the Z suffix and as many fractional digits as it holds.
const utcLayout = "2006-01-02T15:04:05.999999999Z"

// formatUTC writes t as the surface writes every timestamp.
func formatUTC(t time.Time) string { return t.UTC().Format(utcLayout) }

// parseUTC reads the argument name as a timestamp the surface accepts, ISO 8601 with the Z suffix. A
// timestamp carrying any other offset, +00:00 included, is refused with an ArgumentError, and so is
// text that is not a timestamp.
func parseUTC(name, text string) (time.Time, error) {
	if !strings.HasSuffix(text, "Z") {
		return time.Time{}, Refuse(name + " must be an ISO 8601 timestamp in UTC ending in Z, and other offsets are refused rather than converted")
	}
	t, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return time.Time{}, Refuse(name + " is not an ISO 8601 timestamp in UTC ending in Z")
	}
	return t, nil
}

// timestampSchema is the JSON Schema of a timestamp on the surface. The pattern states the Z suffix
// the service layer enforces, since date-time alone admits any offset.
const timestampSchema = `{"type":"string","format":"date-time","pattern":"Z$"}`
