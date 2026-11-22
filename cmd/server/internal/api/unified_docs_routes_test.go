package api

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/docslot"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/taskdocs"
)

// TestUnifiedTaskDocRoutesCoexistWithTagRoutes guards against Gin wildcard
// name conflicts at the shared /docs/:docType node.
func TestUnifiedTaskDocRoutesCoexistWithTagRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/projects/:id/tasks/:task_id/docs/:docType/tags", func(c *gin.Context) {})

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("route registration panicked: %v", recovered)
		}
	}()
	RegisterUnifiedDocRoutes(router.Group("/api/v1"), routeRegistrationDocService{})

	registered := map[string]bool{}
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	for _, expected := range []string{
		"GET /api/v1/projects/:id/tasks/:task_id/docs/:docType/tags",
		"GET /api/v1/projects/:id/tasks/:task_id/docs/:docType/export",
		"POST /api/v1/projects/:id/tasks/:task_id/docs/:docType/append",
	} {
		if !registered[expected] {
			t.Fatalf("route not registered: %s", expected)
		}
	}
}

type routeRegistrationDocService struct{}

func (routeRegistrationDocService) Append(docslot.DocumentScope, string, string, string, string, *int, string, string) (*docslot.AppendResult, error) {
	return nil, nil
}
func (routeRegistrationDocService) Export(docslot.DocumentScope, string, string) (*docslot.ExportResult, error) {
	return nil, nil
}
func (routeRegistrationDocService) ListChunks(docslot.DocumentScope, string, string) ([]taskdocs.DocChunk, *taskdocs.DocMeta, error) {
	return nil, nil, nil
}
func (routeRegistrationDocService) Squash(docslot.DocumentScope, string, string, string, string) (*taskdocs.DocMeta, error) {
	return nil, nil
}
func (routeRegistrationDocService) AppendTask(string, string, string, string, string, *int, string, string) (*docslot.AppendResult, error) {
	return nil, nil
}
func (routeRegistrationDocService) ExportTask(string, string, string) (*docslot.ExportResult, error) {
	return nil, nil
}
func (routeRegistrationDocService) ListTaskChunks(string, string, string) ([]taskdocs.DocChunk, *taskdocs.DocMeta, error) {
	return nil, nil, nil
}
func (routeRegistrationDocService) SquashTask(string, string, string, string, string) (*taskdocs.DocMeta, error) {
	return nil, nil
}
func (routeRegistrationDocService) GetSections(docslot.DocumentScope, string, string) (*taskdocs.SectionMeta, error) {
	return nil, nil
}
func (routeRegistrationDocService) GetSection(docslot.DocumentScope, string, string, string, bool) (*taskdocs.SectionContent, error) {
	return nil, nil
}
func (routeRegistrationDocService) UpdateSection(docslot.DocumentScope, string, string, string, string, int) error {
	return nil
}
func (routeRegistrationDocService) InsertSection(docslot.DocumentScope, string, string, string, string, *string, int) (*taskdocs.Section, error) {
	return nil, nil
}
func (routeRegistrationDocService) DeleteSection(docslot.DocumentScope, string, string, string, bool, int) error {
	return nil
}
func (routeRegistrationDocService) GetTaskSections(string, string, string) (*taskdocs.SectionMeta, error) {
	return nil, nil
}
func (routeRegistrationDocService) GetTaskSection(string, string, string, string, bool) (*taskdocs.SectionContent, error) {
	return nil, nil
}
func (routeRegistrationDocService) UpdateTaskSection(string, string, string, string, string, int) error {
	return nil
}
func (routeRegistrationDocService) InsertTaskSection(string, string, string, string, string, *string, int) (*taskdocs.Section, error) {
	return nil, nil
}
func (routeRegistrationDocService) DeleteTaskSection(string, string, string, string, bool, int) error {
	return nil
}
func (routeRegistrationDocService) PathResolver() docslot.PathResolver {
	return nil
}
