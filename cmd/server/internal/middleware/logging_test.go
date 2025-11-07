package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15/AIDG/pkg/logger"
)

func TestRequestLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, err := logger.Init(logger.Config{Level: "debug", Environment: "test"})
	if err != nil {
		t.Fatalf("logger init failed: %v", err)
	}

	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/ping", func(c *gin.Context) {
		if _, ok := c.Get("request_id"); !ok {
			t.Fatalf("request_id not set in context")
		}
		c.String(http.StatusOK, "pong")
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", w.Code)
	}

	if w.Header().Get("X-Request-ID") == "" {
		t.Fatalf("missing X-Request-ID header")
	}
}
