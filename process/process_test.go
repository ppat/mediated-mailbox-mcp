package process_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/version"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// serving is a program's configuration type composed as core/config composes one that serves, the
// process's parts inline beside the program's own.
type serving struct {
	config.Process    `yaml:",inline"`
	config.Operations `yaml:",inline"`
	config.Serving    `yaml:",inline"`
	Own               own `yaml:"own"`
}

type own struct {
	Value int `yaml:"value"`
}

func (s serving) Check() error {
	for _, check := range []func() error{s.Process.Check, s.Operations.Check, s.Serving.Check, s.Own.check} {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

func (serving) Defaults(int) serving {
	return serving{Process: config.DefaultProcess(), Operations: config.DefaultOperations(), Serving: config.DefaultServing(), Own: own{Value: 1}}
}

func (o own) check() error {
	if o.Value < 0 {
		return errors.New("own.value is negative")
	}
	return nil
}

// once is the configuration type of a program that does one piece of work and exits, the process
// part inline beside its own and no operations part.
type once struct {
	config.Process `yaml:",inline"`
	Own            own `yaml:"own"`
}

func (o once) Check() error {
	if err := o.Process.Check(); err != nil {
		return err
	}
	return o.Own.check()
}

func (once) Defaults(int) once { return once{Process: config.DefaultProcess(), Own: own{Value: 1}} }

// jobsOnly is the configuration type of a program that runs job kinds and serves no endpoint, so it
// has no serving part.
type jobsOnly struct {
	config.Process    `yaml:",inline"`
	config.Operations `yaml:",inline"`
}

func (j jobsOnly) Check() error { return j.Process.Check() }

func (jobsOnly) Defaults(int) jobsOnly {
	return jobsOnly{Process: config.DefaultProcess(), Operations: config.DefaultOperations()}
}

// lockedBuffer is a destination the process and the test write and read from different goroutines.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// record is one JSON log record, by the fields the tests read.
type record struct {
	Level, Msg, Path, Source, Error, Component string
	Listen                                     string
	ProbeListen                                string `json:"probe_listen"`
	TLS                                        *bool
}

// records returns every record in logs, failing the test on a line that is not a JSON record.
func records(t *testing.T, logs string) []record {
	t.Helper()
	var got []record
	for line := range strings.Lines(logs) {
		var r record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("the log holds a line that is not a JSON record: %q", line)
		}
		got = append(got, r)
	}
	return got
}

func endpoint(pattern string, h http.Handler) process.Endpoint[serving] {
	return process.Endpoint[serving]{Pattern: pattern, Build: func(context.Context, process.Capabilities, serving) (http.Handler, error) {
		return h, nil
	}}
}

func task(work func(ctx context.Context, p process.Capabilities) error) *process.Task[once] {
	return &process.Task[once]{Build: func(_ context.Context, p process.Capabilities, _ once) (func(context.Context) error, error) {
		return func(ctx context.Context) error { return work(ctx, p) }, nil
	}}
}

// logFromAPart writes what a program's part writes once the start has set the level, one record at
// info and one at warn, through the logger the process hands the part.
func logFromAPart(p process.Capabilities) {
	p.Logger().Info("a part's record at info")
	p.Logger().Warn("a part's record at warn")
}

// starts are the two ways a program runs, each started the same way. Each runs a spec of its kind with
// args and environ and reports whether it went on to build its parts. The part's build writes a record
// at info and one at warn before it fails, so a start that gets that far writes past the level.
var starts = []struct {
	name string
	run  func(t *testing.T, init func([]string, []string) error, args, environ []string, stdout, stderr io.Writer) (bool, error)
}{
	{"until stopped", func(t *testing.T, init func([]string, []string) error, args, environ []string, stdout, stderr io.Writer) (bool, error) {
		built := false
		spec := process.Spec[serving]{
			Name: "test",
			Endpoints: []process.Endpoint[serving]{{Pattern: "/", Build: func(_ context.Context, p process.Capabilities, _ serving) (http.Handler, error) {
				built = true
				logFromAPart(p)
				return nil, errors.New("the build ran")
			}}},
		}
		if init != nil {
			spec.Init = func(a, e []string, _ serving) error { return init(a, e) }
		}
		err := runFor(t, spec, args, environ, stdout, stderr)
		if err != nil && strings.Contains(err.Error(), "the build ran") {
			err = nil
		}
		return built, err
	}},
	{"once", func(t *testing.T, init func([]string, []string) error, args, environ []string, stdout, stderr io.Writer) (bool, error) {
		built := false
		spec := process.Spec[once]{Name: "test", Task: &process.Task[once]{Build: func(_ context.Context, p process.Capabilities, _ once) (func(context.Context) error, error) {
			built = true
			logFromAPart(p)
			return nil, errors.New("the build ran")
		}}}
		if init != nil {
			spec.Init = func(a, e []string, _ once) error { return init(a, e) }
		}
		err := process.RunContext(t.Context(), args, environ, stdout, stderr, spec)
		if err != nil && strings.Contains(err.Error(), "the build ran") {
			err = nil
		}
		return built, err
	}},
}

// runFor runs spec with args and environ under a context ended after five seconds, every listener an
// ephemeral loopback port unless args names one.
func runFor[C config.Program[C]](t *testing.T, spec process.Spec[C], args, environ []string, stdout, stderr io.Writer) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	return process.RunContext(ctx, loopbackArgs[C](args), environ, stdout, stderr, spec)
}

