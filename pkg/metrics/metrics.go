package metrics

import (
	"os"

	"github.com/prometheus/client_golang/prometheus"
)

const ApplicationName = "edge-api"

var BinaryName = ApplicationName

func init() {
	BinaryName = os.Args[0]
}

var JobEnqueuedCount = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name:        "edge_enqueued_jobs_count",
		Help:        "job count by result (finished/timeouted/panicked/cancelled) by type",
		ConstLabels: prometheus.Labels{"service": ApplicationName, "component": BinaryName},
	},
	[]string{"type"},
)

var JobProcessedCount = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name:        "edge_processed_jobs_count",
		Help:        "job count by result (finished/timeouted/panicked/cancelled) by type",
		ConstLabels: prometheus.Labels{"service": ApplicationName, "component": BinaryName},
	},
	[]string{"type", "result"},
)

var JobQueueSize = prometheus.NewGauge(prometheus.GaugeOpts{
	Name:        "edge_job_queue_size",
	Help:        "background job queue size (total pending jobs)",
	ConstLabels: prometheus.Labels{"service": ApplicationName, "component": BinaryName},
})

var JobActiveSize = prometheus.NewGauge(prometheus.GaugeOpts{
	Name:        "edge_job_active_size",
	Help:        "active background job (total processing jobs)",
	ConstLabels: prometheus.Labels{"service": ApplicationName, "component": BinaryName},
})

var BackgroundJobDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:        "edge_job_duration",
		Help:        "background job duration (in seconds) by type",
		ConstLabels: prometheus.Labels{"service": ApplicationName, "component": BinaryName},
		Buckets:     []float64{1, 10, 30, 60 * 2, 60 * 10, 60 * 30, 60 * 120, 60 * 240},
	},
	[]string{"type"},
)
