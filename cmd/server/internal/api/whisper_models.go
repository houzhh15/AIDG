package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	ID      string `json:"id"`
	Object  string `json:"object"`
	Path    string `json:"path"`
	Created int64  `json:"created"`
	Size    int64  `json:"size"`    // Size in bytes
	SizeMB  string `json:"size_mb"` // Size in MB for display
	Exists  bool   `json:"exists"`  // Whether model is downloaded
}

// WhisperModelsResponse represents the response from Whisper models API
type WhisperModelsResponse struct {
	Object string         `json:"object"`
	Models []WhisperModel `json:"models"`
}

// HuggingFaceModelInfo represents model info from HuggingFace
type HuggingFaceModelInfo struct {
	Siblings []struct {
		Rfilename string `json:"rfilename"`
		Size      int64  `json:"size"`
	} `json:"siblings"`
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
		endpoint := fmt.Sprintf("%s/api/v1/models", whisperAPIURL)
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

// getModelSizeFromHuggingFace fetches model size from HuggingFace API
func getModelSizeFromHuggingFace(modelPath string) (int64, error) {
	// Extract model name from path (e.g., "ggml-large-v3.bin" -> "ggml-large-v3")
	modelName := strings.TrimSuffix(modelPath, ".bin")

	// Map common model names to their HuggingFace paths
	modelMap := map[string]string{
		"ggml-tiny":          "openai/whisper-tiny",
		"ggml-tiny.en":       "openai/whisper-tiny.en",
		"ggml-base":          "openai/whisper-base",
		"ggml-base.en":       "openai/whisper-base.en",
		"ggml-small":         "openai/whisper-small",
		"ggml-small.en":      "openai/whisper-small.en",
		"ggml-medium":        "openai/whisper-medium",
		"ggml-medium.en":     "openai/whisper-medium.en",
		"ggml-large-v1":      "openai/whisper-large-v1",
		"ggml-large-v2":      "openai/whisper-large-v2",
		"ggml-large-v3":      "openai/whisper-large-v3",
		"ggml-large-v3-turbo": "openai/whisper-large-v3-turbo",
	}

	hfRepo, exists := modelMap[modelName]
	if !exists {
		return 0, fmt.Errorf("unknown model: %s", modelName)
	}

	// Fetch model info from HuggingFace
	url := fmt.Sprintf("https://huggingface.co/api/models/%s", hfRepo)
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HuggingFace API returned status %d", resp.StatusCode)
	}

	var hfInfo HuggingFaceModelInfo
	if err := json.NewDecoder(resp.Body).Decode(&hfInfo); err != nil {
		return 0, err
	}

	// Find the .bin file size
	for _, sibling := range hfInfo.Siblings {
		if strings.HasSuffix(sibling.Rfilename, ".bin") {
			return sibling.Size, nil
		}
	}

	return 0, fmt.Errorf("model file not found in HuggingFace repo")
}

// convertToModelInfo converts WhisperModel to WhisperModelInfo with size and existence info
func convertToModelInfo(models []WhisperModel, whisperAPIURL string) ([]WhisperModelInfo, error) {
	var modelInfos []WhisperModelInfo

	for _, model := range models {
		info := WhisperModelInfo{
			ID:      model.ID,
			Object:  model.Object,
			Path:    model.Path,
			Created: model.Created,
			Exists:  true, // If returned by API, it exists
		}

		// Get size from HuggingFace
		size, err := getModelSizeFromHuggingFace(model.Path)
		if err != nil {
			// If we can't get size, set to 0 and continue
			info.Size = 0
			info.SizeMB = "未知"
		} else {
			info.Size = size
			info.SizeMB = fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
		}

		modelInfos = append(modelInfos, info)
	}

	return modelInfos, nil
}

// HandleGetWhisperModelsExtended returns available Whisper models with size information
// GET /api/v1/services/whisper/models-extended
func HandleGetWhisperModelsExtended() gin.HandlerFunc {
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
		endpoint := fmt.Sprintf("%s/api/v1/models", whisperAPIURL)
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

		// Convert to extended model info
		modelInfos, err := convertToModelInfo(modelsResp.Models, whisperAPIURL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   fmt.Sprintf("Failed to get model sizes: %v", err),
			})
			return
		}

		// Return success with extended models
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"models":     modelInfos,
				"total":      len(modelInfos),
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
		endpoint := fmt.Sprintf("%s/api/v1/models?stream=true", whisperAPIURL)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		// Create request body
		reqBody := map[string]string{"path": req.Path}
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
