package logging_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/logging"
	"github.com/ppat/mediated-mailbox-mcp/logging/core"
	"github.com/ppat/mediated-mailbox-mcp/settings"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// line is the part of a JSON log line these tests read.
type line struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

// written returns the lines out holds, each of which must be a JSON object.
func written(t *testing.T, out *bytes.Buffer) []line {
	t.Helper()
	var got []line
	for text := range strings.Lines(out.String()) {
		var l line
		if err := json.Unmarshal([]byte(text), &l); err != nil {
			t.Fatalf("a log line is not a JSON object: %q: %v", text, err)
		}
		got = append(got, l)
	}
	return got
}

// logEachLevel logs one line at each level through a logger at level and returns what it wrote.
func logEachLevel(t *testing.T, level string) []line {
	t.Helper()
	var out bytes.Buffer
	logger, err := logging.New(&out, core.Config{Level: level}, nil)
	if err != nil {
		t.Fatal(err)
	}
	logger.Debug("a detail")
	logger.Info("progress")
	logger.Warn("a decision to act on")
	logger.Error("a failure")
	return written(t, &out)
}

// A logger writes JSON lines at the level its section names and above, and nothing below it.
func TestALoggerWritesItsLevelAndAbove(t *testing.T) {
	debug := line{Level: "DEBUG", Msg: "a detail"}
	info := line{Level: "INFO", Msg: "progress"}
	warn := line{Level: "WARN", Msg: "a decision to act on"}
	failure := line{Level: "ERROR", Msg: "a failure"}
	cases := []struct {
		level string
		want  []line
	}{
		{"debug", []line{debug, info, warn, failure}},
		{"info", []line{info, warn, failure}},
		{"warn", []line{warn, failure}},
		{"error", []line{failure}},
	}
	for _, c := range cases {
		t.Run(c.level, func(t *testing.T) {
			if diff := cmp.Diff(c.want, logEachLevel(t, c.level), compare.Options); diff != "" {
				t.Errorf("lines (-want +got):\n%s", diff)
			}
		})
	}
}

// A level the section refuses is refused with the layer that set it.
func TestARefusedLevelNamesItsSource(t *testing.T) {
	cases := []struct {
		name   string
		source settings.Source
		want   string
	}{
		{"flag", settings.Source{Layer: settings.Flag, Name: "--log.level"}, `log.level "INFO" is not debug, info, warn or error, set by the flag --log.level`},
		{"environment", settings.Source{Layer: settings.Environment, Name: "MEDIATED_MAILBOX_LOG__LEVEL"}, `log.level "INFO" is not debug, info, warn or error, set by the environment variable MEDIATED_MAILBOX_LOG__LEVEL`},
		{"file", settings.Source{Layer: settings.File, Name: "/etc/config.yaml line 3"}, `log.level "INFO" is not debug, info, warn or error, set by the file /etc/config.yaml line 3`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			values := []settings.Value{
				{Path: "database.host", Source: settings.Source{Layer: settings.Flag, Name: "--database.host"}, Value: "db.example"},
				{Path: "log.level", Source: c.source, Value: "INFO"},
			}
			var out bytes.Buffer
			_, err := logging.New(&out, core.Config{Level: "INFO"}, values)
			if err == nil {
				t.Fatalf("New accepted it, want %q", c.want)
			}
			if diff := cmp.Diff(c.want, err.Error(), compare.Options); diff != "" {
				t.Errorf("error (-want +got):\n%s", diff)
			}
			if out.Len() != 0 {
				t.Errorf("the refusal wrote %q", out.String())
			}
		})
	}
}

// The logger used before the configuration is loaded writes JSON lines at info and above.
func TestTheInitialLoggerWritesInfoAndAbove(t *testing.T) {
	var out bytes.Buffer
	logger := logging.Initial(&out)
	logger.Debug("a detail")
	logger.Info("progress")
	logger.Error("a failure")
	want := []line{{Level: "INFO", Msg: "progress"}, {Level: "ERROR", Msg: "a failure"}}
	if diff := cmp.Diff(want, written(t, &out), compare.Options); diff != "" {
		t.Errorf("lines (-want +got):\n%s", diff)
	}
}
