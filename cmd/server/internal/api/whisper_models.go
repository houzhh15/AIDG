package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

// WhisperModelsResponse represents the response from Whisper models API
type WhisperModelsResponse struct {
	Object string         `json:"object"`
	Models []WhisperModel `json:"models"`
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
