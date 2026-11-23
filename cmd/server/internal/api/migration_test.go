package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
)

func migrationAPIFixture(t *testing.T) (*projects.ProjectRegistry, string) {
	t.Helper()
	root := t.TempDir()
	projects.ProjectsRoot = root
	projects.ProjectsStatePath = filepath.Join(root, "server_projects.json")
	registry := projects.NewProjectRegistry()
	now := time.Now().UTC()
	registry.Set(&projects.Project{ID: "p", Name: "p", CreatedAt: now, UpdatedAt: now})
	if err := os.MkdirAll(filepath.Join(root, "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "p", "tasks.json"), []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return registry, root
}

func TestMigrationPreviewRejectsUnknownFieldsAndAcceptsEmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registry, _ := migrationAPIFixture(t)
	router := gin.New()
	router.POST("/api/v1/projects/:id/migrations/schema/preview", HandlePreviewProjectMigration(registry))

	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p/migrations/schema/preview", bytes.NewBufferString(`{"unexpected":true}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status=%d body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/projects/p/migrations/schema/preview", nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("empty preview status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Plan struct {
			Digest string `json:"digest"`
		} `json:"plan"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload.Plan.Digest == "" {
		t.Fatalf("invalid plan response: %s (%v)", response.Body.String(), err)
	}
}

func TestMigrationExecuteRejectsRoutePlanMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registry, _ := migrationAPIFixture(t)
	router := gin.New()
	router.POST("/api/v1/projects/:id/migrations/schema/execute", HandleExecuteProjectMigration(registry))
	body := `{"plan":{"plan_version":"1","scope":"project","project_id":"other"}}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p/migrations/schema/execute", bytes.NewBufferString(body))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("mismatch status=%d body=%s", response.Code, response.Body.String())
	}
}
