package fixture_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/fixture"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
)

// texts returns the text each field of m holds, a list field's elements joined. A field is read by
// reflection, so a field added to Message shows up here without being named.
func texts(m fixture.Message) map[string]string {
	out := map[string]string{}
	v := reflect.ValueOf(m)
	for i := range v.NumField() {
		f := v.Field(i)
		if f.Kind() == reflect.Slice {
			parts := make([]string, f.Len())
			for j := range f.Len() {
				parts[j] = f.Index(j).String()
			}
			out[v.Type().Field(i).Name] = strings.Join(parts, " ")
			continue
		}
		out[v.Type().Field(i).Name] = f.String()
	}
	return out
}

// markerKinds names the kind of marker each field of m carries. A field added to Message without a
// marker shows up here as carrying none.
func markerKinds(m fixture.Message) map[string]string {
	kinds := map[string]string{}
	for name, text := range texts(m) {
		body := strings.Contains(text, marker.BodyPrefix)
		field := strings.Contains(text, marker.FieldPrefix)
		switch {
		case body && field:
			kinds[name] = "both"
		case body:
			kinds[name] = "body"
		case field:
			kinds[name] = "field"
		case text == "":
			kinds[name] = "empty"
		default:
			kinds[name] = "none"
		}
	}
	return kinds
}

// The body carries a body marker and every metadata field a field marker, and no field carries the
// other kind, so a search for body text never matches visible metadata. A field only some messages
// have is empty on the others.
func TestEveryFixtureCarriesMarkers(t *testing.T) {
	for _, m := range fixture.All() {
		want := map[string]string{
			"FromAddress": "field", "FromName": "field", "ToAddress": "field", "Subject": "field", "Body": "body",
			"CcAddress": "field", "ListID": "field", "Attachments": "field",
		}
		for _, optional := range []string{"CcAddress", "ListID", "Attachments"} {
			if texts(m)[optional] == "" {
				want[optional] = "empty"
			}
		}
		if diff := cmp.Diff(want, markerKinds(m), compare.Options); diff != "" {
			t.Errorf("fixture %+v marker kinds (-want +got):\n%s", m, diff)
		}
	}
}

// Each fixture's markers are its own. No field of one fixture contains a field of another, so a
// marker found in any output names the one fixture it came from. The one exception is the reply's
// subject, which holds the subject of the newsletter it replies to.
func TestFixtureMarkersAreDistinct(t *testing.T) {
	all := fixture.All()
	reply, newsletter := fixture.NewsletterReply(), fixture.Newsletter()
	for i, a := range all {
		for j, b := range all {
			if i == j {
				continue
			}
			for fa, outer := range texts(a) {
				for fb, inner := range texts(b) {
					if inner == "" || !strings.Contains(outer, inner) {
						continue
					}
					if fa == "Subject" && fb == "Subject" && outer == reply.Subject && inner == newsletter.Subject {
						continue
					}
					t.Errorf("fixture %d's value %q contains fixture %d's value %q", i, outer, j, inner)
				}
			}
		}
	}
}

// Each field only some messages have is set on at least one fixture, so a mapping of that field to
// the canonical model is exercised.
func TestEveryOptionalFieldIsSetSomewhere(t *testing.T) {
	for _, optional := range []string{"CcAddress", "ListID", "Attachments"} {
		set := false
		for _, m := range fixture.All() {
			set = set || texts(m)[optional] != ""
		}
		if !set {
			t.Errorf("no fixture sets %s", optional)
		}
	}
}

// The reply's subject is the newsletter's with a reply prefix, which a provider needs to put the two
// in one thread.
func TestTheReplySharesTheNewslettersSubject(t *testing.T) {
	if got, want := fixture.NewsletterReply().Subject, "Re: "+fixture.Newsletter().Subject; got != want {
		t.Errorf("the reply's subject is %q, want %q", got, want)
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
