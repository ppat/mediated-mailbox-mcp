package fixture_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/fixture"
)

// markerKinds names the kind of marker each field of m carries. A field is read by reflection, so a
// field added to Message without a marker shows up here as carrying none.
func markerKinds(m fixture.Message) map[string]string {
	kinds := map[string]string{}
	v := reflect.ValueOf(m)
	for i := range v.NumField() {
		text := v.Field(i).String()
		body := strings.Contains(text, marker.BodyPrefix)
		field := strings.Contains(text, marker.FieldPrefix)
		switch {
		case body && field:
			kinds[v.Type().Field(i).Name] = "both"
		case body:
			kinds[v.Type().Field(i).Name] = "body"
		case field:
			kinds[v.Type().Field(i).Name] = "field"
		default:
			kinds[v.Type().Field(i).Name] = "none"
		}
	}
	return kinds
}

// The body carries a body marker and every metadata field a field marker, and no field carries the
// other kind, so a search for body text never matches visible metadata.
func TestEveryFixtureCarriesMarkers(t *testing.T) {
	want := map[string]string{"FromAddress": "field", "FromName": "field", "Subject": "field", "Body": "body"}
	for _, m := range fixture.All() {
		if diff := cmp.Diff(want, markerKinds(m), compare.Options); diff != "" {
			t.Errorf("fixture %+v marker kinds (-want +got):\n%s", m, diff)
		}
	}
}

// Each fixture's markers are its own. No field of one fixture contains a field of another, so a
// marker found in any output names the one fixture it came from.
func TestFixtureMarkersAreDistinct(t *testing.T) {
	all := fixture.All()
	for i, a := range all {
		for j, b := range all {
			if i == j {
				continue
			}
			va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
			for fa := range va.NumField() {
				for fb := range vb.NumField() {
					if outer, inner := va.Field(fa).String(), vb.Field(fb).String(); strings.Contains(outer, inner) {
						t.Errorf("fixture %d's value %q contains fixture %d's value %q", i, outer, j, inner)
					}
				}
			}
		}
	}
}

// ADR-0044 requires markers shaped like markup among the metadata fields.
func TestSomeMetadataMarkerIsMarkup(t *testing.T) {
	for _, m := range fixture.All() {
		for _, text := range []string{m.FromName, m.Subject} {
			if strings.HasPrefix(text, "<") {
				return
			}
		}
	}
	t.Error("no fixture carries a markup-shaped field marker")
}

// Senders sit under the reserved .example domain, so no fixture names a real one.
func TestSendersUseTheReservedDomain(t *testing.T) {
	for _, m := range fixture.All() {
		if !strings.HasSuffix(m.FromAddress, ".example") {
			t.Errorf("sender %q is outside the reserved .example domain", m.FromAddress)
		}
	}
}
