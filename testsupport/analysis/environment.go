package analysis

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Environment reports a read of the environment outside a deployable's composition root, the rule
// ADR-0078 sets so that every setting passes through the configuration library's layers and
// refusals. It covers the deployables and the shared libraries by covering every package of this
// module outside testsupport, the test tooling. It exempts test files, and a deployable's
// composition root, the file main.go of a package main one directory below the module's root,
// which hands the environment to the library.
//
// The refused functions are every standard-library function that returns an environment
// variable's value by a name its caller gives, or returns the environment as a whole. Those are
// os.Getenv, os.LookupEnv, os.Environ, os.ExpandEnv, syscall.Getenv, syscall.Environ and
// (*exec.Cmd).Environ. os.Expand reads no variable, only calling the function its caller passes, so
// it stays allowed, and a reader passed to it is reported where it is named. Functions that read
// fixed platform variables for their own purpose, such as os.UserHomeDir, os.TempDir and
// http.ProxyFromEnvironment, carry no setting and stay allowed. Any use is reported, a call or a
// function or method value alike, since a value passed on reads the environment wherever it is
// called.
var Environment = &analysis.Analyzer{
	Name: "environment",
	Doc:  "reports a read of the environment outside a deployable's composition root",
	Run:  runEnvironment,
}

const environmentMessage = "%s outside a deployable's composition root, so the setting would bypass the configuration library's layers and refusals. Take it from the configuration the composition root loads (ADR-0078)"

// environmentReaders are the refused functions by their full names, each with the finding's
// opening words.
var environmentReaders = map[string]string{
	"os.Getenv":              "reads the environment with os.Getenv",
	"os.LookupEnv":           "reads the environment with os.LookupEnv",
	"os.Environ":             "reads the environment with os.Environ",
	"os.ExpandEnv":           "reads the environment with os.ExpandEnv",
	"syscall.Getenv":         "reads the environment with syscall.Getenv",
	"syscall.Environ":        "reads the environment with syscall.Environ",
	"(*os/exec.Cmd).Environ": "reads the environment with (*exec.Cmd).Environ",
}

func runEnvironment(pass *analysis.Pass) (any, error) {
	path := pass.Pkg.Path()
	rel, ok := strings.CutPrefix(path, modulePath+"/")
	if path != modulePath && (!ok || rel == "testsupport" || strings.HasPrefix(rel, "testsupport/")) {
		return nil, nil
	}
	root := pass.Pkg.Name() == "main" && ok && !strings.Contains(rel, "/")
	for _, file := range pass.Files {
		// A file not named .go is one the go command generates, such as a test binary's main.
		name := pass.Fset.File(file.Pos()).Name()
		base := filepath.Base(name)
		if !strings.HasSuffix(base, ".go") || strings.HasSuffix(base, "_test.go") || (root && base == "main.go") {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			fn, ok := pass.TypesInfo.Uses[id].(*types.Func)
			if !ok {
				return true
			}
			if opening, refused := environmentReaders[fn.FullName()]; refused {
				pass.Reportf(id.Pos(), environmentMessage, opening)
			}
			return true
		})
	}
	return nil, nil
}
