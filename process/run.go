package process

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process/logging"
	"github.com/ppat/mediated-mailbox-mcp/process/settings"
	"github.com/ppat/mediated-mailbox-mcp/process/version"
)

// Run runs spec as a program's main.go does, with the program's arguments and environment, until the
// program is told to stop by SIGINT or SIGTERM, or its task is done. It builds the logger over
// standard output before the configuration exists and makes it the default for code the project does
// not own (ADR-0122). A run that fails logs its error and exits with status 1.
func Run[C config.Program[C]](args, environ []string, spec Spec[C]) {
	level := new(slog.LevelVar)
	logger := logging.New(os.Stdout, level)
	slog.SetDefault(logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, args, environ, os.Stdout, os.Stderr, logger, level, spec, nil)
	stop()
	if err != nil {
		os.Exit(1)
	}
}

// RunContext runs spec as Run does, until ctx ends or its task is done, writing its log records and
// its answers to --help and --version to stdout and its start banner to stderr, and returns the run's
// error rather than exiting. A test or a harness runs a program through it, so what it runs is what
// the program's main.go runs, apart from the shared things the options substitute, which Run never
// does (ADR-0142).
func RunContext[C config.Program[C]](ctx context.Context, args, environ []string, stdout, stderr io.Writer, spec Spec[C], options ...Option) error {
	level := new(slog.LevelVar)
	return run(ctx, args, environ, stdout, stderr, logging.New(stdout, level), level, spec, options)
}

// run is the whole process. Its error is logged before it is returned, so a start that fails before
// the level is read still reports it as JSON (ADR-0122).
func run[C config.Program[C]](ctx context.Context, args, environ []string, stdout, stderr io.Writer, logger *slog.Logger, level *slog.LevelVar, spec Spec[C], options []Option) error {
	err := runSpec(ctx, args, environ, stdout, stderr, logger, level, spec, options)
	if err != nil {
		logger.Error("the process failed", "error", err)
	}
	return err
}

func runSpec[C config.Program[C]](ctx context.Context, args, environ []string, stdout, stderr io.Writer, logger *slog.Logger, level *slog.LevelVar, spec Spec[C], options []Option) error {
	if err := refuseSpec(spec); err != nil {
		return err
	}
	c, started, err := start(args, environ, stdout, stderr, logger, level, spec)
	if !started {
		return err
	}
	pr, err := newProcess(spec.Name, logger, c.revisions)
	if err != nil {
		return err
	}
	for _, o := range options {
		o.fill(pr)
	}
	if spec.Task != nil {
		return runTask(ctx, pr, spec, c.config)
	}
	return serve(ctx, pr, spec, c.config, c.parts)
}

// refuseSpec refuses a spec that names no program, lists nothing, or lists a task beside endpoints or
// job kinds, since a program either does one piece of work and exits or runs until it is told to
// stop.
func refuseSpec[C any](spec Spec[C]) error {
	switch {
	case spec.Name == "":
		return errors.New("the spec names no program")
	case spec.Task != nil && (len(spec.Endpoints) > 0 || len(spec.Jobs) > 0):
		return errors.New("the spec lists a task beside endpoints or job kinds")
	case spec.Task == nil && len(spec.Endpoints) == 0 && len(spec.Jobs) == 0:
		return errors.New("the spec lists no endpoint, no job kind and no task")
	}
	return nil
}

// loaded is the loaded configuration, and the process's parts of it a program that runs until it is
// told to stop serves from.
type loaded[C any] struct {
	config    C
	parts     parts
	revisions map[string]string
}

// parts are the operations part, and the serving part of a spec that lists endpoints.
type parts struct {
	operations config.Operations
	serving    config.Serving
}

