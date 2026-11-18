// Package bodyliteral builds a released body around the release step, which must not compile.
package bodyliteral

import "github.com/ppat/mediated-mailbox-mcp/api/internal/core/release"

var _ = release.Body{text: "a restricted sender's body"}
