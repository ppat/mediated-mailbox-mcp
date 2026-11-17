//go:build banproof

package migrate

// This file breaks the migrator task's import list on purpose. The import names a third-party package
// no list over shipped code admits.
import (
	_ "golang.org/x/tools/go/packages" // want depguard "list 'migrator-migrate'" depguard "list 'non-test-code'"
)