// start is what every process does before it builds anything, in the order ADR-0142 sets. It refuses
// the database driver's password variables before the configuration is read, loads the configuration
// and answers --help and --version on stdout, logs every effective value with the layer that set it,
// then sets the level from log_level, runs the configuration's checks and the spec's Init, and writes
// the start banner to stderr. started is false with a nil error once --help or --version is answered.
func start[C config.Program[C]](args, environ []string, stdout, stderr io.Writer, logger *slog.Logger, level *slog.LevelVar, spec Spec[C]) (l loaded[C], started bool, err error) {
	if err := RefusePasswordVariables(environ); err != nil {
		return l, false, err
	}
	var zero C
	got, err := settings.Load(zero.Defaults(runtime.NumCPU()), args, environ)
	var help *settings.HelpRequested
	if errors.As(err, &help) {
		_, err := io.WriteString(stdout, help.Text)
		return l, false, err
	}
	if errors.As(err, new(*settings.VersionRequested)) {
		_, err := fmt.Fprintln(stdout, version.Number)
		return l, false, err
	}
	if err != nil {
		return l, false, fmt.Errorf("loading the configuration: %w", err)
	}
	// The effective configuration is written before the level applies, so a refused value's source,
	// log_level's included, reaches the log whatever level the configuration sets (ADR-0078, ADR-0122).
	for _, v := range got.Values {
		logger.Info("configuration", "path", v.Path, "source", v.Source.String(), "value", v.Value)
	}
	c := got.Config
	lv, err := logging.ParseLevel(c.ProcessPart().LogLevel)
	if err != nil {
		return l, false, fmt.Errorf("validating the configuration: %w", err)
	}
	level.Set(lv)
	if err := c.Check(); err != nil {
		return l, false, fmt.Errorf("validating the configuration: %w", err)
	}
	if spec.Init != nil {
		if err := spec.Init(args, environ, c); err != nil {
			return l, false, err
		}
	}
	served, lines, err := partsOf(spec, c)
	if err != nil {
		return l, false, err
	}
	writeBanner(stderr, banner("mediated-mailbox-"+spec.Name, lines), logger)
	return loaded[C]{config: c, parts: served, revisions: got.Revisions}, true, nil
}

// partsOf returns the parts the process serves from and the banner's lines below the one naming the
// program and its release, where it serves and where its probes listen. A program that runs until it
// is told to stop needs the operations part, and one that lists endpoints the serving part too, so a
// spec whose configuration type lacks them is refused here.
func partsOf[C any](spec Spec[C], c C) (parts, []string, error) {
	var p parts
	if spec.Task != nil {
		return p, nil, nil
	}
	ops, ok := any(c).(config.HasOperations)
	if !ok {
		return p, nil, errors.New("the spec lists endpoints or job kinds and its configuration has no operations part")
	}
	p.operations = ops.OperationsPart()
	var lines []string
	if len(spec.Endpoints) > 0 {
		s, ok := any(c).(config.HasServing)
		if !ok {
			return p, nil, errors.New("the spec lists endpoints and its configuration has no serving part")
		}
		p.serving = s.ServingPart()
		lines = append(lines, fmt.Sprintf("serves  %s on %s", scheme(p.serving.ServesTLS()), p.serving.Listen))
	}
	return p, append(lines, "probes  http on "+p.operations.ProbeListen), nil
}

// RefusePasswordVariables refuses a start while PGPASSWORD or PGSSLPASSWORD is set, an empty value
// included, since the database driver reads either on every parse and a stray one would win over
// the mounted password file (ADR-0078, ADR-0139). Run calls it before the configuration is read for
// every program, and process/host calls it while programs still run on the host.
func RefusePasswordVariables(environ []string) error {
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if name == "PGPASSWORD" || name == "PGSSLPASSWORD" {
			return fmt.Errorf("the environment sets %s, and the database password comes only from the mounted password file", name)
		}
	}
	return nil
}

// banner is the start banner, a few lines for a person reading the process's output that name the
// program and its release version, and below them its lines, such as where it serves.
func banner(name string, lines []string) string {
	text := fmt.Sprintf("%s %s\n", name, version.Number)
	for _, l := range lines {
		text += "  " + l + "\n"
	}
	return text
}

func scheme(tls bool) string {
	if tls {
		return "https"
	}
	return "http"
}

// writeBanner writes the start banner. It is no log record, and a failed write stops nothing
// (ADR-0122).
func writeBanner(stderr io.Writer, text string, logger *slog.Logger) {
	if _, err := io.WriteString(stderr, text); err != nil {
		logger.Warn("writing the start banner failed", "error", err)
	}
}
