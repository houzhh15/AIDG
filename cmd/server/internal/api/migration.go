package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/tasklifecycle"
	"github.com/houzhh15/AIDG/cmd/server/internal/migration"
)

const maxMigrationRequestBytes = 2 << 20

func migrationService(reg *projects.ProjectRegistry) *migration.Service {
	return migration.NewService(projectsRoot(), reg)
}

func decodeStrictMigrationJSON(c *gin.Context, target any) error {
	reader := http.MaxBytesReader(c.Writer, c.Request.Body, maxMigrationRequestBytes)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request must contain one JSON object")
	}
	return nil
}

func HandlePreviewProjectMigration(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct{}
		if err := decodeStrictMigrationJSON(c, &request); err != nil && !errors.Is(err, io.EOF) {
			badRequestResponse(c, "invalid migration request")
			return
		}
		plan, err := migrationService(reg).AnalyzeProject(c.Param("id"))
		if err != nil {
			handleMigrationError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"plan": plan})
	}
}

func HandlePreviewTaskMigration(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct{}
		if err := decodeStrictMigrationJSON(c, &request); err != nil && !errors.Is(err, io.EOF) {
			badRequestResponse(c, "invalid migration request")
			return
		}
		plan, err := migrationService(reg).AnalyzeTask(c.Param("id"), c.Param("task_id"))
		if err != nil {
			handleMigrationError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"plan": plan})
	}
}

func HandleExecuteProjectMigration(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return handleExecuteMigration(reg, false)
}

func HandleExecuteTaskMigration(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return handleExecuteMigration(reg, true)
}

func handleExecuteMigration(reg *projects.ProjectRegistry, taskScope bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Plan migration.Plan `json:"plan"`
		}
		if err := decodeStrictMigrationJSON(c, &request); err != nil {
			badRequestResponse(c, "invalid migration request")
			return
		}
		if request.Plan.ProjectID != c.Param("id") || (taskScope && request.Plan.TaskID != c.Param("task_id")) || (!taskScope && request.Plan.TaskID != "") {
			badRequestResponse(c, "migration plan scope must match route")
			return
		}
		result, err := migrationService(reg).Execute(request.Plan, c.GetString("user"))
		if err != nil {
			if result.PlanDigest != "" {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "migration_failed", "result": result})
				return
			}
			handleMigrationError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"result": result})
	}
}

func handleMigrationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, tasklifecycle.ErrNotFound):
		notFoundResponse(c, "migration resource")
	case errors.Is(err, migration.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "migration_conflict"})
	case errors.Is(err, migration.ErrBlocked):
		c.JSON(http.StatusConflict, gin.H{"error": "migration_blocked"})
	case errors.Is(err, migration.ErrInvalidPlan), errors.Is(err, migration.ErrInvalidScope):
		badRequestResponse(c, "migration rejected")
	default:
		internalErrorResponse(c, err)
	}
}
