// Command keypaircheck checks that a key pair is one the credential library seals to and opens with
// (ADR-0088). The release workflow runs it on the pair a key-generation binary attached to the
// release writes, the part of that binary's check a Go test cannot carry, because a test runs only
// in the workflow of its kind (ADR-0124). Run from anywhere.
//
//	go tool keypaircheck -private-key-file credential.key -public-key-file credential.pub
//
// It seals a value to the public key file, opens it with a keyring loaded from both files, and
// exits non-zero unless the value comes back unchanged.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "keypaircheck:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("keypaircheck", flag.ContinueOnError)
	privatePath := flags.String("private-key-file", "", "the private key file to open with")
	publicPath := flags.String("public-key-file", "", "the public key file to seal to")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *privatePath == "" || *publicPath == "" {
		return errors.New("-private-key-file and -public-key-file are both required")
	}
	public, err := seal.LoadPublicKey(*publicPath)
	if err != nil {
		return fmt.Errorf("loading the public key: %w", err)
	}
	purpose := seal.AccountCredential("keypaircheck")
	value := []byte("a value sealed by keypaircheck")
	sealed, err := public.Seal(value, purpose)
	if err != nil {
		return fmt.Errorf("sealing to the public key: %w", err)
	}
	ring, err := open.Load(*publicPath, *privatePath)
	if err != nil {
		return fmt.Errorf("loading the keyring: %w", err)
	}
	opened, err := ring.Open(sealed, purpose)
	if err != nil {
		return fmt.Errorf("opening the sealed value: %w", err)
	}
	if !bytes.Equal(opened, value) {
		return errors.New("the opened value differs from the value sealed")
	}
	return nil
}
