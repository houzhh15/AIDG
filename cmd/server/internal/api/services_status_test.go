package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleServicesStatus_OrchestratorNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/services/status", nil)

	handler := HandleServicesStatus(nil)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ServicesStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.False(t, response.WhisperAvailable)
	assert.False(t, response.DepsServiceAvailable)
}

func TestHandleServicesStatus_CLI_Mode_WhisperAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建临时文件模拟 whisper 可执行文件
	tempDir := t.TempDir()
	whisperPath := filepath.Join(tempDir, "whisper")
	err := os.WriteFile(whisperPath, []byte("#!/bin/bash\necho 'whisper'"), 0755)
	require.NoError(t, err)

	// 设置环境变量
	oldWhisperMode := os.Getenv("WHISPER_MODE")
	oldWhisperPath := os.Getenv("WHISPER_PROGRAM_PATH")
	defer func() {
		os.Setenv("WHISPER_MODE", oldWhisperMode)
		os.Setenv("WHISPER_PROGRAM_PATH", oldWhisperPath)
	}()
	os.Setenv("WHISPER_MODE", "cli")
	os.Setenv("WHISPER_PROGRAM_PATH", whisperPath)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/services/status", nil)

	handler := HandleServicesStatus(nil)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ServicesStatusResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.WhisperAvailable)
	assert.Equal(t, "cli_available", response.WhisperMode)
}

func TestHandleServicesStatus_CLI_Mode_WhisperUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 设置不存在的文件路径
	oldWhisperMode := os.Getenv("WHISPER_MODE")
	oldWhisperPath := os.Getenv("WHISPER_PROGRAM_PATH")
	defer func() {
		os.Setenv("WHISPER_MODE", oldWhisperMode)
		os.Setenv("WHISPER_PROGRAM_PATH", oldWhisperPath)
	}()
	os.Setenv("WHISPER_MODE", "cli")
	os.Setenv("WHISPER_PROGRAM_PATH", "/nonexistent/path/to/whisper")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/services/status", nil)

	handler := HandleServicesStatus(nil)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ServicesStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.False(t, response.WhisperAvailable)
	assert.Equal(t, "cli_unavailable", response.WhisperMode)
}

func TestHandleServicesStatus_CLI_Mode_NoPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 不设置 WHISPER_PROGRAM_PATH
	oldWhisperMode := os.Getenv("WHISPER_MODE")
	oldWhisperPath := os.Getenv("WHISPER_PROGRAM_PATH")
	defer func() {
		os.Setenv("WHISPER_MODE", oldWhisperMode)
		os.Setenv("WHISPER_PROGRAM_PATH", oldWhisperPath)
	}()
	os.Setenv("WHISPER_MODE", "cli")
	os.Unsetenv("WHISPER_PROGRAM_PATH")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/services/status", nil)

	handler := HandleServicesStatus(nil)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ServicesStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.False(t, response.WhisperAvailable)
	assert.Equal(t, "cli_no_path", response.WhisperMode)
}

func TestHandleServicesStatus_Local_Mode_DepsAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建临时文件模拟脚本
	tempDir := t.TempDir()
	diarizationPath := filepath.Join(tempDir, "diarize.py")
	embeddingPath := filepath.Join(tempDir, "embed.py")

	err := os.WriteFile(diarizationPath, []byte("#!/usr/bin/env python3\nprint('diarize')"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(embeddingPath, []byte("#!/usr/bin/env python3\nprint('embed')"), 0755)
	require.NoError(t, err)

	// 设置环境变量
	oldDepMode := os.Getenv("DEPENDENCY_MODE")
	oldDiarizationPath := os.Getenv("DIARIZATION_SCRIPT_PATH")
	oldEmbeddingPath := os.Getenv("EMBEDDING_SCRIPT_PATH")
	defer func() {
		os.Setenv("DEPENDENCY_MODE", oldDepMode)
		os.Setenv("DIARIZATION_SCRIPT_PATH", oldDiarizationPath)
		os.Setenv("EMBEDDING_SCRIPT_PATH", oldEmbeddingPath)
	}()
	os.Setenv("DEPENDENCY_MODE", "local")
	os.Setenv("DIARIZATION_SCRIPT_PATH", diarizationPath)
	os.Setenv("EMBEDDING_SCRIPT_PATH", embeddingPath)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/services/status", nil)

	handler := HandleServicesStatus(nil)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ServicesStatusResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.DepsServiceAvailable)
	assert.Equal(t, "local_available", response.DependencyMode)
}

