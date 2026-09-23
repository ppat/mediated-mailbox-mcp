package analysis

import (
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis"
)

// go vet reads another package's variables from export data, where a line directive's file name
// replaces the real one, so a pure core's variable can arrive looking declared in a test file. Only
// the declaring package and its external tests may write a variable a test file declares, so from
// any other package the variable is the pure core's whatever its file is called. The analysis test
// harness loads dependencies from source, where the real name shows, so this case is built by hand.
func TestAVariableNamedInATestFileIsTheCoresOutsideItsPackage(t *testing.T) {
	fset := token.NewFileSet()
	file := fset.AddFile("spoof_test.go", -1, 100)
	core := types.NewPackage(modulePath+"/core/state", "state")
	v := types.NewVar(file.Pos(10), core, "ErrSpoofed", types.Universe.Lookup("error").Type())
	core.Scope().Insert(v)
	cases := []struct {
		name     string
		writer   *types.Package
		reported bool
	}{
		{"another package", types.NewPackage(modulePath+"/mediate", "main"), true},
		{"the declaring package", core, false},
		{"its external tests", types.NewPackage(modulePath+"/core/state_test", "state_test"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := pureCoreVar(&analysis.Pass{Fset: fset, Pkg: c.writer}, v) != nil
			if got != c.reported {
				t.Errorf("reported %v, want %v", got, c.reported)
			}
		})
	}
}
