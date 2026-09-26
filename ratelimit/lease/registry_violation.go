//go:build banproof

package lease

// This file registers and serves metrics through client_golang's default registry on purpose, each
// way the ban refuses. The last three statements use the process's own registry, which the ban
// allows, so a ban grown too wide reports a finding no annotation wants. The list for non-test code
// admits promhttp, because the mediator serves its metrics endpoint through promhttp.HandlerFor.
import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto" // want depguard "list 'ratelimit'" depguard "list 'non-test-code'"
	"github.com/prometheus/client_golang/prometheus/promhttp" // want depguard "list 'ratelimit'"
)

func registerOnTheDefaultRegistry() http.Handler {
	c := prometheus.NewCounter(prometheus.CounterOpts{Name: "violation_total", Help: "A counter."})
	prometheus.MustRegister(c)                     // want forbidigo `use of .prometheus\.MustRegister. forbidden because "a process registers its metrics on the registry it builds`
	if err := prometheus.Register(c); err != nil { // want forbidigo `use of .prometheus\.Register. forbidden because "a process registers its metrics on the registry it builds`
		panic(err)
	}
	prometheus.Unregister(c)                                                     // want forbidigo `use of .prometheus\.Unregister. forbidden because "a process registers its metrics on the registry it builds`
	var registerer prometheus.Registerer = prometheus.DefaultRegisterer          // want forbidigo `use of .prometheus\.DefaultRegisterer. forbidden because "a process registers its metrics on the registry it builds`
	var gatherer prometheus.Gatherer = prometheus.DefaultGatherer                // want forbidigo `use of .prometheus\.DefaultGatherer. forbidden because "a process registers its metrics on the registry it builds`
	promauto.NewGauge(prometheus.GaugeOpts{Name: "violation", Help: "A gauge."}) // want forbidigo `use of .promauto\.NewGauge. forbidden because "promauto's constructors without a registry`
	_, _ = registerer, gatherer
	_ = promhttp.Handler() // want forbidigo `use of .promhttp\.Handler. forbidden because "it serves the default registry`

	reg := prometheus.NewRegistry()
	reg.MustRegister(c)
	promauto.With(reg).NewGauge(prometheus.GaugeOpts{Name: "allowed", Help: "A gauge."})
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}
