// Package helper sits in the test library outside its subsections and imports one on purpose, and
// the check refuses it.
package helper

import (
	_ "github.com/ppat/mediated-mailbox-mcp/db/listing"
)
