//go:build banproof

package main

// This file imports packages ./... does not list on purpose, one in each kind of directory ./...
// skips that git keeps. propose's import list admits each by prefix and no lint reads the packages,
// so banproof's check that the build of ./... reaches only packages ./... lists is the one refusing
// them.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/propose/internal/.target"         // want unlinted "imports github.com/ppat/mediated-mailbox-mcp/propose/internal/.target, a package ./... does not list"
	_ "github.com/ppat/mediated-mailbox-mcp/propose/internal/_target"         // want unlinted "imports github.com/ppat/mediated-mailbox-mcp/propose/internal/_target, a package ./... does not list"
	_ "github.com/ppat/mediated-mailbox-mcp/propose/internal/testdata/target" // want unlinted "imports github.com/ppat/mediated-mailbox-mcp/propose/internal/testdata/target, a package ./... does not list"
)
