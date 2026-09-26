// Command keygen writes a new key pair that credentials are sealed to (ADR-0088). No standard tool
// writes X-Wing keys, so the operator runs this command to set up a deployment and to replace a key
// (ADR-0092).
//
// The -private-key-file flag names where the private key goes, the 32-byte X-Wing seed, written
// with mode 0600. The -public-key-file flag names where the public key goes, the 1216-byte X-Wing
// public key, written with mode 0644. Both files hold the raw key and nothing else. Neither file may
// exist already, so a key in use is never overwritten. The command prints the public key's
// identifier, the one every value sealed to it names in its header, to standard output.
//
//	keygen -private-key-file credential.key -public-key-file credential.pub
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "keygen:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("keygen", flag.ContinueOnError)
	privatePath := flags.String("private-key-file", "", "the file to write the private key to")
	publicPath := flags.String("public-key-file", "", "the file to write the public key to")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *privatePath == "" || *publicPath == "" {
		return errors.New("-private-key-file and -public-key-file are both required")
	}
	key, err := seal.KEM().GenerateKey()
	if err != nil {
		return fmt.Errorf("generating the key pair: %w", err)
	}
	seed, err := key.Bytes()
	if err != nil {
		return fmt.Errorf("serializing the private key: %w", err)
	}
	public := key.PublicKey().Bytes()
	if err := create(*privatePath, seed, 0o600); err != nil {
		return err
	}
	if err := create(*publicPath, public, 0o644); err != nil {
		return errors.Join(err, os.Remove(*privatePath))
	}
	_, err = fmt.Fprintln(stdout, seal.IDOf(public))
	return err
}

// create writes content to a new file at path, refusing a path that exists. The mode is set
// explicitly once the file exists, so the process's umask cannot narrow it.
func create(path string, content []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode) //nolint:gosec // the path is the one the operator names
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	err = f.Chmod(mode)
	if err == nil {
		_, err = f.Write(content)
	}
	if err == nil {
		err = f.Sync()
	}
	if err = errors.Join(err, f.Close()); err != nil {
		return errors.Join(fmt.Errorf("writing %s: %w", path, err), os.Remove(path))
	}
	return nil
}
