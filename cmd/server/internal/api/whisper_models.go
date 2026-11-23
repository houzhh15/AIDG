package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

// WhisperModel represents a Whisper model from the API
type WhisperModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Path    string `json:"path"`
	Created int64  `json:"created"`
}

// WhisperModelInfo represents extended model information including size
type WhisperModelInfo struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`        // Size in bytes
	SizeMB      string `json:"size_mb"`     // Size in MB for display
	Exists      bool   `json:"exists"`      // Whether model is downloaded
	Description string `json:"description"` // Model description
}

// WhisperModelsResponse represents the response from Whisper models API
type WhisperModelsResponse struct {
	Object string         `json:"object"`
	Models []WhisperModel `json:"models"`
}

// WhisperModelsMetadata represents the static models configuration
type WhisperModelsMetadata struct {
	Models []WhisperModelInfo `json:"models"`
}

// loadWhisperModelsMetadata loads the static whisper models configuration from JSON file
func loadWhisperModelsMetadata() (*WhisperModelsMetadata, error) {
	// Try to load from config directory
	// In Docker, CONFIG_PATH should be set to /app/config
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		// Default to /app/config for Docker, fallback to ./config for local development
		if _, err := os.Stat("/app/config"); err == nil {
			configPath = "/app/config"
		} else {
			configPath = "./config"
		}
	}

	jsonPath := filepath.Join(configPath, "whisper_models.json")
	info, err := os.Lstat(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect whisper model metadata: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > 1<<20 {
		return nil, fmt.Errorf("whisper model metadata is not a bounded regular file")
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read whisper model metadata: %w", err)
	}

	var metadata WhisperModelsMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse whisper_models.json: %w", err)
	}

	return &metadata, nil
}

// getDownloadedModels queries the Whisper service to get list of downloaded models
func getDownloadedModels(whisperAPIURL string) (map[string]bool, error) {
	client := whisperHTTPClient(5 * time.Second)
	endpoint := fmt.Sprintf("%s/api/whisper/model", whisperAPIURL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("whisper API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return nil, fmt.Errorf("Whisper model response is too large")
	}
	var modelsResp WhisperModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("invalid Whisper model response")
	}

	downloaded := make(map[string]bool)
	for _, model := range modelsResp.Models {
		downloaded[model.ID] = true
	}

	return downloaded, nil
}

// HandleGetWhisperModels returns available Whisper models
// GET /api/v1/services/whisper/models
var whisperModelPathPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func validatedWhisperAPIURL() (string, error) {
	raw := strings.TrimSpace(os.Getenv("WHISPER_API_URL"))
	if raw == "" {
		raw = "http://whisper:80"
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("invalid Whisper service URL")
	}
	allowed := map[string]struct{}{"whisper": {}, "localhost": {}, "127.0.0.1": {}, "::1": {}}
	for _, host := range strings.Split(os.Getenv("WHISPER_ALLOWED_HOSTS"), ",") {
		if host = strings.ToLower(strings.TrimSpace(host)); host != "" {
			allowed[host] = struct{}{}
		}
	}
	if _, ok := allowed[strings.ToLower(parsed.Hostname())]; !ok {
		return "", fmt.Errorf("Whisper service host is not allowlisted")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func whisperHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func HandleGetWhisperModels() gin.HandlerFunc {
	return func(c *gin.Context) {
		whisperAPIURL, err := validatedWhisperAPIURL()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Whisper service is not safely configured"})
			return
		}
		client := whisperHTTPClient(5 * time.Second)

		// Request models from Whisper API
		endpoint := fmt.Sprintf("%s/api/whisper/model", whisperAPIURL)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to create Whisper request",
			})
			return
		}

		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"error":   "Whisper service is unavailable",
			})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			c.JSON(http.StatusBadGateway, gin.H{
				"success": false,
				"error":   "Whisper service returned an error",
			})
			return
		}

		// External responses are bounded and parse errors are never reflected.
		body, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
		if err != nil || len(body) > 1<<20 {
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": "Whisper service response is too large"})
			return
		}
		var modelsResp WhisperModelsResponse
		if err := json.Unmarshal(body, &modelsResp); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{
				"success": false,
				"error":   "Whisper service returned an invalid response",
			})
			return
		}

		// Return success with models
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"models":     modelsResp.Models,
				"total":      len(modelsResp.Models),
				"fetched_at": time.Now().Format(time.RFC3339),
			},
		})
	}
}

// HandleGetWhisperModelsExtended returns available Whisper models with size information
// GET /api/v1/services/whisper/models-extended
func HandleGetWhisperModelsExtended() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Load models metadata from JSON file
		metadata, err := loadWhisperModelsMetadata()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Whisper model metadata is unavailable",
			})
			return
		}

		whisperAPIURL, err := validatedWhisperAPIURL()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Whisper service is not safely configured"})
			return
		}

		// Check which models are downloaded
		downloadedModels, err := getDownloadedModels(whisperAPIURL)
		if err != nil {
			// Keep diagnostics generic: transport errors may contain the internal endpoint.
			downloadedModels = make(map[string]bool)
		}

		// Update exists flag
		models := metadata.Models
		for i := range models {
			models[i].Exists = downloadedModels[models[i].ID]
		}

		// Return success with models
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"models":     models,
				"total":      len(models),
				"fetched_at": time.Now().Format(time.RFC3339),
			},
		})
	}
}

// HandleDownloadWhisperModel downloads a Whisper model via streaming API
// POST /api/v1/services/whisper/models/download
func HandleDownloadWhisperModel() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
		var req struct {
			Path string `json:"path" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request: path is required"})
			return
		}
		if !utf8.ValidString(req.Path) || !whisperModelPathPattern.MatchString(req.Path) || req.Path == "." || req.Path == ".." {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid model path"})
			return
		}
		whisperAPIURL, err := validatedWhisperAPIURL()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Whisper service is not safely configured"})
			return
		}
		client := whisperHTTPClient(30 * time.Minute)

		// Send download request to Whisper API
		// go-whisper API expects POST /api/whisper/model with {"model": "filename"}
		// Add Accept: text/event-stream header for streaming progress
		endpoint := fmt.Sprintf("%s/api/whisper/model", whisperAPIURL)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		// Create request body - go-whisper expects "model" field, not "path"
		reqBody := map[string]string{"model": req.Path}
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   fmt.Sprintf("Failed to marshal request: %v", err),
			})
			return
		}

		reqHTTP, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(string(jsonData)))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to create Whisper request",
			})
			return
		}
		reqHTTP.Header.Set("Content-Type", "application/json")
		reqHTTP.Header.Set("Accept", "text/event-stream") // Enable streaming progress

		resp, err := client.Do(reqHTTP)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"error":   "Whisper service is unavailable",
			})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			c.JSON(http.StatusBadGateway, gin.H{
				"success": false,
				"error":   "Whisper service rejected the model request",
			})
			return
		}

		// Set headers for SSE
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")

		// Stream the response back to client
		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				if _, writeErr := c.Writer.Write(buf[:n]); writeErr != nil {
					return
				}
				c.Writer.Flush()
			}
			if err != nil {
				if err == io.EOF {
					break
				}
				return
			}
		}
	}
}
