package check_test

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
)

// importLists holds, per import list that names generated subsections, the subsections it names.
type importLists map[string][]subsection

// admissions reads the library's import lists and returns the subsections each names. A component list
// may name, beside generated subsections, the transaction helper and the migration chain with the
// package that applies it, none of which holds a statement of its own. Any other entry under db/ in a
// component list is a problem, because a prefix entry would admit every subsection, and the grant
// check, which plans the subsections a list names, would plan none of those it admits.
func (lib library) admissions(t *testing.T, subs []subsection) (lists importLists, problems []string) {
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
	for _, name := range lib.crossCutting {
		if _, ok := rules[name]; !ok {
			problems = append(problems, fmt.Sprintf("list %q is taken as cross-cutting, and %s does not define it", name, lib.importLists))
		}
	}
	lists = importLists{}
	for _, name := range slices.Sorted(maps.Keys(rules)) {
		if slices.Contains(lib.libraryWide, name) {
			continue
		}
		for _, entry := range rules[name].Allow {
			if admitsRoot(entry) && !slices.Contains(lib.crossCutting, name) {
				problems = append(problems, fmt.Sprintf("list %q admits %q, which admits the data-access library's root package. A component list admits only subsections", name, entry))
				continue
			}
			rest, under := strings.CutPrefix(entry, module+"/db/")
			if !under {
				continue
			}
			sub, exact := strings.CutSuffix(rest, "$")
			i := slices.IndexFunc(subs, func(s subsection) bool { return s.name == sub })
			switch {
			case exact && (sub == "tx" || sub == "migrate" || sub == "migrations"):
			case exact && i >= 0:
				lists[name] = append(lists[name], subs[i])
			default:
				problems = append(problems, fmt.Sprintf("list %q admits %q. A component list names db/tx$, db/migrate$, db/migrations$ and generated subsections exactly", name, entry))
			}
		}
	}
	return lists, problems
}

// admitsRoot reports whether an allow entry admits the data-access library's root package, by its
// exact path or by a prefix of it. The root package holds no statements, so a file there that called
// a subsection's statements would reach them for every list admitting the root, and the grant check,
// which plans only subsections a list names, would never plan them. A cross-cutting list may admit it
// by a wide prefix, since every file it governs is also held to its component's own list.
func admitsRoot(entry string) bool {
	root := module + "/db"
	if exact, ok := strings.CutSuffix(entry, "$"); ok {
		return exact == root
	}
	return strings.Contains(entry, "/") && strings.HasPrefix(root, entry)
}

func TestImportListEntries(t *testing.T) {
	_, problems := realLibrary.admissions(t, realLibrary.subsections(t))
	requireNoProblems(t, problems)
}

func TestImportListEntriesReported(t *testing.T) {
	_, problems := testLibrary.admissions(t, testLibrary.subsections(t))
	requireProblems(t, problems, []string{
		`list "prefix" admits "github.com/ppat/mediated-mailbox-mcp/db/". A component list names db/tx$, db/migrate$, db/migrations$ and generated subsections exactly`,
		`list "rootpackage" admits "github.com/ppat/mediated-mailbox-mcp/db$", which admits the data-access library's root package. A component list admits only subsections`,
		`list "rootpackage" admits "github.com/ppat/mediated-mailbox-mcp/d", which admits the data-access library's root package. A component list admits only subsections`,
	})
}
