package api

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/taskdocs"
)

// HandleAppendTaskDoc POST /api/v1/projects/:id/tasks/:task_id/{docType}/append
// 追加文档内容
func HandleAppendTaskDoc(svc *taskdocs.DocService, docType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		taskID := c.Param("task_id")

		var req struct {
			Content         string `json:"content"`
			ExpectedVersion *int   `json:"expected_version"`
			Op              string `json:"op"`
			Source          string `json:"source"`
		}

		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Content) == "" {
			badRequestResponse(c, "invalid body")
			return
		}

		userVal, _ := c.Get("user")
		username, _ := userVal.(string)

		meta, chunk, duplicate, err := svc.Append(projectID, taskID, docType, req.Content, username, req.ExpectedVersion, req.Op, req.Source)
		if err != nil {
			if err.Error() == "version_mismatch" {
				c.JSON(http.StatusConflict, gin.H{"error": "version_mismatch"})
				return
			}
			internalErrorResponse(c, err)
			return
		}

		// enriched response meta fields
		resp := gin.H{
			"version":       meta.Version,
			"duplicate":     duplicate,
			"etag":          meta.ETag,
			"last_sequence": meta.LastSequence,
			"chunk_count":   meta.ChunkCount,
			"deleted_count": meta.DeletedCount,
		}

		if compiledPath, err2 := taskdocs.DocCompiledPath(projectID, taskID, docType); err2 == nil {
			if fi, statErr := os.Stat(compiledPath); statErr == nil {
				resp["compiled_size"] = fi.Size()
			}
		}

		if !duplicate && chunk != nil {
			resp["sequence"] = chunk.Sequence
			resp["timestamp"] = chunk.Timestamp
		}

		c.JSON(http.StatusOK, resp)
	}
}

// HandleListTaskDocChunks GET /api/v1/projects/:id/tasks/:task_id/{docType}/chunks
// 列出文档块
func HandleListTaskDocChunks(svc *taskdocs.DocService, docType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		taskID := c.Param("task_id")

		chunks, meta, err := svc.List(projectID, taskID, docType)
		if err != nil {
			internalErrorResponse(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"chunks": chunks, "meta": meta})
	}
}

// HandleDeleteTaskDocChunk DELETE /api/v1/projects/:id/tasks/:task_id/{docType}/chunks/:seq
// 删除文档块
func HandleDeleteTaskDocChunk(svc *taskdocs.DocService, docType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		taskID := c.Param("task_id")
		seqStr := c.Param("seq")
		seq, _ := strconv.Atoi(seqStr)

		meta, err := svc.Delete(projectID, taskID, docType, seq)
		if err != nil {
			internalErrorResponse(c, err)
			return
		}

		resp := gin.H{
			"version":          meta.Version,
			"deleted_sequence": seq,
			"etag":             meta.ETag,
			"last_sequence":    meta.LastSequence,
			"chunk_count":      meta.ChunkCount,
			"deleted_count":    meta.DeletedCount,
		}

		if compiledPath, err2 := taskdocs.DocCompiledPath(projectID, taskID, docType); err2 == nil {
			if fi, statErr := os.Stat(compiledPath); statErr == nil {
				resp["compiled_size"] = fi.Size()
			}
		}

		c.JSON(http.StatusOK, resp)
	}
}

// HandleToggleTaskDocChunk PATCH /api/v1/projects/:id/tasks/:task_id/{docType}/chunks/:seq/toggle
// 切换文档块状态
func HandleToggleTaskDocChunk(svc *taskdocs.DocService, docType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		taskID := c.Param("task_id")
		seqStr := c.Param("seq")
		seq, _ := strconv.Atoi(seqStr)

		meta, err := svc.Toggle(projectID, taskID, docType, seq)
		if err != nil {
			if err.Error() == "version_mismatch" {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			internalErrorResponse(c, err)
			return
		}

		resp := gin.H{
			"version":       meta.Version,
			"etag":          meta.ETag,
			"last_sequence": meta.LastSequence,
			"chunk_count":   meta.ChunkCount,
			"deleted_count": meta.DeletedCount,
		}

		c.JSON(http.StatusOK, resp)
	}
}

// HandleSquashTaskDoc POST /api/v1/projects/:id/tasks/:task_id/{docType}/squash
// 压缩文档块
func HandleSquashTaskDoc(svc *taskdocs.DocService, docType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		taskID := c.Param("task_id")

		var body struct {
			ExpectedVersion *int `json:"expected_version"`
		}
		_ = c.ShouldBindJSON(&body)

		userVal, _ := c.Get("user")
		username, _ := userVal.(string)

		meta, err := svc.Squash(projectID, taskID, docType, username, body.ExpectedVersion)
		if err != nil {
			if err.Error() == "version_mismatch" {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			internalErrorResponse(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"version":       meta.Version,
			"etag":          meta.ETag,
			"last_sequence": meta.LastSequence,
			"chunk_count":   meta.ChunkCount,
		})
	}
}

// HandleExportTaskDoc GET /api/v1/projects/:id/tasks/:task_id/{docType}/export
// 导出文档内容
func HandleExportTaskDoc(svc *taskdocs.DocService, docType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		taskID := c.Param("task_id")

		compiledPath, err := taskdocs.DocCompiledPath(projectID, taskID, docType)
		if err != nil {
			internalErrorResponse(c, err)
			return
		}

		b, _ := os.ReadFile(compiledPath)
		meta, _ := taskdocs.LoadOrInitMeta(projectID, taskID, docType)

		c.JSON(http.StatusOK, gin.H{
			"content": string(b),
			"version": meta.Version,
			"etag":    meta.ETag,
		})
	}
}
