package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestValidatedWhisperAPIURLRequiresAllowlistedHost(t *testing.T) {
	t.Setenv("WHISPER_API_URL", "http://169.254.169.254/latest/meta-data")
	t.Setenv("WHISPER_ALLOWED_HOSTS", "")
	if _, err := validatedWhisperAPIURL(); err == nil {
		t.Fatal("metadata host must not be accepted")
	}
	t.Setenv("WHISPER_API_URL", "http://whisper:80")
	if got, err := validatedWhisperAPIURL(); err != nil || got != "http://whisper:80" {
		t.Fatalf("default internal host rejected: %q %v", got, err)
	}
	t.Setenv("WHISPER_API_URL", "http://models.internal:8080")
	t.Setenv("WHISPER_ALLOWED_HOSTS", "models.internal")
	if _, err := validatedWhisperAPIURL(); err != nil {
		t.Fatalf("explicit allowlist rejected: %v", err)
	}
}

func TestDownloadWhisperModelRejectsUnsafeModelPathBeforeNetwork(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("WHISPER_API_URL", "http://127.0.0.1:1")
	router := gin.New()
	router.POST("/download", HandleDownloadWhisperModel())
	for _, body := range []string{
		`{"path":"../../etc/passwd"}`,
		`{"path":"https://attacker.invalid/model"}`,
		`{"path":".."}`,
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/download", bytes.NewBufferString(body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body %s status=%d response=%s", body, response.Code, response.Body.String())
		}
	}
}

func TestGetWhisperModelsSanitizesUpstreamErrors(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal endpoint secret"))
	}))
	defer upstream.Close()
	t.Setenv("WHISPER_API_URL", upstream.URL)
	t.Setenv("WHISPER_ALLOWED_HOSTS", "127.0.0.1")
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/models", HandleGetWhisperModels())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/models", nil))
	if response.Code != http.StatusBadGateway {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if bytes.Contains(response.Body.Bytes(), []byte("internal endpoint secret")) || bytes.Contains(response.Body.Bytes(), []byte(upstream.URL)) {
		t.Fatalf("upstream detail leaked: %s", response.Body.String())
	}
}

func TestGetWhisperModelsRejectsOversizedResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(bytes.Repeat([]byte("x"), (1<<20)+1))
	}))
	defer upstream.Close()
	t.Setenv("WHISPER_API_URL", upstream.URL)
	t.Setenv("WHISPER_ALLOWED_HOSTS", "127.0.0.1")
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/models", HandleGetWhisperModels())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/models", nil))
	if response.Code != http.StatusBadGateway || !bytes.Contains(response.Body.Bytes(), []byte("too large")) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
