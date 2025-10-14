package main

import (
	"encoding/json"
	"net/http"
)

const Version = "1.0.0"

// Handler holds dependencies for HTTP request handling.
type Handler struct {
	validator   *Validator
	executor    *Executor
	auditLogger *AuditLogger
	limiter     *ConcurrencyLimiter
}

// NewHandler creates a new HTTP handler with all dependencies.
// It registers routes for command execution and health checks.
func NewHandler(validator *Validator, executor *Executor, auditLogger *AuditLogger, limiter *ConcurrencyLimiter) http.Handler {
	h := &Handler{
		validator:   validator,
		executor:    executor,
		auditLogger: auditLogger,
		limiter:     limiter,
	}

	// Create router
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/execute", h.HandleExecute)
	mux.HandleFunc("/api/v1/health", h.HandleHealth)

	return mux
}

// HandleExecute processes command execution requests.
// It validates the request, acquires a concurrency slot, executes the command,
// and logs the result to the audit log.
func (h *Handler) HandleExecute(w http.ResponseWriter, r *http.Request) {
	// Check method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req CommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", []string{"Failed to decode JSON: " + err.Error()})
		return
	}

	// Validate request
	if err := h.validator.ValidateRequest(req); err != nil {
		// Log rejection to audit log
		h.auditLogger.LogRejection(req, err.Error(), r.RemoteAddr)
		respondError(w, http.StatusBadRequest, "invalid_arguments", []string{err.Error()})
		return
	}

	// Acquire concurrency slot
	if err := h.limiter.Acquire(r.Context(), req.Command); err != nil {
		respondError(w, http.StatusServiceUnavailable, "service_busy", []string{"Max concurrent executions reached"})
		return
	}
	defer h.limiter.Release(req.Command)

	// Execute command
	resp, err := h.executor.ExecuteCommand(r.Context(), req)

	// Log execution to audit log
	h.auditLogger.LogExecution(req, resp, err, r.RemoteAddr)

	// Handle execution error
	if err != nil {
		respondError(w, http.StatusInternalServerError, "command_failed", []string{err.Error()})
		return
	}

	// Handle non-zero exit code
	if resp.ExitCode != 0 {
		resp.Success = false
		respondJSON(w, http.StatusInternalServerError, resp)
		return
	}

	// Success
	resp.Success = true
	respondJSON(w, http.StatusOK, resp)
}

// HandleHealth handles health check requests.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	// Check method
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return health status
	response := map[string]interface{}{
		"status":  "ok",
		"service": "command-executor",
		"version": Version,
	}

	respondJSON(w, http.StatusOK, response)
}

// respondJSON writes a JSON response with the given status code.
func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// respondError writes a JSON error response with the given status code.
func respondError(w http.ResponseWriter, statusCode int, errorType string, details []string) {
	response := map[string]interface{}{
		"error":   errorType,
		"details": details,
	}
	respondJSON(w, statusCode, response)
}
