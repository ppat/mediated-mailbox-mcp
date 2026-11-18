package api

import (
	"net/http"

	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
)

// Endpoint is the mounted read API, which the package's tests serve and whose registered patterns they
// read. It exists only in this package's test binary.
type Endpoint = endpoint

// Handler serves the read API behind the checks. It exists only in this package's test binary.
func (e *endpoint) Handler() http.Handler { return e.handler }

// Mount mounts routes behind checks as New does, so the package's tests can build the routes of reads,
// setup and policymanagement from options holding a fixed clock and the provider fake's consents, which
// no configuration carries. It exists only in this package's test binary.
func Mount(db accounts.DBTX, checks *server.Checks, datasets []registry.Dataset, routes ...[]server.Route) (*Endpoint, error) {
	return mount(db, checks, datasets, routes...)
}

// Admission is the read API's account check over db, as New hands it to reads. It exists only in this
// package's test binary.
var Admission = admission
