package api

import (
	"reflect"
	"testing"
)

// TestTheRecordingMuxEmbedsNothing requires the recording mux to hold its ServeMux in a named field,
// so no promoted Handle registers a route the server would serve without recording it.
func TestTheRecordingMuxEmbedsNothing(t *testing.T) {
	typ := reflect.TypeFor[recordingMux]()
	for i := range typ.NumField() {
		if f := typ.Field(i); f.Anonymous {
			t.Errorf("recordingMux embeds %s, whose methods would register routes unrecorded", f.Type)
		}
	}
}
