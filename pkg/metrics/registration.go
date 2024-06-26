package metrics

import "github.com/prometheus/client_golang/prometheus"

func RegisterAPIMetrics() {
	prometheus.MustRegister(
		JobEnqueuedCount,
		JobProcessedCount,
		JobQueueSize,
		JobActiveSize,
		BackgroundJobDuration,
		PlatformClientDuration,
	)
}
