package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/process/version"
)

// F6's part of VERIFICATIONS' row for the key-generation command. The program's main function runs
// its keygen subcommand to write a key pair into a temporary directory,
// executioncontext/credential/seal seals a value to the public key file and
// executioncontext/credential/open opens it with a keyring loaded from both files (ADR-0088).
func TestTheCommandWritesAPairTheLibrarySealsToAndOpensWith(t *testing.T) {
	dir := t.TempDir()
	privatePath, publicPath := filepath.Join(dir, "credential.key"), filepath.Join(dir, "credential.pub")
	args := os.Args
	t.Cleanup(func() { os.Args = args })
	os.Args = []string{"mediated-mailbox-cli", "keygen", "-private-key-file", privatePath, "-public-key-file", publicPath}

	main()

	public, err := seal.LoadPublicKey(publicPath)
	if err != nil {
		t.Fatalf("loading the public key the command wrote: %v", err)
	}
	c := seal.AccountCredential("personal")
	sealed, err := public.Seal([]byte("refresh-token"), c)
	if err != nil {
		t.Fatal(err)
	}
	ring, err := open.Load(publicPath, privatePath)
	if err != nil {
		t.Fatalf("loading the keyring from the files the command wrote: %v", err)
	}
	if got, err := ring.Open(sealed, c); err != nil || string(got) != "refresh-token" {
		t.Errorf("opened %q, %v, want the value sealed", got, err)
	}
}

// -version prints the release version and runs no subcommand, even when one is named after it.
func TestTheVersionFlagPrintsTheReleaseVersion(t *testing.T) {
	dir := t.TempDir()
	privatePath, publicPath := filepath.Join(dir, "credential.key"), filepath.Join(dir, "credential.pub")
	for _, flag := range []string{"-version", "--version"} {
		var stdout, stderr bytes.Buffer
		code := run([]string{flag, "keygen", "-private-key-file", privatePath, "-public-key-file", publicPath}, &stdout, &stderr)
		if want := version.Number + "\n"; code != 0 || stdout.String() != want || stderr.Len() != 0 {
			t.Errorf("%s exited %d, printed %q and %q to standard error, want 0 and %q", flag, code, stdout.String(), stderr.String(), want)
		}
	}
	for _, path := range []string{privatePath, publicPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s was written: %v", path, err)
		}
	}
}

// A missing or unknown subcommand, or a flag the program does not take, prints the usage naming
// every subcommand to standard error and exits 2, with nothing on standard output.
func TestAMissingOrUnknownSubcommandPrintsTheUsage(t *testing.T) {
	for name, args := range map[string][]string{
		"no subcommand":   nil,
		"an unknown one":  {"keygenerate"},
		"an unknown flag": {"-private-key-file", "credential.key", "keygen"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 || stdout.Len() != 0 {
			t.Errorf("%s: exited %d and printed %q, want 2 and nothing", name, code, stdout.String())
		}
		if !strings.Contains(stderr.String(), "\n  keygen ") {
			t.Errorf("%s: standard error %q does not list the keygen subcommand", name, stderr.String())
		}
	}
}

// A subcommand's error is printed after the subcommand's name and the program exits 1, as the
// key-generation command did on its own.
func TestASubcommandsErrorExitsOne(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"keygen", "-private-key-file", filepath.Join(t.TempDir(), "credential.key")}, &stdout, &stderr)
	if want := "keygen: -private-key-file and -public-key-file are both required\n"; code != 1 || stderr.String() != want || stdout.Len() != 0 {
		t.Errorf("exited %d, printed %q and %q to standard error, want 1, nothing and %q", code, stdout.String(), stderr.String(), want)
	}
}
