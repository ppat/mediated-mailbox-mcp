package marker_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

func TestMarkers(t *testing.T) {
	got := []string{marker.Body("bank"), marker.Field("bank"), marker.MarkupField("bank")}
	want := []string{"mmbodymarker-bank", "mmfieldmarker-bank", "<script>mmfieldmarker-bank</script>"}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("markers (-want +got):\n%s", diff)
	}
}

// A body leak search looks for BodyPrefix in text that visibly carries field markers, so neither
// prefix may contain the other.
func TestPrefixesAreDisjoint(t *testing.T) {
	if strings.Contains(marker.FieldPrefix, marker.BodyPrefix) || strings.Contains(marker.BodyPrefix, marker.FieldPrefix) {
		t.Errorf("prefixes %q and %q overlap", marker.BodyPrefix, marker.FieldPrefix)
	}
}

func TestTagOutsideLowercaseLettersPanics(t *testing.T) {
	for _, tag := range []string{"", "bank1", "Bank", "bank-name", "bänk"} {
		for name, build := range map[string]func(string) string{"Body": marker.Body, "Field": marker.Field, "MarkupField": marker.MarkupField} {
			t.Run(name+"/"+tag, func(t *testing.T) {
				defer func() {
					if recover() == nil {
						t.Errorf("%s(%q) did not panic", name, tag)
					}
				}()
				build(tag)
			})
		}
	}
}
