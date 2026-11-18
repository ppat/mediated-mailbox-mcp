package property_test

import (
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

// Probe reports a failure and what was logged for each way a function can fail or log, and ends a
// function at a fatal call as testing and rapid do.
func TestProbeRecordsFailuresAndTheirLogs(t *testing.T) {
	for _, c := range []struct {
		name       string
		f          func(rapid.TB)
		wantFailed bool
		wantLog    string
	}{
		{"a function that logs and passes", func(tb rapid.TB) { tb.Logf("seen %d", 1) }, false, "seen 1"},
		{"an error", func(tb rapid.TB) { tb.Errorf("bad %s", "x") }, true, "bad x"},
		{"a fatal call ends the function", func(tb rapid.TB) { tb.Fatal("stop"); tb.Log("after") }, true, "stop"},
		{"a skip ends the function and is no failure", func(tb rapid.TB) { tb.Skip("skipped"); tb.Fail() }, false, "skipped"},
		{"a panic", func(rapid.TB) { panic("boom") }, true, "panic: boom"},
	} {
		t.Run(c.name, func(t *testing.T) {
			failed, log := property.Probe(c.name, c.f)
			if failed != c.wantFailed || log != c.wantLog {
				t.Errorf("Probe = %t, %q, want %t, %q", failed, log, c.wantFailed, c.wantLog)
			}
			if strings.Contains(log, "after") {
				t.Errorf("the function went on past a fatal call: %q", log)
			}
		})
	}
}
