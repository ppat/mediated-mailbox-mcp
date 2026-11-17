package settings_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	durations "github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process/settings"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// The same tree twice, once with its lengths of time as time.Duration, which the YAML decoder reads
// and writes itself, and once as config.Duration, which the library reads and writes through it. The
// inline part and the section holding a list and a keyed map of lengths reach every place the
// library walks.
type timed struct {
	TimedOwn `yaml:",inline"`
	Jobs     TimedJobs `yaml:"jobs"`
}

type TimedOwn struct {
	Interval time.Duration `yaml:"interval"`
}

type TimedJobs struct {
	Window  time.Duration            `yaml:"window"`
	Backoff []time.Duration          `yaml:"backoff"`
	Per     map[string]time.Duration `yaml:"per"`
}

type counted struct {
	CountedOwn `yaml:",inline"`
	Jobs       CountedJobs `yaml:"jobs"`
}

type CountedOwn struct {
	Interval durations.Duration `yaml:"interval"`
}

type CountedJobs struct {
	Window  durations.Duration            `yaml:"window"`
	Backoff []durations.Duration          `yaml:"backoff"`
	Per     map[string]durations.Duration `yaml:"per"`
}

func timedDefaults() timed {
	return timed{
		TimedOwn: TimedOwn{Interval: 5 * time.Minute},
		Jobs:     TimedJobs{Window: 7 * 24 * time.Hour, Backoff: []time.Duration{time.Second, 90 * time.Second}, Per: map[string]time.Duration{"sync": 30 * time.Second}},
	}
}

func countedDefaults() counted {
	return counted{
		CountedOwn: CountedOwn{Interval: durations.Duration(5 * time.Minute)},
		Jobs: CountedJobs{
			Window:  durations.Duration(7 * 24 * time.Hour),
			Backoff: []durations.Duration{durations.Duration(time.Second), durations.Duration(90 * time.Second)},
			Per:     map[string]durations.Duration{"sync": durations.Duration(30 * time.Second)},
		},
	}
}

// asTimed is c with its lengths of time as time.Duration, for comparing the two loads.
func asTimed(c counted) timed {
	t := timed{TimedOwn: TimedOwn{Interval: time.Duration(c.Interval)}, Jobs: TimedJobs{Window: time.Duration(c.Jobs.Window)}}
	for _, d := range c.Jobs.Backoff {
		t.Jobs.Backoff = append(t.Jobs.Backoff, time.Duration(d))
	}
	if c.Jobs.Per != nil {
		t.Jobs.Per = map[string]time.Duration{}
		for k, d := range c.Jobs.Per {
			t.Jobs.Per[k] = time.Duration(d)
		}
	}
	return t
}

