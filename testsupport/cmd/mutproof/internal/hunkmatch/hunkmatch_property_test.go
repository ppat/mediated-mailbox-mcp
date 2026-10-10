package hunkmatch_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/mutproof/internal/hunkmatch"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

// The package claims to place every hunk where git apply places it, and git apply is the expectation
// independent of the code under test (ADR-0046). The property below writes small files from a small
// alphabet, so lines repeat often, takes a patch of an edit to them with git diff, moves the files on
// as later work does, and holds the package's reading of the patch to what git apply does with it.
//
// It carries the agreement with git in general, at the gating count and at the scheduled count of
// the go-property-deep workflow. The rule for a last line without a newline decides a case only when
// that line also matches the same text followed by whitespace elsewhere in the file, which this
// generator seldom produces at the gating count, so at the gate that rule rests on the example tests
// in hunkmatch_test.go, which pin it deterministically. The scheduled count reaches it more often.

// genLine is one line of a generated file. Word picks its text from the case's alphabet, Space adds
// a trailing space and Return a carriage return before the newline. A line with Block set stands for
// a copy of the case's block of lines instead, with Space and Return applied to the copy's last line,
// so blocks repeat whole, exactly or apart from a last line's ending.
type genLine struct {
	Word   int
	Space  bool
	Return bool
	Block  bool
}

// genEdit changes a generated file. Kind 0 replaces the line at At, 1 inserts a line before it and 2
// deletes it. At is taken modulo the file's length.
type genEdit struct {
	Kind int
	At   int
	Word int
}

// genFile is one generated file. Edits make the patch, and Drift moves the file on before the patch
// is applied.
type genFile struct {
	Lines          []genLine
	NoFinalNewline bool
	Edits          []genEdit
	Drift          []genEdit
}

// gitCase is one generated case, one or two files, the block their copies repeat, and the lines of
// context the patch is taken with.
type gitCase struct {
	Files    []genFile
	Block    []int
	Context  int
	Alphabet int
}

func drawEdit(alphabet int) *rapid.Generator[genEdit] {
	return rapid.Custom(func(t *rapid.T) genEdit {
		return genEdit{
			Kind: rapid.IntRange(0, 2).Draw(t, "kind"),
			At:   rapid.IntRange(0, 15).Draw(t, "at"),
			Word: rapid.IntRange(0, alphabet-1).Draw(t, "word"),
		}
	})
}

func drawCase(t *rapid.T) gitCase {
	alphabet := rapid.IntRange(2, 3).Draw(t, "alphabet")
	line := rapid.Custom(func(t *rapid.T) genLine {
		return genLine{
			Word:   rapid.IntRange(0, alphabet-1).Draw(t, "word"),
			Space:  rapid.IntRange(0, 7).Draw(t, "space") == 0,
			Return: rapid.IntRange(0, 7).Draw(t, "return") == 0,
			Block:  rapid.IntRange(0, 1).Draw(t, "block") == 0,
		}
	})
	file := rapid.Custom(func(t *rapid.T) genFile {
		return genFile{
			Lines:          rapid.SliceOfN(line, 3, 12).Draw(t, "lines"),
			NoFinalNewline: rapid.IntRange(0, 1).Draw(t, "no final newline") == 0,
			Edits:          rapid.SliceOfN(drawEdit(alphabet), 1, 2).Draw(t, "edits"),
			Drift:          rapid.SliceOfN(drawEdit(alphabet), 0, 2).Draw(t, "drift"),
		}
	})
	return gitCase{
		Files:    rapid.SliceOfN(file, 1, 2).Draw(t, "files"),
		Block:    rapid.SliceOfN(rapid.IntRange(0, alphabet-1), 2, 5).Draw(t, "block"),
		Context:  rapid.IntRange(0, 4).Draw(t, "context"),
		Alphabet: alphabet,
	}
}

// render writes a file's lines, each ending in a newline but the last when the file has none.
func render(lines []string, finalNewline bool) string {
	s := strings.Join(lines, "\n")
	if finalNewline {
		s += "\n"
	}
	return s
}

func baseLines(c gitCase, f genFile) []string {
	ending := func(s string, l genLine) string {
		if l.Space {
			s += " "
		}
		if l.Return {
			s += "\r"
		}
		return s
	}
	var out []string
	for _, l := range f.Lines {
		if !l.Block {
			out = append(out, ending("w"+strconv.Itoa(l.Word), l))
			continue
		}
		for i, w := range c.Block {
			s := "w" + strconv.Itoa(w)
			if i == len(c.Block)-1 {
				s = ending(s, l)
			}
			out = append(out, s)
		}
	}
	return out
}

func applyEdits(lines []string, edits []genEdit) []string {
	out := append([]string(nil), lines...)
	for _, e := range edits {
		switch e.Kind {
		case 0:
			out[e.At%len(out)] = "m" + strconv.Itoa(e.Word)
		case 1:
			at := e.At % (len(out) + 1)
			out = append(out[:at], append([]string{"i" + strconv.Itoa(e.Word)}, out[at:]...)...)
		default:
			if len(out) > 1 {
				at := e.At % len(out)
				out = append(out[:at], out[at+1:]...)
			}
		}
	}
	return out
}