// loopbackArgs adds ephemeral loopback listeners to args where args names none, the application
// server's only for a configuration type that has a serving part.
func loopbackArgs[C any](args []string) []string {
	flags := []string{"--probe_listen=127.0.0.1:0"}
	var zero C
	if _, ok := any(zero).(config.HasServing); ok {
		flags = append(flags, "--listen=127.0.0.1:0")
	}
	for _, flag := range flags {
		name, _, _ := strings.Cut(flag, "=")
		if !slices.ContainsFunc(args, func(a string) bool { return strings.HasPrefix(a, name+"=") }) {
			args = append([]string{flag}, args...)
		}
	}
	return args
}

// The process refuses to start while PGPASSWORD or PGSSLPASSWORD is set, even to an empty value,
// before it reads its configuration, so it logs no value and builds nothing, whichever way the program
// runs (ADR-0078, ADR-0139). The configuration file named does not exist, so a refusal made after the
// read would report the missing file instead.
func TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead(t *testing.T) {
	for _, s := range starts {
		for _, name := range []string{"PGPASSWORD", "PGSSLPASSWORD"} {
			for _, value := range []string{"secret", ""} {
				t.Run(s.name+"/"+name+"="+value, func(t *testing.T) {
					var stdout lockedBuffer
					environ := []string{"MEDIATED_MAILBOX_CONFIG=/absent/config.yaml", name + "=" + value}
					built, err := s.run(t, nil, nil, environ, &stdout, io.Discard)
					if err == nil || !strings.Contains(err.Error(), "the environment sets "+name) {
						t.Errorf("the run returned %v, want the start refused naming %s", err, name)
					}
					if built {
						t.Error("the process built its parts")
					}
					for _, r := range records(t, stdout.String()) {
						if r.Msg == "configuration" {
							t.Errorf("the process logged the configuration value %s before refusing", r.Path)
						}
					}
				})
			}
		}
	}
	// The driver's other variables pass the refusal, so the start goes on to read the configuration and
	// fails on the missing file instead.
	for _, s := range starts {
		environ := []string{"MEDIATED_MAILBOX_CONFIG=/absent/config.yaml", "PGHOST=elsewhere", "PGPASSFILE=/tmp/pass", "PGSSLMODE=disable", "HOME=/"}
		if _, err := s.run(t, nil, nil, environ, io.Discard, io.Discard); err == nil || strings.Contains(err.Error(), "the environment sets") {
			t.Errorf("%s: the run returned %v, want the driver's other variables let through to the configuration's read", s.name, err)
		}
	}
}