func TestHandleServicesStatus_Local_Mode_DepsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 设置不存在的文件路径
	oldDepMode := os.Getenv("DEPENDENCY_MODE")
	oldDiarizationPath := os.Getenv("DIARIZATION_SCRIPT_PATH")
	oldEmbeddingPath := os.Getenv("EMBEDDING_SCRIPT_PATH")
	defer func() {
		os.Setenv("DEPENDENCY_MODE", oldDepMode)
		os.Setenv("DIARIZATION_SCRIPT_PATH", oldDiarizationPath)
		os.Setenv("EMBEDDING_SCRIPT_PATH", oldEmbeddingPath)
	}()
	os.Setenv("DEPENDENCY_MODE", "local")
	os.Setenv("DIARIZATION_SCRIPT_PATH", "/nonexistent/diarize.py")
	os.Setenv("EMBEDDING_SCRIPT_PATH", "/nonexistent/embed.py")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/services/status", nil)

	handler := HandleServicesStatus(nil)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ServicesStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.False(t, response.DepsServiceAvailable)
	assert.Equal(t, "local_unavailable", response.DependencyMode)
}

func TestHandleServicesStatus_Local_Mode_PartialScripts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 只设置一个脚本存在
	tempDir := t.TempDir()
	diarizationPath := filepath.Join(tempDir, "diarize.py")
	err := os.WriteFile(diarizationPath, []byte("#!/usr/bin/env python3\nprint('diarize')"), 0755)
	require.NoError(t, err)

	oldDepMode := os.Getenv("DEPENDENCY_MODE")
	oldDiarizationPath := os.Getenv("DIARIZATION_SCRIPT_PATH")
	oldEmbeddingPath := os.Getenv("EMBEDDING_SCRIPT_PATH")
	defer func() {
		os.Setenv("DEPENDENCY_MODE", oldDepMode)
		os.Setenv("DIARIZATION_SCRIPT_PATH", oldDiarizationPath)
		os.Setenv("EMBEDDING_SCRIPT_PATH", oldEmbeddingPath)
	}()
	os.Setenv("DEPENDENCY_MODE", "local")
	os.Setenv("DIARIZATION_SCRIPT_PATH", diarizationPath)
	os.Setenv("EMBEDDING_SCRIPT_PATH", "/nonexistent/embed.py")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/services/status", nil)

	handler := HandleServicesStatus(nil)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ServicesStatusResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// 应该不可用，因为两个脚本都需要存在
	assert.False(t, response.DepsServiceAvailable)
	assert.Equal(t, "local_unavailable", response.DependencyMode)
}

func TestHandleServicesStatus_DefaultModes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 不设置任何环境变量，使用默认值
	oldWhisperMode := os.Getenv("WHISPER_MODE")
	oldDepMode := os.Getenv("DEPENDENCY_MODE")
	defer func() {
		os.Setenv("WHISPER_MODE", oldWhisperMode)
		os.Setenv("DEPENDENCY_MODE", oldDepMode)
	}()
	os.Unsetenv("WHISPER_MODE")
	os.Unsetenv("DEPENDENCY_MODE")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/services/status", nil)

	handler := HandleServicesStatus(nil)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ServicesStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// 默认模式应该是服务模式，但由于 orchestrator 为 nil，应该都不可用
	assert.False(t, response.WhisperAvailable)
	assert.False(t, response.DepsServiceAvailable)
	assert.Equal(t, "service_unavailable", response.WhisperMode)
	assert.Equal(t, "service_unavailable", response.DependencyMode)
}
