// Package metrics owns the process-global Prometheus registry and the
// application-level collectors exposed on the admin port.
//
// The package keeps all label cardinality deliberately small: HTTP routes
// use chi RoutePattern (never raw paths), DB labels use operation+table, and
// status is collapsed to a status class ("2xx", "3xx", "4xx", "5xx"). This
// keeps the TSDB footprint bounded under adversarial traffic.
package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Registry is the single process registry. We do not use the default registry
// so tests can construct their own easily and so we control exactly which
// collectors ship.
var Registry = prometheus.NewRegistry()

var (
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "zenflow",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total HTTP requests processed, labeled by method, route pattern, and status class.",
		},
		[]string{"method", "route", "status_class"},
	)

	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "zenflow",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds.",
			// Tuned for sub-second API latencies with tail visibility up to 10s.
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "route", "status_class"},
	)

	HTTPRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "zenflow",
			Subsystem: "http",
			Name:      "requests_in_flight",
			Help:      "Number of HTTP requests currently being served.",
		},
	)

	DBQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "zenflow",
			Subsystem: "db",
			Name:      "queries_total",
			Help:      "Total DB queries, labeled by op, table, and outcome (ok|error).",
		},
		[]string{"op", "table", "outcome"},
	)

	DBQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "zenflow",
			Subsystem: "db",
			Name:      "query_duration_seconds",
			Help:      "DB query latency in seconds.",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"op", "table"},
	)

	DeviceProfilesCreatedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "zenflow",
			Subsystem: "device_profiles",
			Name:      "created_total",
			Help:      "Number of device profiles successfully created.",
		},
	)

	DeviceProfilesValidationErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "zenflow",
			Subsystem: "device_profiles",
			Name:      "validation_errors_total",
			Help:      "Validation errors encountered while creating or patching device profiles, labeled by field.",
		},
		[]string{"field"},
	)

	TemplateLookupsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "zenflow",
			Subsystem: "templates",
			Name:      "lookups_total",
			Help:      "Template lookups, labeled by outcome (hit|miss|error).",
		},
		[]string{"outcome"},
	)
)

func init() {
	Registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		HTTPRequestsTotal,
		HTTPRequestDuration,
		HTTPRequestsInFlight,
		DBQueriesTotal,
		DBQueryDuration,
		DeviceProfilesCreatedTotal,
		DeviceProfilesValidationErrorsTotal,
		TemplateLookupsTotal,
	)
}

// StatusClass collapses an HTTP status code to its class label. Invalid codes
// (0, <100, >=600) map to "unknown" so we never emit an empty label.
func StatusClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500 && code < 600:
		return "5xx"
	default:
		return "unknown"
	}
}

// StatusString is a helper for callers that want the numeric form in logs.
func StatusString(code int) string { return strconv.Itoa(code) }
