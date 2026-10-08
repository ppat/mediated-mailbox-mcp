// Package core is the credential section of a deployable's configuration and its validation, a pure
// decision function over the section's parameters (ADR-0040, ADR-0078). The section names the
// mounted key files a deployable that opens credentials opens and seals them with, the public key
// every sealer seals to and every private key of the keyring (ADR-0079, ADR-0088, ADR-0092). It
// holds paths and never key material.
package core

import (
	"errors"
	"strconv"
)

// Config is the credential section.
type Config struct {
	// PublicKeyFile names the mounted public key a rotated credential is sealed to. A keyring whose
	// private keys derive no key matching it refuses the start (ADR-0088).
	PublicKeyFile string `yaml:"public_key_file" settings:"required"`
	// PrivateKeyFiles names every mounted private key. During a key replacement the old key and the
	// new one are both mounted (ADR-0092).
	PrivateKeyFiles []string `yaml:"private_key_files" settings:"required"`
}

// Validate refuses an empty public key path, an empty list of private keys and an empty path in
// that list.
func Validate(c Config) error {
	switch {
	case c.PublicKeyFile == "":
		return errors.New("credential.public_key_file is empty")
	case len(c.PrivateKeyFiles) == 0:
		return errors.New("credential.private_key_files names no file")
	}
	for i, path := range c.PrivateKeyFiles {
		if path == "" {
			return errors.New("credential.private_key_files entry " + strconv.Itoa(i) + " is empty")
		}
	}
	return nil
}
