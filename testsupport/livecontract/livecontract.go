// Package livecontract is what keeps a contract run against a real provider apart from every other
// test run (ADR-0043). A live test carries its provider's build tag, and on top of that it runs only
// when the marker this package names holds its provider, which only go tool livecontract sets. Any
// other invocation, the build tag included, skips the test before it reads a credential or calls
// the provider. The command's run is the one deliberate way in.
package livecontract

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

// Marker is the environment variable go tool livecontract sets to the provider it runs, and nothing
// else sets.
const Marker = "MEDIATED_MAILBOX_LIVECONTRACT"

// live is one provider's contract run against the real provider, the build tag its test carries,
// the package it sits in and the test's name.
type live struct {
	tag     string
	pkg     string
	test    string
	timeout string
}

// providers returns each provider a live run exists for.
func providers() map[string]live {
	return map[string]live{
		"gmail": {tag: "gmail_live", pkg: "./provider/gmail/", test: "TestTheAdapterPassesTheContractAgainstGmail", timeout: "55m"},
	}
}

// Require skips the test unless go tool livecontract started it for provider. A live test calls it
// first, before it reads a credential or calls the provider.
func Require(t testing.TB, provider string) {
	t.Helper()
	if os.Getenv(Marker) != provider {
		t.Skipf("the contract run against %s runs only through go tool livecontract %s", provider, provider)
	}
}

// Command is the go test invocation go tool livecontract runs, its arguments after go and its
// environment.
type Command struct {
	Args []string
	Env  []string
}

// Invocation returns the invocation that runs provider's live test, with environ, the caller's
// environment, passed through and the marker set to provider. The caller's credential variables
// reach the test that way, and nothing here reads or prints them.
func Invocation(provider string, environ []string) (Command, error) {
	p, ok := providers()[provider]
	if !ok {
		names := make([]string, 0, len(providers()))
		for name := range providers() {
			names = append(names, name)
		}
		slices.Sort(names)
		return Command{}, fmt.Errorf("no live contract run exists for %q, only for %s", provider, strings.Join(names, ", "))
	}
	env := slices.DeleteFunc(slices.Clone(environ), func(v string) bool { return strings.HasPrefix(v, Marker+"=") })
	return Command{
		Args: []string{"test", "-tags", p.tag, "-count=1", "-timeout", p.timeout, "-v", "-run", "^" + p.test + "$", p.pkg},
		Env:  append(env, Marker+"="+provider),
	}, nil
}
