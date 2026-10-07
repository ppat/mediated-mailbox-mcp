//go:build banproof

package core

// This file imports the standard library's two logging packages into a pure core on purpose, so the
// pure-core list is proven to refuse them. A pure core returns its decisions as values and never
// logs, so no line about a decision is written but by the shell that holds it (ADR-0040, ADR-0119).
import (
	_ "log"      // want depguard "import 'log' is not allowed from list 'pure-core'"
	_ "log/slog" // want depguard "import 'log/slog' is not allowed from list 'pure-core'"
)
