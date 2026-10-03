//go:build banproof

package main

// This file imports the standard library's public-key packages on purpose. The UI's shipped code
// outside ui/internal/clientsecret opens no stored value, and its crypto list admits only the
// packages that code uses, while every other list admits the whole standard library (ADR-0081). Each
// import is one the crypto entry of $gostd would admit, and an exact entry sorted just before it
// refuses, crypto/hpke under crypto/hmac$, crypto/mlkem under crypto/hmac$ and crypto/ecdh under
// crypto/cipher$.
import (
	_ "crypto/ecdh"  // want depguard "list 'ui-shipped-crypto'"
	_ "crypto/hpke"  // want depguard "list 'ui-shipped-crypto'"
	_ "crypto/mlkem" // want depguard "list 'ui-shipped-crypto'"
)
