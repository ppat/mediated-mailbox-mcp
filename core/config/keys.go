package config

import (
	"errors"
	"strconv"
)

// Keys is the credential part, under the key credential. It names the mounted key files a program
// seals and opens stored values with, the public key every sealer seals to and every private key of
// the keyring (ADR-0079, ADR-0088, ADR-0092). Every program that reads those files holds it under
// the same keys, the UI included. It holds paths and never key material.
type Keys struct {
	// PublicKeyFile names the mounted public key every sealer seals to. A keyring whose private keys
	// derive no key matching it refuses the start (ADR-0088).
	PublicKeyFile string `yaml:"public_key_file" settings:"required"`
	// PrivateKeyFiles names every mounted private key. During a key replacement the old key and the
	// new one are both mounted (ADR-0092).
	PrivateKeyFiles []string `yaml:"private_key_files" settings:"required"`
}

// Check refuses an empty public key path, an empty list of private keys and an empty path in that
// list.
func (k Keys) Check() error {
	switch {
	case k.PublicKeyFile == "":
		return errors.New("credential.public_key_file is empty")
	case len(k.PrivateKeyFiles) == 0:
		return errors.New("credential.private_key_files names no file")
	}
	for i, path := range k.PrivateKeyFiles {
		if path == "" {
			return errors.New("credential.private_key_files entry " + strconv.Itoa(i) + " is empty")
		}
	}
	return nil
}

// Accounts is the accounts part, embedded inline so its one key sits at the top of the tree.
type Accounts struct {
	// AccountReloadInterval is how often the process's Session reloads its list of accounts, their
	// connections and their policy (ADR-0090).
	AccountReloadInterval Duration `yaml:"account_reload_interval"`
}

// DefaultAccounts is the accounts part's defaults, a reload each minute.
func DefaultAccounts() Accounts {
	return Accounts{AccountReloadInterval: minute}
}

// Check refuses a reload interval that is not positive.
func (a Accounts) Check() error {
	if a.AccountReloadInterval <= 0 {
		return errors.New("account_reload_interval is not positive")
	}
	return nil
}
