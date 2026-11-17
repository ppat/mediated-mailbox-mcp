// Package core holds the database package's pure decision, the label a statement's latency is
// recorded under (ADR-0125). It imports no clock and does no I/O.
package core

import "strings"

// unnamed is the label of a statement whose SQL carries no name and starts with no transaction
// keyword, so the label's values stay a closed set.
const unnamed = "unnamed"

// Label returns a statement's label. It is the name in the `-- name: <Name>` comment the statement's
// SQL starts with, as sqlc writes it, begin, commit or rollback for a statement without one that
// starts with that keyword, and unnamed for every other, so no label is taken from the SQL's text
// beyond a name the code chose.
func Label(sql string) string {
	text := strings.TrimSpace(sql)
	if rest, ok := strings.CutPrefix(text, "-- name:"); ok {
		line, _, _ := strings.Cut(rest, "\n")
		if fields := strings.Fields(line); len(fields) > 0 && identifier(fields[0]) {
			return fields[0]
		}
		return unnamed
	}
	first, _, _ := strings.Cut(text, " ")
	switch keyword := strings.ToLower(strings.TrimSuffix(first, ";")); keyword {
	case "begin", "commit", "rollback":
		return keyword
	default:
		return unnamed
	}
}

// identifier reports whether name is letters, digits and underscores, the form sqlc requires of a
// statement's name, so a comment holding anything else is no name.
func identifier(name string) bool {
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return name != ""
}
