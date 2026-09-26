package check_test

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// componentRoles names the database role each deployable's code runs statements as, keyed by the
// deployable's import list. A deployable's role is mediated_mailbox_ followed by its directory
// (ADR-0075). A shared library connects as no role of its own, so its list has no entry here. Its
// statements run under the role of each deployable whose list admits it, and the grant check plans
// them under each of those roles (ADR-0066). The four deployables here are those that call a provider,
// whose lists admit the rate limiter and the account snapshot library.
var componentRoles = map[string]string{
	"backfill": "mediated_mailbox_backfill",
	"mediate":  "mediated_mailbox_mediate",
	"organize": "mediated_mailbox_organize",
	"sync":     "mediated_mailbox_sync",
}

// testRoles is componentRoles for the test library's import lists. The entry named leftover names no
// list, which TestComponentRolesReported requires to be reported. The entries named narrow and wide
// name lists that name no subsection themselves and admit a library's list that does. The entries
// named coreonly and coreexact name lists admitting only the library's pure core, by a prefix entry
// and by its exact package, which runs none of the library's statements, so they too must be
// reported.
var testRoles = map[string]string{
	"leftover":   "check_fixture_reader",
	"misaligned": "check_fixture_reader",
	"coreexact":  "check_fixture_reader",
	"coreonly":   "check_fixture_reader",
	"narrow":     "check_fixture_reader",
	"reader":     "check_fixture_reader",
	"wide":       "check_fixture_writer",
	"writer":     "check_fixture_writer",
}

// importLists is a library's import lists as the checks read them.
type importLists struct {
	// named holds, per list that names generated subsections, the subsections it names.
	named map[string][]subsection
	// admits holds, per list, the lists that name subsections and govern code the list's allow entries
	// admit. A deployable's list admitting a library's code admits the library's statements.
	admits map[string][]string
}

