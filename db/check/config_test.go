package check_test

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
)

// componentRoles names the database role each component's code runs statements as, keyed by the
// component's import list. It is empty because no component list names a generated subsection yet.
// How many runtime roles exist and which component runs as which is not decided. An entry arrives with
// the first list naming a subsection, and TestComponentRoles refuses either without the other.
var componentRoles = map[string]string{}

// testRoles is componentRoles for the test library's import lists. The entry named leftover names no
// list, which TestComponentRolesReported requires to be reported.
var testRoles = map[string]string{
	"leftover":   "check_fixture_reader",
	"misaligned": "check_fixture_reader",
	"reader":     "check_fixture_reader",
	"writer":     "check_fixture_writer",
}

// admissions reads the library's import lists and returns, per component list, the generated
// subsections it names. Any other entry under db/ in a component list is a problem, because a prefix
// entry would admit every subsection whatever the role's grants.
func (lib library) admissions(t *testing.T, subs []subsection) (admitted map[string][]subsection, problems []string) {
	t.Helper()
	var config struct {
		Linters struct {
			Settings struct {
				Depguard struct {
					Rules map[string]struct {
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
	admitted = map[string][]subsection{}
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
				admitted[name] = append(admitted[name], subs[i])
			default:
				problems = append(problems, fmt.Sprintf("list %q admits %q. A component list names db/tx$ and generated subsections exactly", name, entry))
			}
		}
	}
	return admitted, problems
}

// roleProblems reports a list naming subsections without a role to test its grants under, and a role
// entry naming no such list.
func roleProblems(admitted map[string][]subsection, roles map[string]string) []string {
	var out []string
	for _, name := range slices.Sorted(maps.Keys(admitted)) {
		if _, ok := roles[name]; !ok {
			out = append(out, fmt.Sprintf("list %q names data-access subsections but has no role", name))
		}
	}
	for _, name := range slices.Sorted(maps.Keys(roles)) {
		if _, ok := admitted[name]; !ok {
			out = append(out, fmt.Sprintf("role entry %q names no list that names data-access subsections", name))
		}
	}
	return out
}

func TestComponentRoles(t *testing.T) {
	admitted, problems := realLibrary.admissions(t, realLibrary.subsections(t))
	requireNoProblems(t, append(problems, roleProblems(admitted, componentRoles)...))
}

func TestComponentRolesReported(t *testing.T) {
	admitted, problems := testLibrary.admissions(t, testLibrary.subsections(t))
	requireProblems(t, append(problems, roleProblems(admitted, testRoles)...), []string{
		`list "prefix" admits "github.com/ppat/mediated-mailbox-mcp/db/". A component list names db/tx$ and generated subsections exactly`,
		`list "unmapped" names data-access subsections but has no role`,
		`role entry "leftover" names no list that names data-access subsections`,
	})
}