// --help and --version are answered on standard output and stop the start before any configuration is
// logged, any check runs or anything is built (ADR-0078).
func TestHelpAndVersionAreAnsweredOnStandardOutput(t *testing.T) {
	for _, s := range starts {
		for _, flag := range []string{"--help", "--version"} {
			t.Run(s.name+"/"+flag, func(t *testing.T) {
				var stdout lockedBuffer
				initRan := false
				built, err := s.run(t, func([]string, []string) error { initRan = true; return nil }, []string{flag, "--own.value=-1"}, nil, &stdout, io.Discard)
				if err != nil || built || initRan {
					t.Errorf("the run returned %v, built %t, ran Init %t, want nil with nothing run", err, built, initRan)
				}
				want := version.Number + "\n"
				if flag == "--help" {
					if !strings.Contains(stdout.String(), "own.value") {
						t.Errorf("--help answered %q, want every key", stdout.String())
					}
					return
				}
				if stdout.String() != want {
					t.Errorf("--version answered %q, want %q", stdout.String(), want)
				}
			})
		}
	}
}

// Every effective value is logged with its source before the level applies, then the level log_level
// names governs the log, so under warn the part's record at info is held back and its record at warn
// is written. A level that names none is refused after its source is logged, and the
// configuration's own checks and then the spec's Init run before anything is built (ADR-0078,
// ADR-0122, ADR-0142).
func TestTheStartLogsItsConfigurationThenSetsItsLevelAndRunsItsChecks(t *testing.T) {
	for _, s := range starts {
		t.Run(s.name+"/warn", func(t *testing.T) {
			var stdout lockedBuffer
			built, err := s.run(t, nil, []string{"--log_level=warn"}, nil, &stdout, io.Discard)
			if err != nil || !built {
				t.Fatalf("the run returned %v, built %t, want the parts built", err, built)
			}
			var paths []string
			warned := false
			for _, r := range records(t, stdout.String()) {
				switch {
				case r.Msg == "configuration":
					paths = append(paths, r.Path+" "+r.Source)
				case r.Level == "INFO":
					t.Errorf("the process wrote %q at info after log_level set warn", r.Msg)
				case r.Level == "WARN" && r.Msg == "a part's record at warn":
					warned = true
				}
			}
			if !slices.Contains(paths, "log_level flag --log_level") || !slices.Contains(paths, "own.value default") {
				t.Errorf("the configuration logged is %v, want every value with its source", paths)
			}
			// The part's record at warn shows the part wrote through the log, so the absence of its
			// record at info is the level's doing.
			if !warned {
				t.Errorf("the part's record at warn was not written:\n%s", stdout.String())
			}
		})
		for _, level := range []string{"INFO", "info+2"} {
			t.Run(s.name+"/a level naming none, "+level, func(t *testing.T) {
				var stdout lockedBuffer
				built, err := s.run(t, nil, []string{"--log_level=" + level}, nil, &stdout, io.Discard)
				if err == nil || !strings.HasPrefix(err.Error(), "validating the configuration: log_level") || built {
					t.Errorf("the run returned %v, built %t, want log_level refused", err, built)
				}
				if !strings.Contains(stdout.String(), `"path":"log_level","source":"flag --log_level","value":"`+level+`"`) {
					t.Errorf("the refused level's source was not logged:\n%s", stdout.String())
				}
			})
		}
		t.Run(s.name+"/a refused value under warn", func(t *testing.T) {
			var stdout lockedBuffer
			built, err := s.run(t, nil, []string{"--log_level=warn", "--own.value=-1"}, nil, &stdout, io.Discard)
			if err == nil || built {
				t.Errorf("the run returned %v, built %t, want own.value refused", err, built)
			}
			if !strings.Contains(stdout.String(), `"path":"own.value","source":"flag --own.value","value":-1`) {
				t.Errorf("the refused value's source was not logged under warn:\n%s", stdout.String())
			}
		})
		t.Run(s.name+"/the configuration's checks", func(t *testing.T) {
			initRan := false
			built, err := s.run(t, func([]string, []string) error { initRan = true; return nil }, []string{"--own.value=-1"}, nil, io.Discard, io.Discard)
			if err == nil || err.Error() != "validating the configuration: own.value is negative" || built || initRan {
				t.Errorf("the run returned %v, built %t, ran Init %t, want the check's refusal before Init", err, built, initRan)
			}
		})
		t.Run(s.name+"/Init", func(t *testing.T) {
			var gotArgs, gotEnviron []string
			args, environ := []string{"--own.value=2"}, []string{"MEDIATED_MAILBOX_OWN__VALUE=3"}
			built, err := s.run(t, func(a, e []string) error {
				gotArgs, gotEnviron = a, e
				return errors.New("the start check refuses")
			}, args, environ, io.Discard, io.Discard)
			if err == nil || err.Error() != "the start check refuses" || built {
				t.Errorf("the run returned %v, built %t, want Init's refusal before any build", err, built)
			}
			if !slices.Contains(gotArgs, "--own.value=2") || !slices.Equal(gotEnviron, environ) {
				t.Errorf("Init was given %v and %v, want what the process was given", gotArgs, gotEnviron)
			}
		})
	}
}

