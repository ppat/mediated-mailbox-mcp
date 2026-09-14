//go:build banproof

package sensitivity

// This file breaks the pure-core import list on purpose, and banproof requires every want below.
//
// regexp/syntax proves the standard-library entries are exact. depguard compares an import with
// only the allow entry sorted just before it, and for regexp/syntax that entry is regexp, so the
// import is admitted if regexp loses its $. math/rand would not prove it, because its sorted
// neighbour is math/bits. go-cmp is refused by two lists on one line, which proves uniq-by-line is
// off. It is a third-party package rather than a project one, because an import of a project package
// here would become an import cycle once that package imports this one.
import (
	_ "math/rand"     // want depguard "import 'math/rand' is not allowed from list 'pure-core'"
	_ "os/exec"       // want depguard "import 'os/exec' is not allowed from list 'pure-core'"
	_ "regexp/syntax" // want depguard "import 'regexp/syntax' is not allowed from list 'pure-core'"

	_ "github.com/google/go-cmp/cmp" // want depguard "list 'pure-core'" depguard "list 'non-test-code'"
)
