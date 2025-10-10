package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// AudioChunksTotal 音频切片处理总数计数器
	// Labels: component (asr/sd/embedding/merge), status (success/error)
	AudioChunksTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aidg_audio_chunks_total",
			Help: "Total number of audio chunks processed by component",
		},
		[]string{"component", "status"},
	)

	// AudioErrorsTotal 音频处理错误总数计数器
	// Labels: component (asr/sd/embedding/merge), error_code (ENV_NOT_READY/WHISPER_UNAVAILABLE/...)
	AudioErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aidg_audio_errors_total",
			Help: "Total number of audio processing errors by component and error code",
		},
		[]string{"component", "error_code"},
	)

	// EnvironmentReady 环境就绪状态量规（0=未就绪，1=就绪）
	EnvironmentReady = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "aidg_environment_ready",
			Help: "Environment readiness status (0=not ready, 1=ready)",
		},
	)

	// AudioProcessingDuration 音频处理耗时直方图（秒）
	// Labels: component (asr/sd/embedding/merge)
	// Buckets: 0.1s, 0.5s, 1s, 2s, 5s, 10s, 30s, 60s, 120s, 300s
	AudioProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aidg_audio_processing_duration_seconds",
			Help:    "Audio processing duration in seconds by component",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300},
		},
		[]string{"component"},
	)
)

// RecordChunkProcessed 记录音频切片处理完成
func RecordChunkProcessed(component string, success bool) {
	status := "success"
	if !success {
		status = "error"
	}
	AudioChunksTotal.WithLabelValues(component, status).Inc()
}

// RecordError 记录音频处理错误
func RecordError(component, errorCode string) {
	AudioErrorsTotal.WithLabelValues(component, errorCode).Inc()
}

// SetEnvironmentReady 设置环境就绪状态
func SetEnvironmentReady(ready bool) {
	if ready {
		EnvironmentReady.Set(1)
	} else {
		EnvironmentReady.Set(0)
	}
}

// RecordDuration 记录音频处理耗时（秒）
func RecordDuration(component string, durationSeconds float64) {
	AudioProcessingDuration.WithLabelValues(component).Observe(durationSeconds)
}
