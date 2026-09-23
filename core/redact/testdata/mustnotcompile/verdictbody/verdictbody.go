// Package verdictbody builds a gate verdict holding the body it withholds, which must not compile.
package verdictbody

import "github.com/ppat/mediated-mailbox-mcp/core/redact"

var _ = redact.Verdict{body: "a restricted sender's body"}
