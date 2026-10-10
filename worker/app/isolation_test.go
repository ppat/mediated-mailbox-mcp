package app_test

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// unsafeImporters are the packages outside the standard library in the worker's build that import
// unsafe or hold assembly or a linked object file (ADR-0117). Assembly and an object file reach memory
// as unsafe does, with no import of it. The list changes only by review, since a package that joins it,
// a new dependency or a version of one already held, steps outside the memory safety code isolation
// per job kind rests on, and no package of this project may ever be on it.
var unsafeImporters = []string{
	"github.com/JohannesKaufmann/html-to-markdown/v2/collapse",
	"github.com/cespare/xxhash/v2",
	"github.com/jackc/puddle/v2",
	"golang.org/x/sys/unix",
	"google.golang.org/protobuf/internal/impl",
	"google.golang.org/protobuf/internal/protolazy",
	"google.golang.org/protobuf/internal/strs",
	"google.golang.org/protobuf/reflect/protoreflect",
	"google.golang.org/protobuf/types/known/timestamppb",
}

// Code isolation per job kind rests on memory safety, so no package outside the standard library that
// the worker's image links imports unsafe, or holds assembly or a linked object file, beyond the
// dependencies the module already held, on either architecture the image is built for (ADR-0117). The
// build is listed as the image builds it, without cgo.
func TestNoDependencyAddedForAJobImportsUnsafe(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), "go", "list", "-deps",
				"-f", "{{if not .Standard}}{{.ImportPath}}{{range .Imports}} {{.}}{{end}}{{if or .SFiles .SysoFiles}} unsafe{{end}}{{end}}", "..")
			cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+arch, "GOFLAGS=")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("listing the worker's build: %v\n%s", err, stderr.String())
			}
			var got []string
			for line := range strings.Lines(string(out)) {
				fields := strings.Fields(line)
				if len(fields) > 1 && slices.Contains(fields[1:], "unsafe") {
					got = append(got, fields[0])
				}
			}
			slices.Sort(got)
			if d := cmp.Diff(unsafeImporters, got, compare.Options); d != "" {
				t.Errorf("the packages of the worker's build that import unsafe or hold assembly (-allowed +built):\n%s", d)
			}
		})
	}
}

// The worker's image builds its binary without cgo, which code isolation per job kind needs alongside
// the rule on unsafe (ADR-0117). The image's Dockerfile is where the build that ships is defined.
func TestTheWorkerBuildsWithoutCgo(t *testing.T) {
	src, err := os.ReadFile("../Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	builds := regexp.MustCompile(`(?m)^RUN (.*)\bgo build\b.*$`).FindAllStringSubmatch(string(src), -1)
	if len(builds) != 1 {
		t.Fatalf("the worker's Dockerfile has %d go build steps, want 1", len(builds))
	}
	if env := strings.Fields(builds[0][1]); !slices.Contains(env, "CGO_ENABLED=0") {
		t.Fatalf("the worker's image builds with %q before go build, which does not switch cgo off", builds[0][1])
	}
}

// Each job kind's entry constructor takes only what the job kind needs, one value whose fields are
// pinned, so a job kind cannot gain an input, another job kind's pool or the scheduler's whole surface
// among them, unnoticed (ADR-0117). Each kind is handed its own pool, and the jobs it adds and drops
// through an interface of the scheduler's two calls it uses.
func TestEachJobKindsConstructorTakesOnlyWhatItNeeds(t *testing.T) {
	const work = "github.com/ppat/mediated-mailbox-mcp/worker/internal/"
	mustnotcompile.RequireParams(t, work+"backfill", "New", "Config")
	mustnotcompile.RequireFields(t, work+"backfill", "Config",
		"Pool *pgxpool.Pool", "Keys *open.Keyring", "Scanner scan.Scanner", "Client *http.Client",
		"Registry prometheus.Registerer", "Logger *slog.Logger", "Jobs Jobs", "Concurrency int", "ReloadInterval time.Duration")
	mustnotcompile.RequireMethods(t, work+"backfill", "Jobs",
		"Ensure func(key schedule.Key, job schedule.Job) error", "Remove func(key schedule.Key)")
	mustnotcompile.RequireParams(t, work+"deltasync", "New", "Config")
	mustnotcompile.RequireFields(t, work+"deltasync", "Config",
		"Pool *pgxpool.Pool", "Keys *open.Keyring", "Scanner scan.Scanner", "Client *http.Client",
		"Registry prometheus.Registerer", "Logger *slog.Logger", "Jobs Jobs", "Concurrency int", "ReloadInterval time.Duration",
		"SyncInterval time.Duration", "FirstWindow time.Duration", "DecisionsPerTick int")
	mustnotcompile.RequireMethods(t, work+"deltasync", "Jobs",
		"Ensure func(key schedule.Key, job schedule.Job) error", "Remove func(key schedule.Key)")
}
