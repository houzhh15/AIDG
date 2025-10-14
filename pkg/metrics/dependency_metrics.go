// Package metrics provides Prometheus metrics for monitoring AIDG components.
package metrics

import "github.com/prometheus/client_golang/prometheus"

// Dependency execution metrics
var (
	// commandExecutionTotal records the total number of dependency command executions.
	// Labels:
	//   - command: Command name (e.g., "ffmpeg", "pyannote")
	//   - mode: Execution mode (e.g., "local", "remote", "fallback")
	//   - status: Execution status (e.g., "success", "failed", "timeout")
	commandExecutionTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dependency_command_executions_total",
			Help: "Total number of dependency command executions",
		},
		[]string{"command", "mode", "status"},
	)

	// commandExecutionDuration records the duration of dependency command executions.
	// Labels:
	//   - command: Command name (e.g., "ffmpeg", "pyannote")
	//   - mode: Execution mode (e.g., "local", "remote", "fallback")
	// Buckets: 0.1s, 0.5s, 1s, 5s, 10s, 30s, 60s, 300s (5 minutes)
	commandExecutionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dependency_command_duration_seconds",
			Help:    "Duration of dependency command executions in seconds",
			Buckets: []float64{0.1, 0.5, 1, 5, 10, 30, 60, 300},
		},
		[]string{"command", "mode"},
	)

	// degradationEventsTotal records the number of execution mode degradation events.
	// Labels:
	//   - from_mode: Source execution mode (e.g., "remote")
	//   - to_mode: Target execution mode (e.g., "local")
	degradationEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dependency_degradation_events_total",
			Help: "Total number of execution mode degradation events (e.g., remote -> local)",
		},
		[]string{"from_mode", "to_mode"},
	)
)

func init() {
	// Register all dependency-related metrics with Prometheus
	prometheus.MustRegister(commandExecutionTotal)
	prometheus.MustRegister(commandExecutionDuration)
	prometheus.MustRegister(degradationEventsTotal)
}

// RecordCommandExecution records a command execution event.
// Parameters:
//   - command: Command name (e.g., "ffmpeg", "pyannote")
//   - mode: Execution mode (e.g., "local", "remote", "fallback")
//   - status: Execution status (e.g., "success", "failed", "timeout")
func RecordCommandExecution(command, mode, status string) {
	commandExecutionTotal.WithLabelValues(command, mode, status).Inc()
}

// RecordCommandDuration records the duration of a command execution.
// Parameters:
//   - command: Command name (e.g., "ffmpeg", "pyannote")
//   - mode: Execution mode (e.g., "local", "remote", "fallback")
//   - durationSeconds: Execution duration in seconds
func RecordCommandDuration(command, mode string, durationSeconds float64) {
	commandExecutionDuration.WithLabelValues(command, mode).Observe(durationSeconds)
}

// RecordDegradationEvent records a degradation event.
// Parameters:
//   - fromMode: Source execution mode (e.g., "remote")
//   - toMode: Target execution mode (e.g., "local")
func RecordDegradationEvent(fromMode, toMode string) {
	degradationEventsTotal.WithLabelValues(fromMode, toMode).Inc()
}
