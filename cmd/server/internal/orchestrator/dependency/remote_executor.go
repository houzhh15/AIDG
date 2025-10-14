package dependency

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// RemoteExecutor executes commands via HTTP API calls to a dependency service.
// It is suitable for production environments where dependencies run in separate containers.
type RemoteExecutor struct {
	config     ExecutorConfig
	httpClient *http.Client
}

// NewRemoteExecutor creates a new RemoteExecutor with the given configuration.
func NewRemoteExecutor(config ExecutorConfig) *RemoteExecutor {
	return &RemoteExecutor{
		config: config,
		httpClient: &http.Client{
			// HTTP timeout should be slightly larger than command timeout
			Timeout: config.DefaultTimeout + 10*time.Second,
		},
	}
}

// ExecuteCommand executes a command remotely via HTTP POST /api/v1/execute.
func (e *RemoteExecutor) ExecuteCommand(ctx context.Context, req CommandRequest) (CommandResponse, error) {
	// 1. Serialize request to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return CommandResponse{}, fmt.Errorf("failed to serialize request: %w", err)
	}

	// DEBUG: Log the request being sent
	log.Printf("[RemoteExecutor] Sending request to %s: %s", e.config.ServiceURL, string(reqBody))

	// 2. Build HTTP request
	url := fmt.Sprintf("%s/api/v1/execute", e.config.ServiceURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return CommandResponse{}, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// 3. Send request
	start := time.Now()
	httpResp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return CommandResponse{}, fmt.Errorf("failed to call dependency service (network error): %w", err)
	}
	defer httpResp.Body.Close()

	// 4. Parse response
	var resp CommandResponse
	bodyBytes, _ := io.ReadAll(httpResp.Body)
	if err := json.Unmarshal(bodyBytes, &resp); err != nil {
		log.Printf("[RemoteExecutor] Failed to parse response body: %s, error: %v", string(bodyBytes), err)
		return CommandResponse{}, fmt.Errorf("failed to parse response: %w", err)
	}

	// 5. Check HTTP status code
	if httpResp.StatusCode != http.StatusOK {
		log.Printf("[RemoteExecutor] HTTP %d error, response body: %s", httpResp.StatusCode, string(bodyBytes))
		return resp, fmt.Errorf("dependency service returned error (HTTP %d): %s", httpResp.StatusCode, resp.Stderr)
	}

	// 6. Fill duration if service didn't provide it
	if resp.Duration == 0 {
		resp.Duration = time.Since(start)
	}

	return resp, nil
}

// HealthCheck verifies that the remote dependency service is reachable and healthy.
func (e *RemoteExecutor) HealthCheck(ctx context.Context) error {
	// Call remote health check endpoint
	url := fmt.Sprintf("%s/api/v1/health", e.config.ServiceURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("dependency service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dependency service unhealthy (HTTP %d)", resp.StatusCode)
	}

	return nil
}
