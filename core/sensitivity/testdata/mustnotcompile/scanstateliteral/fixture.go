// Package scanstateliteral builds a scan state around its constructors, which must not compile.
package scanstateliteral

import "github.com/ppat/mediated-mailbox-mcp/core/sensitivity"

var _ = sensitivity.ScanState{state: 1}
