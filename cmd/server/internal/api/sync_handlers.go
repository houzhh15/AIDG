package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/domain/sync"
)

// SyncConfig 封装 sync 相关配置
type SyncConfig struct {
	SharedSecret string // HMAC 共享密钥
}

// NewSyncConfig 从环境变量创建配置
func NewSyncConfig() *SyncConfig {
	secret := os.Getenv("SYNC_SHARED_SECRET")
	if secret == "" {
		secret = "neteye@123" // 默认值
	}
	return &SyncConfig{SharedSecret: secret}
}

// summarizeFiles 生成文件摘要 (path:hash 按行排序)
func summarizeFiles(files []sync.SyncFile) string {
	lines := make([]string, 0, len(files))
	for _, f := range files {
		lines = append(lines, f.Path+":"+f.Hash)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// makeSignature 生成 HMAC-SHA256 签名
func (cfg *SyncConfig) makeSignature(payload, target string) string {
	mac := hmac.New(sha256.New, []byte(cfg.SharedSecret))
	mac.Write([]byte(payload + "|" + target))
	return hex.EncodeToString(mac.Sum(nil))
}

// dispatchRequest 调度请求结构
type dispatchRequest struct {
	Target      string         `json:"target"`
	Mode        sync.SyncMode  `json:"mode"`
	Signature   string         `json:"signature"`
	Options     map[string]any `json:"options"`
	ReturnFiles bool           `json:"return_files"`
}

// receiveEnvelope 接收信封结构
type receiveEnvelope struct {
	Mode       sync.SyncMode   `json:"mode"`
	Files      []sync.SyncFile `json:"files"`
	Signature  string          `json:"signature"`
	Source     string          `json:"source"`
	TargetHost string          `json:"target_host"`
}

// HandleSyncPrepare GET /api/v1/sync/prepare
func HandleSyncPrepare(c *gin.Context) {
	files, err := sync.CollectAllowedFiles(sync.DefaultSyncAllowList)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"files": files})
}

// HandleSync POST /api/v1/sync
func HandleSync(c *gin.Context) {
	var req sync.SyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "bad_request"})
		return
	}

	// 验证模式
	if req.Mode != sync.ModeClientOverwrite &&
		req.Mode != sync.ModeServerOverwrite &&
		req.Mode != sync.ModeMergeNoOverwrite &&
		req.Mode != sync.ModePullOverwrite {
		c.JSON(400, gin.H{"error": "invalid_mode"})
		return
	}

	applied := []sync.SyncApplied{}
	conflicts := []sync.SyncApplied{}

	// 处理客户端上传文件
	if req.Mode == sync.ModeClientOverwrite || req.Mode == sync.ModeMergeNoOverwrite {
		for _, f := range req.Files {
			if !sync.IsAllowedSyncPath(f.Path, sync.DefaultSyncAllowList) {
				conflicts = append(conflicts, sync.SyncApplied{Path: f.Path, Action: "forbidden"})
				continue
			}

			if req.Mode == sync.ModeClientOverwrite {
				if err := sync.WriteSyncFile(f, sync.DefaultSyncAllowList); err != nil {
					conflicts = append(conflicts, sync.SyncApplied{Path: f.Path, Action: "error"})
				} else {
					applied = append(applied, sync.SyncApplied{Path: f.Path, Action: "write"})
				}
			} else { // merge_no_overwrite
				if _, err := os.Stat(f.Path); err == nil {
					applied = append(applied, sync.SyncApplied{Path: f.Path, Action: "skip"})
				} else {
					if err := sync.WriteSyncFile(f, sync.DefaultSyncAllowList); err != nil {
						conflicts = append(conflicts, sync.SyncApplied{Path: f.Path, Action: "error"})
					} else {
						applied = append(applied, sync.SyncApplied{Path: f.Path, Action: "write"})
					}
				}
			}
		}
	}

	// 返回服务端文件
	var serverFiles []sync.SyncFile
	if req.Mode == sync.ModeServerOverwrite || (req.Options != nil && req.Options["return_server_files"] == true) {
		files, err := sync.CollectAllowedFiles(sync.DefaultSyncAllowList)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		serverFiles = files
	}

	// 统计
	writes, skips := 0, 0
	for _, a := range applied {
		if a.Action == "write" {
			writes++
		} else if a.Action == "skip" {
			skips++
		}
	}

	resp := sync.SyncResponse{
		Mode:        req.Mode,
		Applied:     applied,
		Conflicts:   conflicts,
		ServerFiles: serverFiles,
		Summary:     gin.H{"writes": writes, "skips": skips, "conflicts": len(conflicts)},
	}
	c.JSON(200, resp)
}

