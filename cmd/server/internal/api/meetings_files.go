package api

// meetings_files.go - Meeting task and chunk file operations
// Handles: ListChunks, ListTaskFiles, GetTaskFile, GetChunkFile, UpdateChunkSegments

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/meetings"
	orchestrator "github.com/houzhh15/AIDG/cmd/server/internal/orchestrator"
)

// ============================================================================
// File Listing Handlers
// ============================================================================

// HandleListChunks GET /api/v1/tasks/:id/chunks
// 列出任务的所有 chunks 及其文件状态
func HandleListChunks(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		entries, err := os.ReadDir(t.Cfg.OutputDir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		re := regexp.MustCompile(`^chunk_([0-9]{4})`)
		type flag struct {
			ID                                                                string `json:"id"`
			Wav, Segments, Speakers, Embeddings, Mapped, GlobalMapped, Merged bool   `json:"-"`
		}
		mp := map[string]*flag{}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			m := re.FindStringSubmatch(name)
			if m == nil {
				continue
			}
			idc := m[1]
			if mp[idc] == nil {
				mp[idc] = &flag{ID: idc}
			}
			if strings.HasSuffix(name, ".wav") {
				mp[idc].Wav = true
			}
			if strings.HasSuffix(name, "_segments.json") {
				mp[idc].Segments = true
			}
			if strings.HasSuffix(name, "_speakers.json") {
				mp[idc].Speakers = true
			}
			if strings.HasSuffix(name, "_embeddings.json") {
				mp[idc].Embeddings = true
			}
			if strings.HasSuffix(name, "_speakers_mapped.json") {
				mp[idc].Mapped = true
			}
			if strings.HasSuffix(name, "_speakers_mapped_global.json") {
				mp[idc].GlobalMapped = true
			}
			if strings.HasSuffix(name, "_merged.txt") {
				mp[idc].Merged = true
			}
		}
		// produce list sorted
		keys := []string{}
		for k := range mp {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := []gin.H{}
		for _, k := range keys {
			f := mp[k]
			out = append(out, gin.H{"id": f.ID, "wav": f.Wav, "segments": f.Segments, "speakers": f.Speakers, "embeddings": f.Embeddings, "mapped": f.Mapped || f.GlobalMapped, "merged": f.Merged})
		}
		c.JSON(http.StatusOK, gin.H{"chunks": out})
	}
}

// HandleListTaskFiles GET /api/v1/tasks/:id/files
// 列出任务输出目录的所有文件
func HandleListTaskFiles(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		entries, err := os.ReadDir(t.Cfg.OutputDir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		files := []gin.H{}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, _ := e.Info()
			files = append(files, gin.H{"name": e.Name(), "size": info.Size(), "mod_time": info.ModTime()})
		}
		c.JSON(http.StatusOK, gin.H{"files": files})
	}
}

// ============================================================================
// File Access Handlers
// ============================================================================

// HandleGetTaskFile GET /api/v1/tasks/:id/files/:filename
// 读取任务输出目录中的指定文件
func HandleGetTaskFile(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		filename := c.Param("filename")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}

		// 安全检查：只允许读取输出目录内的文件，防止路径遍历攻击
		if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filename"})
			return
		}

		// 定义 prompt 文件列表
		promptFiles := map[string]bool{
			"topic.txt":            true,
			"meeting_polish.txt":   true,
			"feature_list.txt":     true,
			"architecture_new.txt": true,
		}

		var filePath string
		meetingsRoot := meetings.TasksRoot()
		if promptFiles[filename] {
			// prompt 文件从 tasks/ 目录读取
			filePath = filepath.Join(meetingsRoot, filename)
		} else {
			// 其他文件从任务的输出目录读取
			filePath = filepath.Join(t.Cfg.OutputDir, filename)
		}

		// 检查文件是否存在
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}

		// 读取文件内容
		content, err := os.ReadFile(filePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
			return
		}

		// 返回纯文本内容
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusOK, string(content))
	}
}

// HandleGetChunkFile GET /api/v1/tasks/:id/chunks/:cid/:kind
// 获取指定 chunk 的文件内容
func HandleGetChunkFile(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cid := c.Param("cid")
		kind := c.Param("kind")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		full, fname, err := resolveChunkFile(t.Cfg.OutputDir, cid, kind)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if _, err := os.Stat(full); err != nil {
			// 如果任务仍在运行或处理中, 返回 202 表示还未生成
			if t.State == orchestrator.StateRunning || t.State == orchestrator.StateStopping || t.State == orchestrator.StateDraining {
				c.JSON(http.StatusAccepted, gin.H{"status": "pending", "expected": fname})
				return
			}
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found", "expected": fname})
			return
		}
		if strings.HasSuffix(full, ".json") {
			c.Header("Content-Type", "application/json")
		}
		c.File(full)
	}
}

// HandleUpdateChunkSegments PUT /api/v1/tasks/:id/chunks/:cid/segments
// 更新指定 chunk 的 segments JSON 文件
func HandleUpdateChunkSegments(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cid := c.Param("cid")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		full, _, err := resolveChunkFile(t.Cfg.OutputDir, cid, "segments")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var js any
		if err := c.ShouldBindJSON(&js); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		b, err := json.MarshalIndent(js, "", "  ")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := os.WriteFile(full, b, 0o644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"updated": filepath.Base(full)})
	}
}
