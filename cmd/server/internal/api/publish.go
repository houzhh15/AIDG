package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/tasklifecycle"
	"github.com/houzhh15/AIDG/cmd/server/internal/publish"
)

func HandlePublish(reg *projects.ProjectRegistry) gin.HandlerFunc {
	service := publish.NewService(projectsRoot(), reg)
	return func(c *gin.Context) {
		manifest, err := publish.DecodeManifest(http.MaxBytesReader(c.Writer, c.Request.Body, 16<<20))
		if err != nil {
			badRequestResponse(c, "invalid publish manifest")
			return
		}
		if manifest.ProjectID != c.Param("id") {
			badRequestResponse(c, "manifest project_id must match route")
			return
		}
		receipt, duplicate, err := service.Publish(manifest, c.GetString("user"))
		if err != nil {
			switch {
			case errors.Is(err, tasklifecycle.ErrVersionConflict):
				c.JSON(http.StatusConflict, gin.H{"error": "publish_conflict"})
			case errors.Is(err, tasklifecycle.ErrNotFound):
				notFoundResponse(c, "publish resource not found")
			default:
				badRequestResponse(c, "publish rejected")
			}
			return
		}
		c.JSON(http.StatusOK, gin.H{"receipt": receipt, "duplicate": duplicate})
	}
}
