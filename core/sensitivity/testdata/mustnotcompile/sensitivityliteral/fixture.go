// Package sensitivityliteral builds a sensitivity around New, which must not compile.
package sensitivityliteral

import "github.com/ppat/mediated-mailbox-mcp/core/sensitivity"

var _ = sensitivity.Sensitivity{class: sensitivity.NormalSender()}
