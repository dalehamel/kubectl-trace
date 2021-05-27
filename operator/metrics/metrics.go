package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	crMetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Tracejob metrics
var (
	TracejobOutputInvalidErrorTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tracejob_output_invalid_error_total",
		Help: "The output destination for the Tracejob is invalid",
	}, []string{"target_namespace"})
	TracejobTargetInvalidErrorTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tracejob_target_invalid_error_total",
		Help: "Something about the target (node / pod / container / process) for the Tracejob is invalid",
	}, []string{"target_namespace"})
	TracejobCreateErrorTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tracejob_create_error_total",
		Help: "There was an error creating the Job resource to run the tracejob",
	}, []string{"target_namespace"})
	TracejobStartedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tracejob_started_total",
		Help: "A Tracejob started successfully",
	}, []string{"target_namespace"})
	TracejobFailedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tracejob_failed_total",
		Help: "A Tracejob started, but failed to complete successfully",
	}, []string{"target_namespace"})
	TracejobSucceededTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tracejob_succeeded_total",
		Help: "A Tracejob completed successfully",
	}, []string{"target_namespace"})
	TracejobStatusUpdateConflictTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tracejob_status_update_conflict_total",
		Help: "An unexpected error occurred while updating Tracejob status",
	}, []string{"target_namespace"})
)

// TracejobGC w/o label
var (
	TracejobGCTracejobListErrorTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tracejobgc_tracejob_list_error_total",
		Help: "There was an error while listing the Tracejobs",
	})
	TracejobGCStatusUpdateErrorTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tracejobgc_status_update_error_total",
		Help: "There was an error while updating the TracejobGC status",
	})
	TracejobGCExecutedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tracejobgc_executed_total",
		Help: "The TracejobGC controller completed a sweep execution successfully",
	})
)

// TracejobGC w/ label
var (
	TracejobGCTracejobDeleteErrorTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tracejobgc_tracejob_delete_error_total",
		Help: "There was an error deleting a Tracejob with specified condition",
	}, []string{"condition"})
	TracejobGCTracejobDeleteTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tracejobgc_tracejob_delete_total",
		Help: "A Tracejob with specified condition was deleted successfully",
	}, []string{"condition"})
	TracejobGCTracejobJobsCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tracejobgc_tracejob_jobs_count",
		Help: "The total remaining Tracejobs with specified condition that did not meet GC threshold ",
	}, []string{"condition"})
)

var (
	GCDurationMilliseconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "gc_duration_milliseconds",
		Help:    "The latency in miliseconds for a Tracejob garbage collection run",
		Buckets: prometheus.ExponentialBuckets(1, 10, 7), // 1 milliseconds to ~16.7 minutes
	})
)

func RegisterAllMetrics() {
	// Register tracejob controller metrics
	crMetrics.Registry.MustRegister(
		TracejobOutputInvalidErrorTotal,
		TracejobTargetInvalidErrorTotal,
		TracejobCreateErrorTotal,
		TracejobStartedTotal,
		TracejobFailedTotal,
		TracejobSucceededTotal,
		TracejobStatusUpdateConflictTotal,
	)

	// Register GC metrics
	crMetrics.Registry.MustRegister(
		GCDurationMilliseconds,
		TracejobGCExecutedTotal,
		TracejobGCTracejobListErrorTotal,
		TracejobGCStatusUpdateErrorTotal,
		TracejobGCTracejobDeleteErrorTotal,
		TracejobGCTracejobDeleteTotal,
		TracejobGCTracejobJobsCount,
	)
}
