package documents

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestImportFile 测试文件导入处理器
func TestImportFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建临时目录用于测试
	tempDir, err := os.MkdirTemp("", "import_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	handler := NewHandler(tempDir)

	// 创建项目目录
	projectDir := filepath.Join(tempDir, "test-project", "documents")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	tests := []struct {
		name           string
		filename       string
		content        []byte
		contentType    string
		expectedStatus int
		expectSuccess  bool
	}{
		{
			name:           "SVG file import",
			filename:       "test.svg",
			content:        []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><circle cx="50" cy="50" r="40"/></svg>`),
			contentType:    "image/svg+xml",
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
		},
		{
			name:           "Unsupported file type",
			filename:       "test.txt",
			content:        []byte("plain text content"),
			contentType:    "text/plain",
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name:           "Empty filename",
			filename:       "",
			content:        []byte("some content"),
			contentType:    "application/octet-stream",
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建 multipart form
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			if tt.filename != "" {
				part, err := writer.CreateFormFile("file", tt.filename)
				if err != nil {
					t.Fatalf("Failed to create form file: %v", err)
				}
				if _, err := io.Copy(part, bytes.NewReader(tt.content)); err != nil {
					t.Fatalf("Failed to copy content: %v", err)
				}
			}
			writer.Close()

			// 创建请求
			req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/test-project/documents/import", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			// 创建响应记录器
			rec := httptest.NewRecorder()

			// 创建 Gin 路由
			router := gin.New()
			router.POST("/api/v1/projects/:id/documents/import", handler.ImportFile)

			// 执行请求
			router.ServeHTTP(rec, req)

			// 验证状态码
			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, rec.Code, rec.Body.String())
			}

			// 对于成功的情况，验证响应内容
			if tt.expectSuccess && rec.Code == http.StatusOK {
				var response ImportFileResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				if !response.Success {
					t.Error("Expected success to be true")
				}

				if response.OriginalFilename != tt.filename {
					t.Errorf("Expected filename %s, got %s", tt.filename, response.OriginalFilename)
				}

				if response.Content == "" {
					t.Error("Expected non-empty content")
				}
			}
		})
	}
}

// TestSupportedExtensions 测试支持的文件扩展名
func TestSupportedExtensions(t *testing.T) {
	// 注意：supportedExtensions 使用不带点的扩展名
	supportedExts := map[string]bool{
		"pdf":  true,
		"pptx": true,
		"ppt":  true,
		"docx": true,
		"doc":  true,
		"xlsx": true,
		"xls":  true,
		"svg":  true,
	}

	for ext, expected := range supportedExts {
		_, ok := supportedExtensions[ext]
		if ok != expected {
			t.Errorf("Extension %s: expected supported=%v, got %v", ext, expected, ok)
		}
	}

	// 测试不支持的扩展名
	unsupportedExts := []string{"txt", "jpg", "png", "mp4", "zip"}
	for _, ext := range unsupportedExts {
		if _, ok := supportedExtensions[ext]; ok {
			t.Errorf("Extension %s should not be supported", ext)
		}
	}
}

// TestFileSizeLimit 测试文件大小限制
func TestFileSizeLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tempDir, err := os.MkdirTemp("", "size_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	handler := NewHandler(tempDir)

	// 创建项目目录
	projectDir := filepath.Join(tempDir, "test-project", "documents")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// 创建超过 20MB 的文件
	largeContent := make([]byte, maxFileSize+1)
	for i := range largeContent {
		largeContent[i] = 'x'
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "large.pdf")
	part.Write(largeContent)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/test-project/documents/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rec := httptest.NewRecorder()

	router := gin.New()
	router.POST("/api/v1/projects/:id/documents/import", handler.ImportFile)

	router.ServeHTTP(rec, req)

	// 400 或 413 都是对超大文件的有效响应
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 400 or 413 for oversized file, got %d", rec.Code)
	}
}
