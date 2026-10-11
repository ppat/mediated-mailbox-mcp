package service

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/redact"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/core/release"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// The body operation's result names its reason by one token of a closed set, and the set its schema
// states is exactly the tokens the operation can return, every reason of the gate and of the release
// step a decision can reach and the conversion's refusal, so the contract neither lists a token
// nothing returns nor misses one a client may receive (ADR-0101). The release step's undecided reason
// is no token, since nothing returns a decision nobody made. The test reads unexported names, so it
// sits in the package.
func TestTheReasonEnumIsTheTokensABodyDecisionReturns(t *testing.T) {
	var returned []string
	for _, r := range []redact.Reason{redact.InvalidState, redact.PendingScan, redact.SkippedRestricted, redact.RestrictedSender, redact.ContentFlagged, redact.Released} {
		returned = append(returned, r.String())
	}
	for _, r := range []release.Reason{release.NotReleasable, release.NoScanner, release.PatternMatched, release.Released} {
		returned = append(returned, r.String())
	}
	returned = append(returned, conversionRefused)
	slices.Sort(returned)
	returned = slices.Compact(returned)

	var schema struct {
		Properties struct {
			Reason struct {
				Enum []string `json:"enum"`
			} `json:"reason"`
		} `json:"properties"`
	}
	if err := json.Unmarshal([]byte(bodySchema), &schema); err != nil {
		t.Fatal(err)
	}
	enum := slices.Sorted(slices.Values(schema.Properties.Reason.Enum))
	if diff := cmp.Diff(returned, enum, compare.Options); diff != "" {
		t.Errorf("the reason enum against the tokens a body decision returns (-returned +enum):\n%s", diff)
	}
	if slices.Contains(enum, release.Undecided.String()) {
		t.Errorf("the reason enum holds %q, which nothing returns", release.Undecided.String())
	}
}
