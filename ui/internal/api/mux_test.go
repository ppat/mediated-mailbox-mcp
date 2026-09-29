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

// TestTheRecordingMuxRefusesACopy requires the recording mux to hold a field whose pointer has Lock
// and Unlock methods, which is what go vet's copylocks check looks for, so a copy of a recording mux,
// whose Handle would record a route where Served and ProbesServed never read it, fails lint.
func TestTheRecordingMuxRefusesACopy(t *testing.T) {
	typ := reflect.TypeFor[recordingMux]()
	for i := range typ.NumField() {
		p := reflect.PointerTo(typ.Field(i).Type)
		_, lock := p.MethodByName("Lock")
		_, unlock := p.MethodByName("Unlock")
		if lock && unlock {
			return
		}
	}
	t.Error("recordingMux holds no field with Lock and Unlock methods, so go vet does not refuse a copy of it")
}
