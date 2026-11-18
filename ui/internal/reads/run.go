package reads

import (
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// run is a job run's whole recorded state, the registry's Run, which the jobs endpoint, the stream's
// run event and the runs dataset's rows share, each built through registry.RunOf (docs/UI.md sections
// 8.3 and 17.5).
type run = registry.Run

func runType() schema.Type { return registry.RunType() }
