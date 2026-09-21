package main

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

const diffHeader = "diff --git a/p/p.go b/p/p.go\n--- a/p/p.go\n+++ b/p/p.go\n"

const validPreamble = "control: A control\nremoves: How it is removed\npackages: ./p ./q\ntests: TestA TestB/sub\nscheduled-count: yes\n\n"

func TestParsePatchReadsThePreamble(t *testing.T) {
	p, err := parsePatch("p/testdata/mutations/a.patch", []byte(validPreamble+diffHeader))
	if err != nil {
		t.Fatal(err)
	}
	type view struct {
		Control, Removes string
		Packages, Tests  []string
		Scheduled        bool
	}
	got := view{p.control, p.removes, p.packages, p.tests, p.scheduled}
	want := view{"A control", "How it is removed", []string{"./p", "./q"}, []string{"TestA", "TestB/sub"}, true}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("preamble (-want +got):\n%s", diff)
	}
}

func TestParsePatchRefusesMalformedPatches(t *testing.T) {
	cases := []struct {
		name, path, src, wantErr string
	}{
		{"outside testdata/mutations", "p/mutations/a.patch", validPreamble + diffHeader, "a .patch file directly in a testdata/mutations directory"},
		{"unknown key", "p/testdata/mutations/a.patch", "owner: me\n" + validPreamble + diffHeader, "holds only the keys"},
		{"free prose", "p/testdata/mutations/a.patch", "This patch removes the check.\n" + validPreamble + diffHeader, "holds only the keys"},
		{"key given twice", "p/testdata/mutations/a.patch", "tests: TestC\n" + validPreamble + diffHeader, "tests is given twice"},
		{"key with no value", "p/testdata/mutations/a.patch", strings.Replace(validPreamble, "TestA TestB/sub", "", 1) + diffHeader, "tests has no value"},
		{"missing key", "p/testdata/mutations/a.patch", strings.Replace(validPreamble, "removes: How it is removed\n", "", 1) + diffHeader, "the preamble lacks removes"},
		{"not a .patch file", "p/testdata/mutations/a.diff", validPreamble + diffHeader, "a .patch file directly in"},
		{"a go test flag in packages", "p/testdata/mutations/a.patch", strings.Replace(validPreamble, "./p ./q", "-run=TestDoubleZero ./baseline", 1) + diffHeader, `"-run=TestDoubleZero" is not one`},
		{"-short in packages", "p/testdata/mutations/a.patch", strings.Replace(validPreamble, "./p ./q", "./count -short", 1) + diffHeader, `"-short" is not one`},
		{"an import path in packages", "p/testdata/mutations/a.patch", strings.Replace(validPreamble, "./p ./q", "example.com/p", 1) + diffHeader, `"example.com/p" is not one`},
		{"no diff", "p/testdata/mutations/a.patch", validPreamble, "no diff --git header"},
		{"scheduled-count not yes or no", "p/testdata/mutations/a.patch", strings.Replace(validPreamble, "scheduled-count: yes", "scheduled-count: true", 1) + diffHeader, "scheduled-count is yes or no"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parsePatch(c.path, []byte(c.src))
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("got error %v, want one containing %q", err, c.wantErr)
			}
		})
	}
}
