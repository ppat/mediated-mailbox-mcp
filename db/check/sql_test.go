package check_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	pg "github.com/pganalyze/pg_query_go/v6"
	pgquery "github.com/wasilibs/go-pgquery"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// finding is one thing a check refuses, located in a SQL file.
type finding struct {
	file  string
	line  int
	check string
	text  string
}

func (f finding) String() string {
	return fmt.Sprintf("%s:%d: %s: %s", f.file, f.line, f.check, f.text)
}

// sqlFile is a parsed SQL file. Node locations are byte offsets into src.
type sqlFile struct {
	path  string
	src   string
	stmts []*pg.RawStmt
}

func parseFile(t *testing.T, path string) sqlFile {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := pgquery.Parse(string(src))
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return sqlFile{path: path, src: string(src), stmts: tree.GetStmts()}
}

// sqlFiles lists the .sql files directly in dir, sorted.
func sqlFiles(t *testing.T, dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(paths)
	return paths
}

// at builds a finding at a byte offset. A negative offset, which the parser uses for "unknown", falls
// back to the statement's first line that is not blank or a comment.
func (f sqlFile) at(stmt *pg.RawStmt, offset int32, check, format string, args ...any) finding {
	pos := int(offset)
	if pos < 0 {
		pos = int(stmt.GetStmtLocation())
		rest := f.src[pos:]
		for {
			trimmed := strings.TrimLeft(rest, " \t\r\n")
			if !strings.HasPrefix(trimmed, "--") {
				pos += len(rest) - len(trimmed)
				break
			}
			end := strings.IndexByte(trimmed, '\n')
			if end < 0 {
				break
			}
			pos += len(rest) - len(trimmed) + end + 1
			rest = trimmed[end+1:]
		}
	}
	return finding{
		file:  f.path,
		line:  strings.Count(f.src[:pos], "\n") + 1,
		check: check,
		text:  fmt.Sprintf(format, args...),
	}
}

// walk visits every node under root, depth first, root included. visit returns false to skip a
// node's children.
func walk(root *pg.Node, visit func(*pg.Node) bool) {
	if root == nil || root.GetNode() == nil || !visit(root) {
		return
	}
	descend(root.ProtoReflect(), visit)
}

func descend(m protoreflect.Message, visit func(*pg.Node) bool) {
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsMap() || fd.Message() == nil:
		case fd.IsList():
			for i := range v.List().Len() {
				visitMessage(v.List().Get(i).Message(), visit)
			}
		default:
			visitMessage(v.Message(), visit)
		}
		return true
	})
}

// visitMessage visits m when it is a node, and descends into it unless the visit declines.
func visitMessage(m protoreflect.Message, visit func(*pg.Node) bool) {
	if n, ok := m.Interface().(*pg.Node); ok && (n.GetNode() == nil || !visit(n)) {
		return
	}
	descend(m, visit)
}

// names returns the string values of a list of String nodes, such as a qualified name.
func names(list []*pg.Node) []string {
	var out []string
	for _, n := range list {
		if s := n.GetString_(); s != nil {
			out = append(out, s.GetSval())
		}
	}
	return out
}

// isParameter reports whether n is a statement parameter in any spelling sqlc accepts, which is $1,
// @name, sqlc.arg(name), sqlc.narg(name) or sqlc.slice(name). The parser reads @name as the prefix
// operator @ applied to a column reference.
func isParameter(n *pg.Node) bool {
	switch {
	case n.GetParamRef() != nil:
		return true
	case n.GetFuncCall() != nil:
		fn := names(n.GetFuncCall().GetFuncname())
		return len(fn) == 2 && fn[0] == "sqlc" && slices.Contains([]string{"arg", "narg", "slice"}, fn[1])
	case n.GetAExpr() != nil:
		e := n.GetAExpr()
		return e.GetKind() == pg.A_Expr_Kind_AEXPR_OP && e.GetLexpr() == nil && slices.Equal(names(e.GetName()), []string{"@"})
	}
	return false
}

// isCastParameter reports whether n is a parameter under a type cast. @name::text parses as @ applied
// to name::text, because the cast binds tighter than the prefix operator, so both shapes are cast
// parameters.
func isCastParameter(n *pg.Node) bool {
	if c := n.GetTypeCast(); c != nil {
		return isParameter(c.GetArg())
	}
	if e := n.GetAExpr(); e != nil && isParameter(n) {
		return e.GetRexpr().GetTypeCast() != nil
	}
	return false
}

// containsParameter reports whether n or any node under it is a parameter.
func containsParameter(n *pg.Node) bool {
	found := false
	walk(n, func(node *pg.Node) bool {
		found = found || isParameter(node)
		return !found
	})
	return found
}

// wantPattern matches a violation file's annotation, which names one check the file must fail.
var wantPattern = regexp.MustCompile(`(?m)^--\s*want\s+([a-z-]+)\s*$`)

func wants(src string) []string {
	var out []string
	for _, m := range wantPattern.FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	return out
}