// HandleSyncDispatch POST /api/v1/sync/dispatch
func HandleSyncDispatch(cfg *SyncConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dispatchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad_request"})
			return
		}

		if req.Target == "" {
			c.JSON(400, gin.H{"error": "target_required"})
			return
		}

		// 验证模式
		if req.Mode != sync.ModeClientOverwrite &&
			req.Mode != sync.ModeServerOverwrite &&
			req.Mode != sync.ModeMergeNoOverwrite &&
			req.Mode != sync.ModePullOverwrite {
			c.JSON(400, gin.H{"error": "invalid_mode"})
			return
		}

		// 收集本地文件
		files, err := sync.CollectAllowedFiles(sync.DefaultSyncAllowList)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		payloadSummary := summarizeFiles(files)

		// 解析目标以获取 host:port
		parsed, perr := url.Parse(strings.TrimSpace(req.Target))
		if perr != nil || parsed.Host == "" {
			c.JSON(400, gin.H{"error": "invalid_target"})
			return
		}
		targetHost := parsed.Host

		// 生成签名
		sig := cfg.makeSignature(payloadSummary, targetHost)

		env := receiveEnvelope{
			Mode:       req.Mode,
			Files:      files,
			Signature:  sig,
			Source:     c.Request.Host,
			TargetHost: targetHost,
		}

		// 发送 HTTP POST
		endpoint := strings.TrimSuffix(req.Target, "/") + "/api/v1/sync/receive"
		bodyBytes, _ := json.Marshal(env)
		httpReq, _ := http.NewRequest("POST", endpoint, strings.NewReader(string(bodyBytes)))
		httpReq.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			c.JSON(502, gin.H{"error": "dispatch_failed", "detail": err.Error()})
			return
		}
		defer resp.Body.Close()

		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			c.JSON(resp.StatusCode, gin.H{"error": "remote_error", "body": string(b)})
			return
		}

		var remoteResp map[string]any
		_ = json.Unmarshal(b, &remoteResp)

		out := gin.H{
			"dispatched_files": len(files),
			"remote_response":  remoteResp,
		}

		// 处理 pull_overwrite 模式
		if req.Mode == sync.ModePullOverwrite {
			if serverFilesRaw, ok := remoteResp["server_files"].([]any); ok {
				appliedCount := 0
				for _, item := range serverFilesRaw {
					m, ok2 := item.(map[string]any)
					if !ok2 {
						continue
					}
					pathStr, _ := m["path"].(string)
					contentStr, _ := m["content"].(string)
					hashStr, _ := m["hash"].(string)
					var sizeInt int64
					if sz, okSz := m["size"].(float64); okSz {
						sizeInt = int64(sz)
					}
					if pathStr == "" {
						continue
					}
					if err := sync.WriteSyncFile(sync.SyncFile{
						Path:    pathStr,
						Content: contentStr,
						Hash:    hashStr,
						Size:    sizeInt,
					}, sync.DefaultSyncAllowList); err == nil {
						appliedCount++
					}
				}
				out["pull_applied"] = appliedCount
			}
		}

		if req.ReturnFiles {
			out["files"] = files
		}

		c.JSON(200, out)
	}
}

// HandleSyncReceive POST /api/v1/sync/receive
func HandleSyncReceive(cfg *SyncConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		var env receiveEnvelope
		if err := c.ShouldBindJSON(&env); err != nil {
			c.JSON(400, gin.H{"error": "bad_request"})
			return
		}

		if env.TargetHost == "" {
			env.TargetHost = c.Request.Host
		}

		// 验证签名
		calcSummary := summarizeFiles(env.Files)
		expected := cfg.makeSignature(calcSummary, env.TargetHost)
		if !hmac.Equal([]byte(expected), []byte(env.Signature)) {
			c.JSON(401, gin.H{"error": "bad_signature", "expected_target_host": env.TargetHost})
			return
		}

		// 应用文件
		applied := []sync.SyncApplied{}
		conflicts := []sync.SyncApplied{}
		mode := env.Mode

		if mode == sync.ModeClientOverwrite || mode == sync.ModeMergeNoOverwrite {
			for _, f := range env.Files {
				if !sync.IsAllowedSyncPath(f.Path, sync.DefaultSyncAllowList) {
					conflicts = append(conflicts, sync.SyncApplied{Path: f.Path, Action: "forbidden"})
					continue
				}

				if mode == sync.ModeClientOverwrite {
					if err := sync.WriteSyncFile(f, sync.DefaultSyncAllowList); err != nil {
						conflicts = append(conflicts, sync.SyncApplied{Path: f.Path, Action: "error"})
					} else {
						applied = append(applied, sync.SyncApplied{Path: f.Path, Action: "write"})
					}
				} else { // merge_no_overwrite
					if _, err := os.Stat(f.Path); err == nil {
						applied = append(applied, sync.SyncApplied{Path: f.Path, Action: "skip"})
					} else {
						if err := sync.WriteSyncFile(f, sync.DefaultSyncAllowList); err != nil {
							conflicts = append(conflicts, sync.SyncApplied{Path: f.Path, Action: "error"})
						} else {
							applied = append(applied, sync.SyncApplied{Path: f.Path, Action: "write"})
						}
					}
				}
			}
		}

		// 返回服务端文件
		var serverFiles []sync.SyncFile
		if mode == sync.ModeServerOverwrite || mode == sync.ModePullOverwrite {
			var err error
			serverFiles, err = sync.CollectAllowedFiles(sync.DefaultSyncAllowList)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
		}

		// 统计
		writes, skips := 0, 0
		for _, a := range applied {
			if a.Action == "write" {
				writes++
			} else if a.Action == "skip" {
				skips++
			}
		}

		c.JSON(200, gin.H{
			"mode":         mode,
			"applied":      applied,
			"conflicts":    conflicts,
			"server_files": serverFiles,
			"summary":      gin.H{"writes": writes, "skips": skips, "conflicts": len(conflicts)},
		})
	}
}

// HandleSVNSync POST /api/v1/svn/sync
// 注意: runSVNSync 函数在 main.go 中未找到实现，可能需要补充
func HandleSVNSync(c *gin.Context) {
	// 基本安全检查: 确保用户上下文存在
	if _, ok := c.Get("user"); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "unauthorized"})
		return
	}

	// TODO: 实现 SVN 同步逻辑
	// 原代码中调用了 runSVNSync(ctx)，但该函数未在 main.go 中定义
	// 需要进一步确认该功能的实现位置或补充实现

	c.JSON(http.StatusNotImplemented, gin.H{
		"success": false,
		"message": "SVN sync not yet implemented - requires runSVNSync function",
	})
}
