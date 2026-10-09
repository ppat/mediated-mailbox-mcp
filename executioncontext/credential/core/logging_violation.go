//go:build banproof

package core

// This file logs from a pure core on purpose, from a core path element at depth three, and banproof
// requires every want below. Both logging packages are refused by the pure-core list alone, since
// the credential section's own list and the list for non-test code admit the standard library. The
// built-in println needs no import, so the default-logger ban is what refuses it.
import (
	"log"      // want depguard "import 'log' is not allowed from list 'pure-core'"
	"log/slog" // want depguard "import 'log/slog' is not allowed from list 'pure-core'"
)

func logFromACore(l *slog.Logger, s *log.Logger) {
	l.Info("from a core")
	s.Print("from a core")
	println("from a core") // want forbidigo `use of .println. forbidden because "a built-in print writes plain text to standard error`
}
