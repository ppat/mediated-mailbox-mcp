package dbmetrics_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// sample is one series as gathered, its labels joined as name=value pairs in order, and its value,
// a counter's or gauge's, or a histogram's count.
type sample struct {
	Labels string
	Value  float64
}

// gathered returns every series of the family named name on reg, sorted by their labels.
func gathered(t *testing.T, reg prometheus.Gatherer, name string) []sample {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var out []sample
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			var labels []string
			for _, l := range m.GetLabel() {
				labels = append(labels, l.GetName()+"="+l.GetValue())
			}
			s := sample{Labels: strings.Join(labels, ",")}
			switch {
			case m.GetCounter() != nil:
				s.Value = m.GetCounter().GetValue()
			case m.GetGauge() != nil:
				s.Value = m.GetGauge().GetValue()
			case m.GetHistogram() != nil:
				s.Value = float64(m.GetHistogram().GetSampleCount())
			}
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Labels < out[j].Labels })
	return out
}
