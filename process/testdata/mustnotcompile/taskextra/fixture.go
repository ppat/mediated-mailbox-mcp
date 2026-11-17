// Package taskextra gives a spec a task whose Build returns something beside its work and an error,
// which must not compile.
package taskextra

import (
	"context"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
)

var _ = process.Task[config.Migrator]{Build: func(context.Context, process.Capabilities, config.Migrator) (func(context.Context) error, func(), error) {
	return nil, nil, nil
}}
