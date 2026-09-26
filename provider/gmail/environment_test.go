package gmail_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
)

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

// F6's part of VERIFICATIONS' row for a credential supplied outside the account's state row. With a
// value in every candidate variable, a source built from the values the deployable supplies holds
// those values alone, an empty refresh token included (ADR-0080).
func TestTheSourceHoldsOnlyTheCredentialItIsGiven(t *testing.T) {
	setCredentialVariables(t)
	for name, token := range map[string]string{"a refresh token": "given-refresh-token", "no refresh token": ""} {
		t.Run(name, func(t *testing.T) {
			src := gmail.NewTokenSource(http.DefaultClient, gmail.Credentials{ClientID: "given-id", ClientSecret: "given-secret", RefreshToken: token})
			if got := src.RefreshToken(); got != token {
				t.Errorf("the source holds refresh token %q, want %q", got, token)
			}
		})
	}
}
