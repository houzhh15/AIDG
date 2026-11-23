package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15/AIDG/cmd/server/internal/audit"
)

type capturedAuditLogger struct {
	mu      sync.Mutex
	records []audit.AuditEntry
}

func (l *capturedAuditLogger) LogAction(operator string, action audit.AuditAction, resourceID string, before, after interface{}, details string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, audit.AuditEntry{Operator: operator, Action: action, ResourceID: resourceID, Before: before, After: after, Details: details})
	return nil
}

func (l *capturedAuditLogger) LogActionSimple(operator string, action audit.AuditAction, resourceID, details string) error {
	return l.LogAction(operator, action, resourceID, nil, nil, details)
}

func TestProjectMutationAuditMiddlewareRecordsOutcomeWithoutBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := &capturedAuditLogger{}
	router := gin.New()
	router.Use(projectMutationAuditMiddleware(logger))
	router.Use(func(c *gin.Context) { c.Set("user", "alice"); c.Next() })
	router.POST("/api/v1/projects/:id/tasks/:task_id/review", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p/tasks/t/review", strings.NewReader(`{"token":"must-not-be-logged"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if len(logger.records) != 1 {
		t.Fatalf("records=%d", len(logger.records))
	}
	record := logger.records[0]
	if record.Action != audit.ActionProjectMutation || record.Operator != "alice" || record.ResourceID != "p/t" {
		t.Fatalf("unexpected record: %+v", record)
	}
	if strings.Contains(record.Details, "token") || strings.Contains(record.Details, "must-not-be-logged") || record.Before != nil || record.After != nil {
		t.Fatalf("audit leaked body: %+v", record)
	}
}

func TestProjectMutationAuditMiddlewareRecordsDeniedAndIgnoresReads(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := &capturedAuditLogger{}
	router := gin.New()
	router.Use(projectMutationAuditMiddleware(logger))
	router.POST("/api/v1/projects/:id/publish", func(c *gin.Context) { c.Status(http.StatusForbidden) })
	router.GET("/api/v1/projects/:id", func(c *gin.Context) { c.Status(http.StatusOK) })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/projects/p/publish", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/projects/p", nil))
	if len(logger.records) != 1 || logger.records[0].Action != audit.ActionProjectMutationDeny || logger.records[0].Operator != "anonymous" {
		t.Fatalf("unexpected records: %+v", logger.records)
	}
}

func TestProjectMutationAuditMiddlewareCoversServiceSideEffects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := &capturedAuditLogger{}
	router := gin.New()
	router.Use(projectMutationAuditMiddleware(logger))
	router.Use(func(c *gin.Context) { c.Set("user", "admin"); c.Next() })
	router.POST("/api/v1/services/whisper/models/download", func(c *gin.Context) { c.Status(http.StatusBadGateway) })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/services/whisper/models/download", nil))
	if len(logger.records) != 1 || logger.records[0].Action != audit.ActionServiceMutation || logger.records[0].Operator != "admin" {
		t.Fatalf("unexpected records: %+v", logger.records)
	}
	if logger.records[0].ResourceID != "/api/v1/services/whisper/models/download" {
		t.Fatalf("unexpected resource: %+v", logger.records[0])
	}
}
