package core_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/logging/core"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// Each of the four words names its level, in order of severity.
func TestEachLevelIsNamed(t *testing.T) {
	cases := []struct {
		text string
		want core.Level
	}{
		{"debug", core.Debug},
		{"info", core.Info},
		{"warn", core.Warn},
		{"error", core.Error},
	}
	for _, c := range cases {
		got, err := core.Parse(core.Config{Level: c.text})
		if err != nil {
			t.Errorf("%s: %v", c.text, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s parsed as %d, want %d", c.text, got, c.want)
		}
	}
	if core.Debug >= core.Info || core.Info >= core.Warn || core.Warn >= core.Error {
		t.Errorf("the levels are not ordered by severity: %d %d %d %d", core.Debug, core.Info, core.Warn, core.Error)
	}
}

// Any other text is refused, the spellings a log library would also accept included.
func TestAnyOtherLevelIsRefused(t *testing.T) {
	for _, text := range []string{"", "INFO", "Info", "warning", "fatal", "trace", "info+2", "INFO+2", "-4", "0", " info", "info "} {
		_, err := core.Parse(core.Config{Level: text})
		want := `log.level "` + text + `" is not debug, info, warn or error`
		if err == nil {
			t.Errorf("%q was accepted, want %q", text, want)
			continue
		}
		if diff := cmp.Diff(want, err.Error(), compare.Options); diff != "" {
			t.Errorf("%q (-want +got):\n%s", text, diff)
		}
	}
}

// The default logs at info.
func TestTheDefaultIsInfo(t *testing.T) {
	if diff := cmp.Diff(core.Config{Level: "info"}, core.Default(), compare.Options); diff != "" {
		t.Errorf("default (-want +got):\n%s", diff)
	}
}

// The log section is pinned field by field, so a new value is a visible change (ADR-0078).
func TestTheSectionIsPinned(t *testing.T) {
	mustnotcompile.RequireFields(t, "github.com/ppat/mediated-mailbox-mcp/logging/core", "Config",
		"Level string",
	)
}
