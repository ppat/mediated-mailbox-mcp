// Package initbuilds gives a spec a start check that returns something it built beside its error,
// which must not compile.
package initbuilds

import (
	"net/http"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
)

var _ = process.Spec[config.API]{Name: "api", Init: func([]string, []string, config.API) (http.Handler, error) {
	return nil, nil
}}
