package telemetry

import (
	"encoding/json"
	"net/http"
)

type TelemetryHandler struct {
	tracker *LatencyTracker
}

// NewTelemetryHandler constructs a new TelemetryHandler
func NewTelemetryHandler(tracker *LatencyTracker) *TelemetryHandler {
	return &TelemetryHandler{
		tracker: tracker,
	}
}

// HandleHealth serves health status and request latency percentiles
func (h *TelemetryHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	p50, p95, p99 := h.tracker.GetPercentiles()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "OK",
		"latency": map[string]string{
			"p50": p50.String(),
			"p95": p95.String(),
			"p99": p99.String(),
		},
	})
}

// RegisterRoutes registers telemetry/health endpoints
func RegisterRoutes(mux *http.ServeMux, handler *TelemetryHandler) {
	mux.HandleFunc("/health", handler.HandleHealth)
	mux.HandleFunc("/api/v1/health", handler.HandleHealth)
}
