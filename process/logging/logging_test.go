package logging_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/process/logging"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// Each of the four names reads as its level, and any other text, slog's own spellings included, is
// refused.
func TestALevelIsOneOfFourNames(t *testing.T) {
	read := []struct {
		name string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
	}
	for _, c := range read {
		t.Run(c.name, func(t *testing.T) {
			got, err := logging.ParseLevel(c.name)
			if err != nil {
				t.Fatalf("ParseLevel(%q) refused it: %v", c.name, err)
			}
			if got != c.want {
				t.Errorf("ParseLevel(%q) = %v, want %v", c.name, got, c.want)
			}
		})
	}
	refused := []struct{ label, name, quoted string }{
		{"empty", "", `""`},
		{"upper case", "INFO", `"INFO"`},
		{"title case", "Warn", `"Warn"`},
		{"warning", "warning", `"warning"`},
		{"an offset up", "info+2", `"info+2"`},
		{"an offset down", "debug-4", `"debug-4"`},
		{"trace", "trace", `"trace"`},
		{"a leading space", " info", `" info"`},
		{"a trailing newline", "info\n", `"info\n"`},
	}
	for _, c := range refused {
		t.Run("refused "+c.label, func(t *testing.T) {
			_, err := logging.ParseLevel(c.name)
			want := "log_level " + c.quoted + " is not one of debug, info, warn and error"
			if err == nil || err.Error() != want {
				t.Errorf("ParseLevel(%q) returned %v, want %q", c.name, err, want)
			}
		})
	}
}

// The default is a name the parser reads, and it is info.
func TestTheDefaultLevelIsInfo(t *testing.T) {
	got, err := logging.ParseLevel(logging.DefaultLevel)
	if err != nil || got != slog.LevelInfo {
		t.Errorf("ParseLevel(DefaultLevel) = %v, %v, want INFO", got, err)
	}
}

// records decodes each line of out as one JSON object.
func records(t *testing.T, out *bytes.Buffer) []map[string]any {
	t.Helper()
	var got []map[string]any
	for _, line := range strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		var r map[string]any
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("a line is not one JSON object: %q: %v", line, err)
		}
		delete(r, "time")
		got = append(got, r)
	}
	return got
}

// The logger writes one JSON object per record at or above the level its variable holds when the
// record is written, and a logger derived with attributes follows the same variable.
func TestTheLoggerWritesJSONAtTheLevelItsVariableHolds(t *testing.T) {
	var out bytes.Buffer
	level := new(slog.LevelVar)
	logger := logging.New(&out, level)
	job := logger.With("job_kind", "backfill", "account", "personal")

	logger.Info("written at info")
	logger.Debug("below info")
	level.Set(slog.LevelWarn)
	job.Info("below warn")
	job.Warn("written at warn", "page", 3)
	logger.Error("written at error")

	want := []map[string]any{
		{"level": "INFO", "msg": "written at info"},
		{"level": "WARN", "msg": "written at warn", "job_kind": "backfill", "account": "personal", "page": float64(3)},
		{"level": "ERROR", "msg": "written at error"},
	}
	if diff := cmp.Diff(want, records(t, &out), compare.Options); diff != "" {
		t.Errorf("records (-want +got):\n%s", diff)
	}
}

// A server's error log writes through the handed logger at warn, so it follows the same level.
func TestAServersErrorLogWritesAtWarn(t *testing.T) {
	var out bytes.Buffer
	level := new(slog.LevelVar)
	serverLog := logging.ServerErrorLog(logging.New(&out, level))

	serverLog.Print("http: TLS handshake error from 192.0.2.1:4000: EOF")
	level.Set(slog.LevelError)
	serverLog.Print("not written above warn")

	want := []map[string]any{{"level": "WARN", "msg": "http: TLS handshake error from 192.0.2.1:4000: EOF"}}
	if diff := cmp.Diff(want, records(t, &out), compare.Options); diff != "" {
		t.Errorf("records (-want +got):\n%s", diff)
	}
}
