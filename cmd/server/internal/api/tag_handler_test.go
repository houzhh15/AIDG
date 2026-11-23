package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestExecutionPlanTagMutatorsAreReadOnly(t *testing.T) {
	root := t.TempDir()
	handler := NewTagHandler(root)
	gin.SetMode(gin.TestMode)
	tests := []struct {
		method  string
		path    string
		handler gin.HandlerFunc
	}{
		{http.MethodPost, "/tags", handler.CreateTag},
		{http.MethodPost, "/tags/tag/switch", handler.SwitchTag},
		{http.MethodDelete, "/tags/tag", handler.DeleteTag},
	}
	for _, tc := range tests {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			router := gin.New()
			router.Handle(tc.method, "/api/v1/projects/:id/tasks/:task_id/docs/:docType"+tc.path, tc.handler)
			request := httptest.NewRequest(tc.method, "/api/v1/projects/p/tasks/t/docs/execution-plan"+tc.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusConflict || response.Body.String() == "" {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if _, err := os.Stat(filepath.Join(root, "p")); !os.IsNotExist(err) {
				t.Fatalf("tag mutator changed storage: %v", err)
			}
		})
	}
}

func TestExecutionPlanTagReadDoesNotInitializeStorage(t *testing.T) {
	root := t.TempDir()
	handler := NewTagHandler(root)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/projects/:id/tasks/:task_id/docs/:docType/tags", handler.ListTags)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/projects/p/tasks/t/docs/execution-plan/tags", nil)
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK && response.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if _, err := os.Stat(filepath.Join(root, "p")); !os.IsNotExist(err) {
		t.Fatalf("tag read initialized storage: %v", err)
	}
}
