package mcp

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

// The server's records below warn and at info or above are lowered to debug, and its warnings and
// errors keep their levels, so the server's failures reach the log at the default level (ADR-0122).
// The SDK writes no warning or error a test can reach through the stateless root, so this test hands
// the handler records directly.
func TestTheServersWarningsAndErrorsKeepTheirLevels(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(routineAtDebug{slog.NewTextHandler(&out, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})}).With("from", "sdk")
	ctx := context.Background()
	logger.Info("server connecting")
	logger.Log(ctx, (slog.LevelInfo+slog.LevelWarn)/2, "a notice")
	logger.Warn("calling a method failed")
	logger.Error("server connect error")
	want := "level=WARN msg=\"calling a method failed\" from=sdk\nlevel=ERROR msg=\"server connect error\" from=sdk\n"
	if got := out.String(); got != want {
		t.Errorf("the handler wrote:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(out.String(), "INFO") {
		t.Errorf("a record was written at info")
	}
}
