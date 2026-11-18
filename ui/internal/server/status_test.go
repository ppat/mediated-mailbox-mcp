package server_test

import (
	"net/http"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/refusal"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
)

// Every code the error contract declares is in the status table, with the one status it takes
// wherever it is refused (docs/UI.md section 17.3). The non-400 statuses are written out here, so a
// status that drifts fails, and a code the table does not hold fails rather than taking the bad
// request a missing entry falls back to, and the table holds no code the error contract does not
// declare.
func TestEveryCodeIsAnsweredWithItsStatus(t *testing.T) {
	statuses := map[int][]refusal.Code{
		http.StatusForbidden: {refusal.IdentityMissing, refusal.StalePage},
		http.StatusNotFound: {
			refusal.NotFound, refusal.UnknownClient, refusal.UnknownProvider, refusal.UnknownRow, refusal.UnknownRule,
			refusal.UnknownRun,
		},
		http.StatusConflict: {
			refusal.ClientChanged, refusal.ClientExists, refusal.ClientInUse, refusal.IdentifierTaken, refusal.NameTaken,
			refusal.NotConnected, refusal.StalePreview, refusal.StaleRule,
		},
	}
	want := map[refusal.Code]int{}
	for status, codes := range statuses {
		for _, c := range codes {
			want[c] = status
		}
	}
	codes := refusal.Codes()
	for _, c := range codes {
		status, ok := want[c]
		if !ok {
			status = http.StatusBadRequest
		}
		got, held := server.Statuses[c]
		switch {
		case !held:
			t.Errorf("%s has no status in the table", c)
		case got != status:
			t.Errorf("%s is answered %d, want %d", c, got, status)
		}
	}
	if len(server.Statuses) != len(codes) {
		t.Errorf("the table holds %d codes, the error contract declares %d", len(server.Statuses), len(codes))
	}
}