// A config.Duration is read from the same text a time.Duration is, in every layer, and refused for
// the same text with the same error, such as a bare number, which the YAML decoder never reads as a
// length of time. Its help, its effective values and the rendering its sections' revisions are taken
// from are a time.Duration's, so a revision does not move when a configuration type's lengths of
// time become config.Duration (ADR-0078).
func TestADurationIsReadAndWrittenAsText(t *testing.T) {
	dir := t.TempDir()
	write := func(text string) string {
		name := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(name, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		return name
	}
	texts := []string{
		"5m", "1h30m", "-5m", "0s", "300ms", "1.5h", "2h45m30.5s", `"5m"`, "'90s'",
		"0", "5", "300000000000", "1.5", "-1", "1d", "5 m", "abc", "true", "~", "[5m]", "{a: 5m}", `""`, "",
	}
	type outcome struct {
		Config   timed
		Values   []settings.Value
		Revision map[string]string
		Err      string
	}
	run := func(name string, args []string, environ []string) {
		t.Helper()
		wantLoaded, wantErr := settings.Load(timedDefaults(), args, environ)
		gotLoaded, gotErr := settings.Load(countedDefaults(), args, environ)
		want := outcome{Config: wantLoaded.Config, Revision: wantLoaded.Revisions}
		got := outcome{Config: asTimed(gotLoaded.Config), Revision: gotLoaded.Revisions}
		if wantErr != nil {
			want.Err = strings.ReplaceAll(wantErr.Error(), "settings_test.timed", "settings_test.counted")
			want.Err = strings.ReplaceAll(want.Err, "settings_test.Timed", "settings_test.Counted")
			want.Config = timed{}
		}
		if gotErr != nil {
			got.Err = gotErr.Error()
			got.Config = timed{}
		}
		for _, v := range wantLoaded.Values {
			v.Value = asText(v.Value)
			want.Values = append(want.Values, v)
		}
		got.Values = gotLoaded.Values
		if diff := cmp.Diff(want, got, compare.Options); diff != "" {
			t.Errorf("%s: config.Duration differs from time.Duration (-want +got):\n%s", name, diff)
		}
	}
	run("the defaults", nil, nil)
	for _, text := range texts {
		run("flag "+text, []string{"--interval=" + text, "--jobs.per.sync=" + text}, nil)
		run("environment "+text, nil, []string{"MEDIATED_MAILBOX_JOBS__WINDOW=" + text})
		run("flag list "+text, []string{"--jobs.backoff=[1s, " + text + "]"}, nil)
		if text != "" {
			run("file "+text, []string{"--config-file=" + write("interval: "+text+"\njobs:\n  window: "+text+"\n  backoff: ["+text+"]\n  per:\n    sync: "+text+"\n")}, nil)
		}
	}
	run("a file section of the wrong shape", []string{"--config-file=" + write("jobs: 5m\n")}, nil)
	run("a file key the type does not declare", []string{"--config-file=" + write("jobs:\n  windwo: 5m\n")}, nil)

	if diff := cmp.Diff(helpOf(t, timedDefaults()), helpOf(t, countedDefaults()), compare.Options); diff != "" {
		t.Errorf("help (-want +got):\n%s", diff)
	}
}

// What a config.Duration holds is the length the text names in nanoseconds, and it is written back
// as the text a time.Duration writes, in the help, the effective values, a list of them included,
// and the rendering a revision is taken from, never as the number.
func TestADurationHoldsNanosecondsAndIsWrittenAsTheText(t *testing.T) {
	loaded, err := settings.Load(countedDefaults(), []string{"--interval=1h30m", "--jobs.backoff=[2s]"}, []string{"MEDIATED_MAILBOX_JOBS__PER__SYNC=250ms"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := int64(loaded.Config.Interval), int64(5_400_000_000_000); got != want {
		t.Errorf("interval holds %d, want %d nanoseconds", got, want)
	}
	if got, want := int64(loaded.Config.Jobs.Per["sync"]), int64(250_000_000); got != want {
		t.Errorf("jobs.per.sync holds %d, want %d nanoseconds", got, want)
	}
	values := map[string]any{}
	for _, v := range loaded.Values {
		values[v.Path] = v.Value
	}
	if diff := cmp.Diff(map[string]any{"interval": "1h30m0s", "jobs.window": "168h0m0s", "jobs.per.sync": "250ms"},
		map[string]any{"interval": values["interval"], "jobs.window": values["jobs.window"], "jobs.per.sync": values["jobs.per.sync"]}, compare.Options); diff != "" {
		t.Errorf("effective values (-want +got):\n%s", diff)
	}
	if got, want := values["jobs.backoff"], []string{"2s"}; !cmp.Equal(got, want) {
		t.Errorf("jobs.backoff is %#v, want %#v", got, want)
	}
	help := helpOf(t, countedDefaults())
	for _, want := range []string{"\ninterval\n  environment: MEDIATED_MAILBOX_INTERVAL\n  flag: --interval=VALUE\n  default: 5m0s\n", "jobs.per.sync: 30s\n", "  default: [1s, 1m30s]\n"} {
		if !strings.Contains(help, want) {
			t.Errorf("help lacks %q:\n%s", want, help)
		}
	}
	text, err := settings.Load(timed{TimedOwn: TimedOwn{Interval: 90 * time.Minute}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	counted, err := settings.Load(counted{CountedOwn: CountedOwn{Interval: durations.Duration(90 * time.Minute)}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := counted.Revisions["interval"], text.Revisions["interval"]; got != want {
		t.Errorf("the revision of a length of time is %s, want %s, its text's", got, want)
	}
}

// helpOf returns the help Load writes for defaults.
func helpOf[T any](t *testing.T, defaults T) string {
	t.Helper()
	_, err := settings.Load(defaults, []string{"--help"}, nil)
	var help *settings.HelpRequested
	if !errors.As(err, &help) {
		t.Fatalf("Load returned %v, want the help", err)
	}
	return help.Text
}

// asText is an effective value of the tree whose lengths of time are time.Duration as the library
// reports a config.Duration's, each length of time, alone, in a list or in a map, as its text.
func asText(v any) any {
	switch v := v.(type) {
	case time.Duration:
		return v.String()
	case []time.Duration:
		out := make([]string, len(v))
		for i, d := range v {
			out[i] = d.String()
		}
		return out
	case map[string]time.Duration:
		out := make(map[string]string, len(v))
		for k, d := range v {
			out[k] = d.String()
		}
		return out
	}
	return v
}
