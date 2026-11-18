package policymanagement

import (
	"net/http"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/refusal"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
)

// Identity is who a decision or a policy write is recorded as made by (ADR-0084). With Header set, the
// identity is that header's value, which an authenticating proxy the deployment declares sets, and a
// write without it is refused. With Header unset, it is Operator, the configured operator name, and
// nothing in the request is trusted (docs/UI.md section 15).
type Identity struct {
	Header   string
	Operator string
}

// identity returns who the request is recorded as made by, and answers the request when that identity
// is empty, the declared header missing or the operator name configured empty.
func (s *handlers) identity(w http.ResponseWriter, r *http.Request) (string, bool) {
	v := s.opts.Identity.Operator
	if s.opts.Identity.Header != "" {
		v = r.Header.Get(s.opts.Identity.Header)
	}
	// A write always records who made it, so an identity that resolves empty, a missing header or an
	// operator name configured empty, is refused rather than recorded.
	v = strings.TrimSpace(v)
	if v == "" {
		server.WriteFailure(w, r, server.ClientFault(refusal.IdentityMissing, "No identity was forwarded, so the change was not recorded"))
		return "", false
	}
	return v, true
}
