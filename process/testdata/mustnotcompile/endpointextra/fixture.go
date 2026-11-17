// Package endpointextra gives a spec an endpoint whose Build takes a parameter beyond the build's
// context, the capabilities and the configuration, which must not compile.
package endpointextra

import (
	"context"
	"net/http"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
)

var _ = process.Endpoint[config.API]{Pattern: "/", Build: func(context.Context, process.Capabilities, config.API, *http.Client) (http.Handler, error) {
	return nil, nil
}}
