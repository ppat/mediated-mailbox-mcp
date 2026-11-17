// Package app is the UI's entry package, which holds its spec and nothing else (ADR-0142). The spec
// lists three endpoints, the read API at /ui/api/, built by api.New, the pages at /ui/, built by
// pages.New, and the redirect at /, built by pages.Redirect (ADR-0137). Each sits behind the checks
// ui/internal/server builds once for the process. The process starts the UI as it starts every
// program, refusing the database driver's password variables before the configuration is read,
// reading the configuration, answering --help and --version, logging every effective value, setting
// the level from log_level, running the configuration's checks and writing the start banner, then
// builds the endpoints and serves them. It sits outside internal, so a spec other than the UI's own can
// join it.
package app

import (
	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/pages"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/paths"
)

// Spec is the UI's spec.
func Spec() process.Spec[config.UI] {
	return process.Spec[config.UI]{
		Name: "ui",
		Endpoints: []process.Endpoint[config.UI]{
			{Pattern: paths.API + "/", Build: api.New},
			{Pattern: paths.Root + "/", Build: pages.New},
			{Pattern: "/{$}", Build: pages.Redirect},
		},
	}
}
