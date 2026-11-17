package adapters_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// The list of adapters and a token expose no field, so code outside the provider library, which the
// compiler refuses this package to, holds a list only as Shipped or the provider fake built it and
// cannot add an adapter or reach a token's source. A field exported later fails here, as no fixture
// written against today's fields would (ADR-0010, ADR-0142).
func TestAListAndATokenExposeNoField(t *testing.T) {
	mustnotcompile.RequireNoExportedFields(t, "github.com/ppat/mediated-mailbox-mcp/provider/internal/adapters", "List", "Token")
}
