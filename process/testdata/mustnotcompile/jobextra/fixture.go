// Package jobextra gives a spec a job kind whose Build takes a parameter beyond the build's context,
// the capabilities and the configuration, which must not compile.
package jobextra

import (
	"context"
	"net/http"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
)

var _ = process.Job[config.Worker]{Name: "kind", Build: func(context.Context, process.Capabilities, config.Worker, *http.Client) (schedule.Plan, error) {
	return schedule.Plan{}, nil
}}
