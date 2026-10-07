//go:build banproof

package logging_test

import (
	l "log/slog"
	"testing"
)

// This file logs through the process's default logger from a test on purpose, so the ban is proven
// to reach test files too, which take the logger they hand the code under test as a value.
func TestLogThroughTheDefault(t *testing.T) {
	l.Info("progress") // want forbidigo `use of .l\.Info. forbidden because "project code logs through the logger its composition root hands it`
}