// A start the configuration leaves without a level logs at info.
func TestTheDefaultLevelIsInfo(t *testing.T) {
	var stdout lockedBuffer
	err := process.RunContext(t.Context(), nil, nil, &stdout, io.Discard, process.Spec[once]{Name: "test", Task: task(func(_ context.Context, p process.Capabilities) error {
		p.Logger().Info("working")
		return nil
	})})
	if err != nil || !strings.Contains(stdout.String(), `"msg":"working"`) {
		t.Errorf("the run returned %v and logged %s, want an info record", err, stdout.String())
	}
}

// The start banner names the published name and the release version, and below them where the
// program serves and where its probes listen, on standard error, which carries nothing else. A program
// that runs once has no lines below the first (ADR-0054, ADR-0122).
func TestTheStartBannerGoesToStandardError(t *testing.T) {
	var stderr lockedBuffer
	_, err := starts[0].run(t, nil, []string{"--listen=127.0.0.1:0", "--probe_listen=127.0.0.1:0"}, nil, io.Discard, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	want := "mediated-mailbox-test " + version.Number + "\n  serves  http on 127.0.0.1:0\n  probes  http on 127.0.0.1:0\n"
	if stderr.String() != want {
		t.Errorf("the banner is %q, want %q", stderr.String(), want)
	}
	var onceErr lockedBuffer
	if _, err := starts[1].run(t, nil, nil, nil, io.Discard, &onceErr); err != nil {
		t.Fatal(err)
	}
	if want := "mediated-mailbox-test " + version.Number + "\n"; onceErr.String() != want {
		t.Errorf("the banner of a program that runs once is %q, want %q", onceErr.String(), want)
	}
}

// A spec that names no program, lists nothing, or lists a task beside endpoints or job kinds is
// refused before the configuration is read, and so is a spec of endpoints whose configuration has no
// serving part, before anything is built (ADR-0142).
func TestASpecThatCannotRunIsRefused(t *testing.T) {
	noop := endpoint("/", http.NotFoundHandler())
	work := func(context.Context, process.Capabilities) error { return nil }
	cases := map[string]func() error{
		"no name": func() error {
			return runFor(t, process.Spec[serving]{Endpoints: []process.Endpoint[serving]{noop}}, nil, nil, io.Discard, io.Discard)
		},
		"nothing listed": func() error {
			return runFor(t, process.Spec[serving]{Name: "test"}, nil, nil, io.Discard, io.Discard)
		},
		"a task beside endpoints": func() error {
			return runFor(t, process.Spec[serving]{
				Name: "test", Endpoints: []process.Endpoint[serving]{noop},
				Task: &process.Task[serving]{Build: func(context.Context, process.Capabilities, serving) (func(context.Context) error, error) {
					return nil, nil
				}},
			}, nil, nil, io.Discard, io.Discard)
		},
		"a task beside job kinds": func() error {
			return runFor(t, process.Spec[once]{
				Name: "test", Task: task(work), Jobs: []process.Job[once]{{Name: "kind"}},
			}, nil, nil, io.Discard, io.Discard)
		},
		"endpoints with no serving part": func() error {
			return runFor(t, process.Spec[jobsOnly]{Name: "test", Endpoints: []process.Endpoint[jobsOnly]{{Pattern: "/", Build: func(context.Context, process.Capabilities, jobsOnly) (http.Handler, error) {
				t.Error("an endpoint was built")
				return http.NotFoundHandler(), nil
			}}}}, nil, nil, io.Discard, io.Discard)
		},
	}
	want := map[string]string{
		"no name":                        "the spec names no program",
		"nothing listed":                 "the spec lists no endpoint, no job kind and no task",
		"a task beside endpoints":        "the spec lists a task beside endpoints or job kinds",
		"a task beside job kinds":        "the spec lists a task beside endpoints or job kinds",
		"endpoints with no serving part": "the spec lists endpoints and its configuration has no serving part",
	}
	got := map[string]string{}
	for name, run := range cases {
		err := run()
		got[name] = fmt.Sprint(err)
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("refusals (-want +got):\n%s", diff)
	}
}

// A program that does one piece of work is run by the process as every program is. Its task is built
// with capabilities carrying the program's name, the work's error is returned once every goroutine the
// work started has returned and what the slots hold is closed, and the work runs with no operations
// endpoint, so a probe address is refused as a key the process does not read (ADR-0139, ADR-0142).
func TestAProgramThatRunsOnceDoesItsWorkAndClosesItsSlots(t *testing.T) {
	slot := process.NewOnePerProcess[*bool]("thing")
	var stdout lockedBuffer
	var closed, goroutineReturned bool
	err := process.RunContext(t.Context(), nil, nil, &stdout, io.Discard, process.Spec[once]{Name: "test", Task: task(func(ctx context.Context, p process.Capabilities) error {
		if _, err := slot.Get(ctx, p, func(context.Context, process.Capabilities) (*bool, func(), error) {
			return &closed, func() { closed = goroutineReturned }, nil
		}); err != nil {
			return err
		}
		p.Go("helper", func(context.Context) {
			time.Sleep(50 * time.Millisecond)
			goroutineReturned = true
		})
		p.Logger().Info("working")
		return errors.New("the work failed")
	})})
	if err == nil || err.Error() != "the work failed" {
		t.Errorf("the run returned %v, want the work's error", err)
	}
	if !closed {
		t.Error("the run returned before the slot was closed after the goroutine returned")
	}
	var component string
	for _, r := range records(t, stdout.String()) {
		if r.Msg == "working" {
			component = r.Component
		}
	}
	if component != "test" {
		t.Errorf("the work's record carries component %q, want the program's name", component)
	}
	err = process.RunContext(t.Context(), []string{"--probe_listen=:8080"}, nil, io.Discard, io.Discard, process.Spec[once]{Name: "test", Task: task(func(context.Context, process.Capabilities) error {
		t.Error("the work ran with a probe address")
		return nil
	})})
	if err == nil || !strings.HasPrefix(err.Error(), "loading the configuration: ") {
		t.Errorf("with a probe address the run returned %v, want the key refused", err)
	}
}
