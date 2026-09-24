package gmail_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// mount writes each value to its own file in a new directory, the way the platform mounts a
// secret, and returns the file paths.
func mount(t *testing.T, clientID, clientSecret, refreshToken string) gmail.Files {
	t.Helper()
	dir := t.TempDir()
	files := gmail.Files{
		ClientID:     filepath.Join(dir, "client-id"),
		ClientSecret: filepath.Join(dir, "client-secret"),
		RefreshToken: filepath.Join(dir, "refresh-token"),
	}
	for path, value := range map[string]string{
		files.ClientID:     clientID,
		files.ClientSecret: clientSecret,
		files.RefreshToken: refreshToken,
	} {
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return files
}

// dirEntries lists the names in dir.
func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestLoadReadsTheMountedFiles(t *testing.T) {
	files := mount(t, "the-client-id\n", "the-client-secret\n", "the-refresh-token\n")
	got, err := gmail.Load(files)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := gmail.Credentials{
		ClientID:     "the-client-id",
		ClientSecret: "the-client-secret",
		RefreshToken: "the-refresh-token",
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("Load (-want +got):\n%s", diff)
	}
}

func TestLoadRefusesAMissingOrEmptyFile(t *testing.T) {
	cases := []struct {
		name   string
		damage func(*gmail.Files)
	}{
		{"missing file", func(f *gmail.Files) { f.ClientSecret = filepath.Join(filepath.Dir(f.ClientSecret), "absent") }},
		{"empty path", func(f *gmail.Files) { f.ClientID = "" }},
		{"empty file", func(f *gmail.Files) {
			if err := os.WriteFile(f.RefreshToken, nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"whitespace only", func(f *gmail.Files) {
			if err := os.WriteFile(f.RefreshToken, []byte(" \n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := mount(t, "id", "secret", "token")
			c.damage(&files)
			got, err := gmail.Load(files)
			if err == nil {
				t.Fatalf("Load succeeded with %+v", got)
			}
			if diff := cmp.Diff(gmail.Credentials{}, got, compare.Options); diff != "" {
				t.Errorf("Load returned credentials alongside its error (-want +got):\n%s", diff)
			}
		})
	}
}

// A write-back replaces the location's content whole, readable only by its owner, and leaves no
// other file beside it.
func TestWriteBackReplacesTheLocationWhole(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "refresh-token")
	if err := os.WriteFile(path, []byte("a-longer-previous-refresh-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := gmail.WriteBack(path, "rotated-token"); err != nil {
		t.Fatalf("WriteBack: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "rotated-token" {
		t.Errorf("location holds %q, want %q", got, "rotated-token")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("location mode = %v, want -rw-------", perm)
	}
	if diff := cmp.Diff([]string{"refresh-token"}, dirEntries(t, dir), compare.Options); diff != "" {
		t.Errorf("directory entries (-want +got):\n%s", diff)
	}
}

func TestWriteBackRefusesAnEmptyToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "refresh-token")
	if err := gmail.WriteBack(path, ""); err == nil {
		t.Fatal("WriteBack wrote an empty token")
	}
	if names := dirEntries(t, dir); len(names) != 0 {
		t.Errorf("WriteBack left %q behind", names)
	}
}

// Environment variable names a credential could plausibly be read from, including the ones a
// reading of this package's own names suggests. No list of names can prove the environment is
// never read, so TestNoSourceFileReadsTheEnvironment carries the general case.
var credentialVariables = []string{
	"CLIENT_ID", "CLIENT_SECRET", "REFRESH_TOKEN",
	"GMAIL_CLIENT_ID", "GMAIL_CLIENT_SECRET", "GMAIL_REFRESH_TOKEN",
	"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REFRESH_TOKEN",
	"GOOGLE_OAUTH_CLIENT_ID", "GOOGLE_OAUTH_CLIENT_SECRET", "GOOGLE_OAUTH_REFRESH_TOKEN",
	"OAUTH_CLIENT_ID", "OAUTH_CLIENT_SECRET", "OAUTH_REFRESH_TOKEN",
}

// setCredentialVariables puts a value in every candidate variable, a different one per name.
func setCredentialVariables(t *testing.T) {
	t.Helper()
	for _, name := range credentialVariables {
		t.Setenv(name, "from-environment-"+name)
	}
}

// With the credentials in the environment and one mounted file absent, Load fails rather than
// reading that value from the environment (ADR-0038). Each file is taken away alone, so a fallback
// for any one of them shows.
func TestLoadIgnoresTheEnvironmentWhenAFileIsAbsent(t *testing.T) {
	for name, pick := range map[string]func(gmail.Files) string{
		"client ID":     func(f gmail.Files) string { return f.ClientID },
		"client secret": func(f gmail.Files) string { return f.ClientSecret },
		"refresh token": func(f gmail.Files) string { return f.RefreshToken },
	} {
		t.Run(name, func(t *testing.T) {
			setCredentialVariables(t)
			files := mount(t, "id", "secret", "token")
			if err := os.Remove(pick(files)); err != nil {
				t.Fatal(err)
			}
			got, err := gmail.Load(files)
			if err == nil {
				t.Fatalf("Load succeeded with the %s file absent, returning %+v", name, got)
			}
			if diff := cmp.Diff(gmail.Credentials{}, got, compare.Options); diff != "" {
				t.Errorf("Load returned credentials alongside its error (-want +got):\n%s", diff)
			}
		})
	}
}

// With the credentials in the environment and in the mounted files, Load returns the files'
// values.
func TestLoadIgnoresTheEnvironmentWhenTheFilesArePresent(t *testing.T) {
	setCredentialVariables(t)
	got, err := gmail.Load(mount(t, "file-client-id", "file-client-secret", "file-refresh-token"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := gmail.Credentials{ //nolint:gosec // test values, not credentials
		ClientID:     "file-client-id",
		ClientSecret: "file-client-secret",
		RefreshToken: "file-refresh-token",
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("Load (-want +got):\n%s", diff)
	}
}

// environmentReaders are the functions, by import path, that return environment variables. Other
// ways of reading the environment, such as the environ file under /proc or a command's Environ,
// are left to review.
var environmentReaders = map[string][]string{
	"golang.org/x/sys/unix": {"Environ", "Getenv"},
	"os":                    {"Environ", "ExpandEnv", "Getenv", "LookupEnv"},
	"syscall":               {"Environ", "Getenv"},
}

// No non-test file of this package calls a function that reads the environment, under whatever
// name its package is imported, dot imports included. A lint ban cannot carry this rule, because
// forbidigo matches in every file of the module and ADR-0051 lets a composition root read its
// configuration from the environment, while banproof refuses the exclusion rule that would narrow
// the ban to this package.
func TestNoSourceFileReadsTheEnvironment(t *testing.T) {
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	fset := token.NewFileSet()
	for _, name := range sources {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		imported := map[string]string{}
		var dotted []string
		for _, spec := range f.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			local := path[strings.LastIndex(path, "/")+1:]
			if spec.Name != nil {
				local = spec.Name.Name
			}
			if local == "." {
				dotted = append(dotted, path)
				continue
			}
			imported[local] = path
		}
		report := func(pos token.Pos, path, name string) {
			if slices.Contains(environmentReaders[path], name) {
				found = append(found, fmt.Sprintf("%s: %s.%s", fset.Position(pos), path, name))
			}
		}
		// A selector's name is never a reference to a dot-imported function, so the walk reports
		// the selector as a whole, then walks its receiver and skips its name. Every other node is
		// walked in full.
		var visit func(ast.Node) bool
		visit = func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if pkg, ok := n.X.(*ast.Ident); ok {
					report(n.Sel.Pos(), imported[pkg.Name], n.Sel.Name)
				}
				ast.Inspect(n.X, visit)
				return false
			case *ast.Ident:
				for _, path := range dotted {
					report(n.Pos(), path, n.Name)
				}
			}
			return true
		}
		ast.Inspect(f, visit)
	}
	if len(found) != 0 {
		t.Errorf("source files read the environment:\n%s", strings.Join(found, "\n"))
	}
}

// A write-back first removes any new file an earlier write-back left beside the location, and
// leaves every other file alone. A new file younger than ten minutes may be another deployable's
// write in progress, so it stays.
func TestWriteBackRemovesANewFileLeftBesideTheLocation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "refresh-token")
	for name, content := range map[string]string{
		".refresh-token.1234567": "live-token-from-a-crash",
		".refresh-token.7654321": "another-deployables-write-in-progress",
		"refresh-token-notes":    "unrelated",
		".other.1234567":         "unrelated",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-11 * time.Minute)
	for _, name := range []string{".refresh-token.1234567", "refresh-token-notes", ".other.1234567"} {
		if err := os.Chtimes(filepath.Join(dir, name), old, old); err != nil {
			t.Fatal(err)
		}
	}
	if err := gmail.WriteBack(path, "rotated-token"); err != nil {
		t.Fatalf("WriteBack: %v", err)
	}
	want := []string{".other.1234567", ".refresh-token.7654321", "refresh-token", "refresh-token-notes"}
	if diff := cmp.Diff(want, dirEntries(t, dir), compare.Options); diff != "" {
		t.Errorf("directory entries (-want +got):\n%s", diff)
	}
}