func fileName(i int) string { return fmt.Sprintf("f%d.txt", i) }

// gitIn runs git in dir and ends the case when it fails.
func gitIn(t rapid.TB, dir string, args ...string) string {
	out, err := runGit(dir, args...)
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func write(t rapid.TB, path, content string) {
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// hunkHeader matches a hunk header's two ranges, so a test can point the header at another line.
var hunkHeader = regexp.MustCompile(`(?m)^@@ -\d+((?:,\d+)?) \+\d+((?:,\d+)?) @@`)

// Differential testing against a second implementation of the same rule, git apply (ADR-0055), over
// every generated case that git apply --check accepts. The package finds a
// place for every hunk, and applying each hunk at the place it reads as git's leaves exactly the files
// git apply leaves. When the patch is one hunk the package finds at more than one place, git apply
// takes each of those places once the hunk's header names it, so every match the package reports is
// one git could use.
func TestHunkMatchAgreesWithGitApply(t *testing.T) {
	property.Check(t, drawCase, func(t rapid.TB, c gitCase) {
		dir, err := os.MkdirTemp("", "hunkmatch-")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Error(err)
			}
		}()
		gitIn(t, dir, "init", "--quiet")
		gitIn(t, dir, "config", "core.autocrlf", "false")
		drifted := map[string]string{}
		for i, f := range c.Files {
			base := baseLines(c, f)
			write(t, filepath.Join(dir, fileName(i)), render(base, !f.NoFinalNewline))
			drifted[fileName(i)] = render(applyEdits(base, f.Drift), !f.NoFinalNewline)
		}
		gitIn(t, dir, "add", "--all")
		for i, f := range c.Files {
			write(t, filepath.Join(dir, fileName(i)), render(applyEdits(baseLines(c, f), f.Edits), !f.NoFinalNewline))
		}
		diff := gitIn(t, dir, "diff", "-U"+strconv.Itoa(c.Context))
		if diff == "" {
			return
		}
		patch := "control: a control\n\n" + diff
		patchPath := filepath.Join(dir, ".git", "p.patch")
		restore := func() {
			for name, content := range drifted {
				write(t, filepath.Join(dir, name), content)
			}
		}
		restore()
		write(t, patchPath, patch)
		if _, err := runGit(dir, "apply", "--check", patchPath); err != nil {
			return
		}

		hunks, files, err := hunkmatch.Place(dir, []byte(patch))
		if err != nil {
			t.Fatalf("the package cannot read a patch git applies: %v\n%s", err, patch)
		}
		for _, h := range hunks {
			if len(h.Matches) == 0 {
				t.Fatalf("%s: hunk %d (%s) matches nowhere for the package, and git applies it\n%s\nfiles %q", h.Path, h.Number, h.Header, patch, drifted)
			}
		}
		requireSameAsGit := func(patch string, files map[string]string) {
			write(t, patchPath, patch)
			gitIn(t, dir, "apply", patchPath)
			got := map[string]string{}
			for name := range files {
				data, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				got[name] = string(data)
			}
			restore()
			if d := cmp.Diff(got, files, compare.Options); d != "" {
				t.Fatalf("the files git apply leaves (-) and those the package reads the patch as leaving (+):\n%s\npatch:\n%s\nfiles %q", d, patch, drifted)
			}
		}
		requireSameAsGit(patch, files)

		if len(hunks) != 1 || len(hunks[0].Matches) < 2 {
			return
		}
		for _, m := range hunks[0].Matches {
			moved := hunkHeader.ReplaceAllString(patch, fmt.Sprintf("@@ -%d$1 +%d$2 @@", m, m))
			pointed, files, err := hunkmatch.Place(dir, []byte(moved))
			if err != nil {
				t.Fatal(err)
			}
			if pointed[0].Chosen != m {
				t.Fatalf("with its header at line %d the package reads git as taking line %d\n%s", m, pointed[0].Chosen, moved)
			}
			requireSameAsGit(moved, files)
		}
	})
}

// caseKind classifies a generated case by whether a line of it ends in a carriage return or a trailing
// space, which only the byte comparison of line endings tells apart from a line without one.
func caseKind(c gitCase) string {
	for _, f := range c.Files {
		for _, l := range f.Lines {
			if l.Return || l.Space {
				return "a carriage return or trailing space"
			}
		}
	}
	return "neither"
}

// The generator report for the property above. A line ending in a carriage return or a trailing space
// needs a share the gating count reaches many times over, since the comparison of line endings rests
// on the property at the gate as well as on its example test.
func TestHunkMatchAgreesWithGitApplyMix(t *testing.T) {
	property.Report(t, drawCase, caseKind, map[string]float64{
		"a carriage return or trailing space": 0.15,
	})
}
