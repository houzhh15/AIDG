package api

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterUnifiedDocRoutesCoexistsWithDocumentTagRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	apiV1 := router.Group("/api/v1")
	apiV1.GET("/projects/:id/tasks/:task_id/docs/:docType/tags", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	require.NotPanics(t, func() {
		RegisterUnifiedDocRoutes(apiV1, nil)
	})

	found := false
	for _, route := range router.Routes() {
		if route.Method == http.MethodGet &&
			route.Path == "/api/v1/projects/:id/tasks/:task_id/docs/:docType/export" {
			found = true
			break
		}
	}
	require.True(t, found, "unified task document export route was not registered")
}
