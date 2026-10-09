//go:build banproof

package logging_test

import (
	"log"
	"log/slog"
	"testing"
)

// This file logs through the process's default loggers and a built-in print function from a test on
// purpose, since the ban covers test files as well. The last statement logs through a logger the test
// built, which the ban allows.
func TestLogThroughTheDefaults(t *testing.T) {
	slog.Info("i")   // want forbidigo `use of .slog\.Info. forbidden because "code logs through the logger it was handed`
	log.Println("p") // want forbidigo `use of .log\.Println. forbidden because "code logs through the logger it was handed`
	println("p")     // want forbidigo `use of .println. forbidden because "a built-in print writes plain text to standard error`
	slog.New(slog.DiscardHandler).Info("i")
}