// admissions reads the library's import lists and returns what each names and admits. Any other entry
// under db/ in a component list is a problem, because a prefix entry would admit every subsection
// whatever the role's grants.
func (lib library) admissions(t *testing.T, subs []subsection) (lists importLists, problems []string) {
	t.Helper()
	var config struct {
		Linters struct {
			Settings struct {
				Depguard struct {
					Rules map[string]struct {
						Files []string `yaml:"files"`
						Allow []string `yaml:"allow"`
					} `yaml:"rules"`
				} `yaml:"depguard"`
			} `yaml:"settings"`
		} `yaml:"linters"`
	}
	readYAML(t, lib.importLists, &config)
	rules := config.Linters.Settings.Depguard.Rules
	if len(rules) == 0 {
		t.Fatalf("%s has no depguard rules", lib.importLists)
	}
	for _, name := range lib.libraryWide {
		if _, ok := rules[name]; !ok {
			problems = append(problems, fmt.Sprintf("list %q is taken as library-wide, and %s does not define it", name, lib.importLists))
		}
	}
	for _, name := range lib.crossCutting {
		if _, ok := rules[name]; !ok {
			problems = append(problems, fmt.Sprintf("list %q is taken as cross-cutting, and %s does not define it", name, lib.importLists))
		}
	}
	lists = importLists{named: map[string][]subsection{}, admits: map[string][]string{}}
	for _, name := range slices.Sorted(maps.Keys(rules)) {
		if slices.Contains(lib.libraryWide, name) {
			continue
		}
		for _, entry := range rules[name].Allow {
			rest, under := strings.CutPrefix(entry, module+"/db/")
			if !under {
				continue
			}
			sub, exact := strings.CutSuffix(rest, "$")
			i := slices.IndexFunc(subs, func(s subsection) bool { return s.name == sub })
			switch {
			case exact && sub == "tx":
			case exact && i >= 0:
				lists.named[name] = append(lists.named[name], subs[i])
			default:
				problems = append(problems, fmt.Sprintf("list %q admits %q. A component list names db/tx$ and generated subsections exactly", name, entry))
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(rules)) {
		if slices.Contains(lib.libraryWide, name) || slices.Contains(lib.crossCutting, name) {
			continue
		}
		for _, other := range slices.Sorted(maps.Keys(lists.named)) {
			if other != name && admitsCode(rules[name].Allow, lib.statementPackages(t, rules[other].Files, lists.named[other])) {
				lists.admits[name] = append(lists.admits[name], other)
			}
		}
	}
	return lists, problems
}

// statementPackages returns the import paths of the packages that run a library's statements. They
// are the packages under the directories the library's list governs, one for each glob of the form
// ${base-path}/<directory>/**.go, whose non-test files import one of the subsections the list names
// or another package that runs them, closed over the library until nothing changes. A package that
// reaches none, such as a library's pure core, runs none of the statements. Violation files are left
// out, since they are never built.
func (lib library) statementPackages(t *testing.T, files []string, subs []subsection) []string {
	t.Helper()
	imports := map[string][]string{}
	for _, glob := range files {
		dir, under := strings.CutPrefix(glob, "${base-path}/")
		dir, whole := strings.CutSuffix(dir, "/**.go")
		if !under || !whole || dir == "" || strings.Contains(dir, "*") {
			continue
		}
		err := walkSkippingTestdata(filepath.Join(lib.root, dir), func(path string, d fs.DirEntry) error {
			name := d.Name()
			if d.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "_violation.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(lib.root, filepath.Dir(path))
			if err != nil {
				return err
			}
			pkg := module + "/" + filepath.ToSlash(rel)
			if _, seen := imports[pkg]; !seen {
				imports[pkg] = nil
			}
			for _, spec := range file.Imports {
				if imported, err := strconv.Unquote(spec.Path.Value); err == nil {
					imports[pkg] = append(imports[pkg], imported)
				}
			}
			return nil
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
	}
	runs := map[string]bool{}
	for _, s := range subs {
		runs[module+"/db/"+s.name] = true
	}
	for changed := true; changed; {
		changed = false
		for pkg, imported := range imports {
			if !runs[pkg] && slices.ContainsFunc(imported, func(i string) bool { return runs[i] }) {
				runs[pkg], changed = true, true
			}
		}
	}
	var out []string
	for pkg := range imports {
		if runs[pkg] {
			out = append(out, pkg)
		}
	}
	return slices.Sorted(slices.Values(out))
}

// admitsCode reports whether an allow entry admits one of the packages. depguard reads an entry
// ending in $ as the exact package and any other entry as a string prefix, and so does this check.
func admitsCode(allow, pkgs []string) bool {
	for _, entry := range allow {
		for _, pkg := range pkgs {
			if exact, ok := strings.CutSuffix(entry, "$"); ok && pkg == exact || !ok && strings.HasPrefix(pkg, entry) {
				return true
			}
		}
	}
	return false
}

// plan is one list's subsections, to be run under one role. For a deployable's list the role is its
// own, and via names the list. For a library's list via names the deployable's list admitting it.
type plan struct {
	list string
	via  string
	role string
	subs []subsection
}

// plans returns every list's subsections under every role that runs them. A list with a role runs its
// subsections under that role. A list without one is a library's, and runs them under the role of
// each list with a role that admits it. A list with neither is skipped, because roleProblems refuses
// it.
func plans(lists importLists, roles map[string]string) []plan {
	var out []plan
	for _, list := range slices.Sorted(maps.Keys(lists.named)) {
		if role, ok := roles[list]; ok {
			out = append(out, plan{list: list, via: list, role: role, subs: lists.named[list]})
			continue
		}
		for _, via := range slices.Sorted(maps.Keys(lists.admits)) {
			if role, ok := roles[via]; ok && slices.Contains(lists.admits[via], list) {
				out = append(out, plan{list: list, via: via, role: role, subs: lists.named[list]})
			}
		}
	}
	return out
}

// roleProblems reports a list naming subsections with no role to test its grants under, neither its
// own nor that of a list admitting it, a list admitting a library's list without a role of its own,
// whose statements would then run under a role nothing tested, and a role entry naming a list that
// neither names subsections nor admits a list that does.
func roleProblems(lists importLists, roles map[string]string) []string {
	var out []string
	for _, name := range slices.Sorted(maps.Keys(lists.admits)) {
		if _, ok := roles[name]; ok {
			continue
		}
		for _, library := range lists.admits[name] {
			if _, ok := roles[library]; !ok {
				out = append(out, fmt.Sprintf("list %q admits the library list %q but has no role, so the library's statements are not tested under it", name, library))
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(lists.named)) {
		if !slices.ContainsFunc(plans(lists, roles), func(p plan) bool { return p.list == name }) {
			out = append(out, fmt.Sprintf("list %q names data-access subsections but has no role", name))
		}
	}
	for _, name := range slices.Sorted(maps.Keys(roles)) {
		if _, ok := lists.named[name]; !ok && len(lists.admits[name]) == 0 {
			out = append(out, fmt.Sprintf("role entry %q names no list that names data-access subsections", name))
		}
	}
	return out
}

func TestComponentRoles(t *testing.T) {
	lists, problems := realLibrary.admissions(t, realLibrary.subsections(t))
	requireNoProblems(t, append(problems, roleProblems(lists, componentRoles)...))
}

func TestComponentRolesReported(t *testing.T) {
	lists, problems := testLibrary.admissions(t, testLibrary.subsections(t))
	requireProblems(t, append(problems, roleProblems(lists, testRoles)...), []string{
		`list "propose" admits the library list "library" but has no role, so the library's statements are not tested under it`,
		`role entry "coreonly" names no list that names data-access subsections`,
		`role entry "coreexact" names no list that names data-access subsections`,
		`list "leaseprefix" admits the library list "library" but has no role, so the library's statements are not tested under it`,
		`list "facadeonly" admits the library list "library" but has no role, so the library's statements are not tested under it`,
		`list "orphan" names data-access subsections but has no role`,
		`list "prefix" admits "github.com/ppat/mediated-mailbox-mcp/db/". A component list names db/tx$ and generated subsections exactly`,
		`list "unmapped" names data-access subsections but has no role`,
		`role entry "leftover" names no list that names data-access subsections`,
	})
}
