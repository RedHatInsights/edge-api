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

var PlatformClientDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:        "edge_platform_client_duration",
		Help:        "platform HTTP client duration (in ms) by method and status",
		ConstLabels: prometheus.Labels{"service": ApplicationName, "component": BinaryName},
		Buckets:     []float64{20, 50, 100, 200, 500, 1000, 2000, 5000},
	},
	[]string{"method", "status"},
)

var StorageTransferCount = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name:        "edge_storage_transfer_counts",
		Help:        "number of download operations via S3 storage proxy",
		ConstLabels: prometheus.Labels{"service": ApplicationName, "component": BinaryName},
	},
)

var StorageTransferBytes = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name:        "edge_storage_transfer_bytes",
		Help:        "bytes transferred via S3 storage proxy",
		ConstLabels: prometheus.Labels{"service": ApplicationName, "component": BinaryName},
	},
)

var StorageTransferDuration = prometheus.NewHistogram(
	prometheus.HistogramOpts{
		Name:        "edge_storage_transfer_duration",
		Help:        "duration of S3 storage proxy operations (in ms)",
		ConstLabels: prometheus.Labels{"service": ApplicationName, "component": BinaryName},
		Buckets:     []float64{5, 20, 50, 100, 200, 500, 1000, 5000, 10000},
	},
)

var MemoryCacheHitCount = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name:        "edge_memory_cache_hit_count",
		Help:        "memory cache hit count by operation",
		ConstLabels: prometheus.Labels{"service": ApplicationName, "component": BinaryName},
	},
	[]string{"op"},
)
