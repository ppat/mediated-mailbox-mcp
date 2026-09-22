// Package bodyliteral builds a body around NewBody, which must not compile.
package bodyliteral

import "github.com/ppat/mediated-mailbox-mcp/core/sensitivity"

var _ = sensitivity.Body{text: "a restricted sender's body"}
