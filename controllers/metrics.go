package controllers

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// When adding metric names, see https://prometheus.io/docs/practices/naming/#metric-names
const (
	specialResourcesCreatedQuery = "sro_managed_resources_total"
	completedStatesQuery         = "sro_states_completed_info"
)

var (
	//#TODO set the metric
	specialResourcesCreated = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: specialResourcesCreatedQuery,
			Help: "Number of specialresources created",
		},
	)
	completedStates = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: completedStatesQuery,
			Help: "For a given specialresource and state, 1 if the state is completed, 0 if it is not.",
		},
		[]string{"specialresource", "state"},
	)
)

func setCompletedState(specialResource string, state string, value int) {
	completedStates.WithLabelValues(specialResource, state).Set(float64(value))
}

func setSpecialResourcesCreated(value int) {
	specialResourcesCreated.Set(float64(value))
}

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(
		specialResourcesCreated,
		completedStates,
	)

}
