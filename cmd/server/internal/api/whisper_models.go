package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./config"
	}

	jsonPath := filepath.Join(configPath, "whisper_models.json")
	fmt.Printf("[DEBUG] Loading whisper models from: %s\n", jsonPath)

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read whisper_models.json from %s: %w", jsonPath, err)
	}

	var metadata WhisperModelsMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse whisper_models.json: %w", err)
	}

	fmt.Printf("[DEBUG] Loaded %d models from JSON\n", len(metadata.Models))

	return &metadata, nil
}

// getDownloadedModels queries the Whisper service to get list of downloaded models
func getDownloadedModels(whisperAPIURL string) (map[string]bool, error) {
	client := &http.Client{Timeout: 5 * time.Second}
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

	var modelsResp WhisperModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, err
	}

	downloaded := make(map[string]bool)
	for _, model := range modelsResp.Models {
		downloaded[model.ID] = true
	}

	return downloaded, nil
}

// HandleGetWhisperModels returns available Whisper models
// GET /api/v1/services/whisper/models
func HandleGetWhisperModels() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get Whisper API URL from environment
		whisperAPIURL := os.Getenv("WHISPER_API_URL")
		if whisperAPIURL == "" {
			whisperAPIURL = "http://whisper:80" // default
		}

		// Create HTTP client with timeout
		client := &http.Client{
			Timeout: 5 * time.Second,
		}

		// Request models from Whisper API
		endpoint := fmt.Sprintf("%s/api/whisper/model", whisperAPIURL)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   fmt.Sprintf("Failed to create request: %v", err),
			})
			return
		}

		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"error":   fmt.Sprintf("Failed to connect to Whisper service: %v", err),
				"hint":    "Make sure Whisper service is running",
			})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			c.JSON(resp.StatusCode, gin.H{
				"success": false,
				"error":   fmt.Sprintf("Whisper API returned status %d: %s", resp.StatusCode, string(bodyBytes)),
			})
			return
		}

		// Parse response
		var modelsResp WhisperModelsResponse
		if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   fmt.Sprintf("Failed to parse response: %v", err),
			})
			return
		}

		// Return success with models
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"models":     modelsResp.Models,
				"total":      len(modelsResp.Models),
				"api_url":    whisperAPIURL,
				"fetched_at": time.Now().Format(time.RFC3339),
			},
		})
	}
}

// HandleGetWhisperModelsExtended returns available Whisper models with size information
// GET /api/v1/services/whisper/models-extended
func HandleGetWhisperModelsExtended() gin.HandlerFunc {
	return func(c *gin.Context) {
		fmt.Println("[DEBUG] HandleGetWhisperModelsExtended called")

		// Load models metadata from JSON file
		metadata, err := loadWhisperModelsMetadata()
		if err != nil {
			fmt.Printf("[ERROR] Failed to load metadata: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   fmt.Sprintf("Failed to load models metadata: %v", err),
			})
			return
		}

		fmt.Printf("[DEBUG] Loaded %d models from JSON\n", len(metadata.Models))

		// Get Whisper API URL
		whisperAPIURL := os.Getenv("WHISPER_API_URL")
		if whisperAPIURL == "" {
			whisperAPIURL = "http://whisper:80"
		}

		// Check which models are downloaded
		downloadedModels, err := getDownloadedModels(whisperAPIURL)
		if err != nil {
			fmt.Printf("[WARN] Failed to get downloaded models: %v\n", err)
			// If we can't reach whisper service, just return all models as not downloaded
			downloadedModels = make(map[string]bool)
		}

		fmt.Printf("[DEBUG] Downloaded models map: %v\n", downloadedModels)

		// Update exists flag
		models := metadata.Models
		for i := range models {
			models[i].Exists = downloadedModels[models[i].ID]
		}

		fmt.Printf("[DEBUG] Returning %d models to client\n", len(models))

		// Return success with models
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"models":     models,
				"total":      len(models),
				"api_url":    whisperAPIURL,
				"fetched_at": time.Now().Format(time.RFC3339),
			},
		})
	}
}

// HandleDownloadWhisperModel downloads a Whisper model via streaming API
// POST /api/v1/services/whisper/models/download
func HandleDownloadWhisperModel() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Path string `json:"path" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid request: path is required",
			})
			return
		}

		// Get Whisper API URL from environment
		whisperAPIURL := os.Getenv("WHISPER_API_URL")
		if whisperAPIURL == "" {
			whisperAPIURL = "http://whisper:80" // default
		}

		// Create HTTP client with longer timeout for downloads
		client := &http.Client{
			Timeout: 30 * time.Minute, // Allow up to 30 minutes for large model downloads
		}

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
				"error":   fmt.Sprintf("Failed to create request: %v", err),
			})
			return
		}
		reqHTTP.Header.Set("Content-Type", "application/json")
		reqHTTP.Header.Set("Accept", "text/event-stream") // Enable streaming progress

		resp, err := client.Do(reqHTTP)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"error":   fmt.Sprintf("Failed to connect to Whisper service: %v", err),
				"hint":    "Make sure Whisper service is running",
			})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			bodyBytes, _ := io.ReadAll(resp.Body)
			c.JSON(resp.StatusCode, gin.H{
				"success": false,
				"error":   fmt.Sprintf("Whisper API returned status %d: %s", resp.StatusCode, string(bodyBytes)),
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
