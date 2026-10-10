package analysis

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Goroutines reports a goroutine started in the worker's code outside its scheduler, the rule ADR-0119
// sets so that a panic in a job stops nothing but its run. The scheduler recovers a panic in a run on
// the run's own goroutine, and Go cannot recover a panic on any other goroutine, so job code starts a
// goroutine only through schedule.Go, which recovers it there and returns it as the run's error. The
// rule covers every package under worker/ outside worker/internal/schedule, the package that holds the
// scheduler and schedule.Go, and exempts test files.
//
// It refuses a go statement, and any use of a function that runs a function it is given on a
// goroutine of its own, a call, a method value and a method expression alike. They are the ones in
// goroutineStarters, from the standard library and from golang.org/x/sync, which this module requires.
// A function is matched by the object it resolves to, so a method promoted from an embedded field is
// refused as the embedded type's own. Three forms are left to review. One is a starter reached through an
// interface value or reflection, which names no function the analyser can resolve. One is a
// dependency whose API calls back a function or a value's method it was given on a goroutine of its
// own, such as a Prometheus collector gathered concurrently, a pgxpool connection hook run by the
// pool's background goroutine, a request body net/http's transport reads, or a reader or writer
// os/exec copies for a command. The last is a function of any other module that starts a goroutine
// running code it is given, since the list names such functions only for golang.org/x/sync.
var Goroutines = &analysis.Analyzer{
	Name: "goroutines",
	Doc:  "reports a goroutine started in the worker's code outside its scheduler",
	Run:  runGoroutines,
}

const (
	workerPath    = modulePath + "/worker"
	schedulerPath = workerPath + "/internal/schedule"
)

// goroutineStarters are the functions, by their full names, that run a function given to them on a
// goroutine of their own.
var goroutineStarters = map[string]bool{
	"(*sync.WaitGroup).Go":                           true,
	"time.AfterFunc":                                 true,
	"context.AfterFunc":                              true,
	"runtime.SetFinalizer":                           true,
	"runtime.AddCleanup":                             true,
	"(*net/http.Server).RegisterOnShutdown":          true,
	"(*golang.org/x/sync/errgroup.Group).Go":         true,
	"(*golang.org/x/sync/errgroup.Group).TryGo":      true,
	"(*golang.org/x/sync/singleflight.Group).DoChan": true,
}

const goroutinesMessage = "starts a goroutine outside schedule.Go, so a panic in it would stop the worker rather than fail the run that raised it. Start it through schedule.Go (ADR-0119)"

func runGoroutines(pass *analysis.Pass) (any, error) {
	path := pass.Pkg.Path()
	if path != workerPath && !strings.HasPrefix(path, workerPath+"/") || path == schedulerPath || strings.HasPrefix(path, schedulerPath+"/") {
		return nil, nil
	}
	for _, file := range pass.Files {
		// A file not named .go is one the go command generates, such as a test binary's main.
		base := filepath.Base(pass.Fset.File(file.Pos()).Name())
		if !strings.HasSuffix(base, ".go") || strings.HasSuffix(base, "_test.go") {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.GoStmt:
				pass.Reportf(n.Pos(), goroutinesMessage)
			case *ast.Ident:
				if fn, ok := pass.TypesInfo.Uses[n].(*types.Func); ok && goroutineStarters[fn.FullName()] {
					pass.Reportf(n.Pos(), goroutinesMessage)
				}
			}
			return true
		})
	}
	return nil, nil
}
