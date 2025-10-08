# meetings-service 后端 API 实现指南

## 需要添加的 API：设备检测端点

### API 路由
```
GET /api/v1/devices/avfoundation
```

### 权限要求
- 需要认证（Bearer Token）
- 权限范围：`meeting.read`（任何可以查看会议的用户都能获取设备列表）

### 实现步骤

#### 1. 创建设备API处理器文件

创建文件：`cmd/server/internal/api/devices.go`

```go
package api

import (
	"net/http"
	"os/exec"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// HandleGetAVFoundationDevices 获取 macOS AVFoundation 设备列表
// list ffmpeg (avfoundation) devices (audio/video)
func HandleGetAVFoundationDevices() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 执行 ffmpeg 命令获取 AVFoundation 设备列表
		cmd := exec.Command("ffmpeg", "-f", "avfoundation", "-list_devices", "true", "-i", "")
		
		// ffmpeg 将设备列表输出到 stderr，返回非零退出码，忽略错误
		out, _ := cmd.CombinedOutput()
		lines := strings.Split(string(out), "\n")
		
		section := "" // 当前节：audio 或 video
		// 正则匹配设备行，格式：[0] MacBook Pro 麦克风
		reDev := regexp.MustCompile(`\[(\d+)\]\s+(.+)`)
		devices := []gin.H{}
		
		for _, ln := range lines {
			ltrim := strings.TrimSpace(ln)
			
			// 识别视频设备节
			if strings.Contains(ltrim, "AVFoundation video devices") {
				section = "video"
				continue
			}
			
			// 识别音频设备节
			if strings.Contains(ltrim, "AVFoundation audio devices") {
				section = "audio"
				continue
			}
			
			// 匹配设备行
			m := reDev.FindStringSubmatch(ltrim)
			if m == nil {
				// 尝试在未裁剪的原始行中匹配（处理带颜色/日志前缀的情况）
				m = reDev.FindStringSubmatch(ln)
			}
			
			if m != nil && (section == "audio" || section == "video") {
				devices = append(devices, gin.H{
					"index": m[1],    // 设备索引
					"name":  m[2],    // 设备名称
					"kind":  section, // 设备类型：audio 或 video
				})
			}
		}
		
		c.JSON(http.StatusOK, gin.H{"devices": devices})
	}
}
```

#### 2. 在 main.go 中注册路由

在 `cmd/server/main.go` 的路由注册部分添加：

```go
// ========== Devices API ==========
r.GET("/api/v1/devices/avfoundation", api.HandleGetAVFoundationDevices())
```

**位置建议**：在会议（Meetings）API 注册之后，项目（Projects）API 注册之前。

#### 3. 配置权限规则

在 `main.go` 的权限映射表中添加（如果有的话）：

```go
routeScopes := map[string][]string{
    // ... 其他路由权限
    "GET /api/v1/devices/avfoundation": {"meeting.read"},
    // ... 其他路由权限
}
```

### API 响应格式

**成功响应** (HTTP 200):
```json
{
  "devices": [
    {
      "index": "0",
      "name": "MacBook Pro 麦克风",
      "kind": "audio"
    },
    {
      "index": "1",
      "name": "BlackHole 2ch",
      "kind": "audio"
    },
    {
      "index": "0",
      "name": "FaceTime HD Camera",
      "kind": "video"
    }
  ]
}
```

### 工作原理

1. **执行 FFmpeg 命令**：
   ```bash
   ffmpeg -f avfoundation -list_devices true -i ""
   ```

2. **解析输出**：
   - FFmpeg 将设备列表输出到 stderr
   - 输出格式示例：
     ```
     [AVFoundation input device @ 0x...] AVFoundation video devices:
     [AVFoundation input device @ 0x...] [0] FaceTime HD Camera
     [AVFoundation input device @ 0x...] AVFoundation audio devices:
     [AVFoundation input device @ 0x...] [0] MacBook Pro Microphone
     [AVFoundation input device @ 0x...] [1] BlackHole 2ch
     ```

3. **提取设备信息**：
   - 使用正则表达式 `\[(\d+)\]\s+(.+)` 匹配设备行
   - 根据 "AVFoundation video/audio devices" 标记判断设备类型
   - 构造包含 index、name、kind 的设备对象

### 注意事项

1. **平台依赖**：
   - 此功能仅在 macOS 上可用
   - 需要系统安装 FFmpeg
   - 在其他平台（Linux/Windows）上执行会失败，可以返回空数组

2. **错误处理**：
   - 当前实现忽略 FFmpeg 的退出错误（正常行为，因为没有提供输入）
   - 如果 FFmpeg 未安装，`exec.Command` 会失败，但不会导致服务崩溃

3. **性能考虑**：
   - FFmpeg 命令执行较快（通常 < 100ms）
   - 前端会在每次打开编辑窗口时调用此API
   - 可以考虑添加简单的缓存（TTL: 60秒）

### 测试方法

1. **直接测试命令**：
   ```bash
   ffmpeg -f avfoundation -list_devices true -i "" 2>&1 | grep "\[.*\]"
   ```

2. **curl 测试**：
   ```bash
   curl -H "Authorization: Bearer YOUR_TOKEN" \
        http://localhost:8080/api/v1/devices/avfoundation
   ```

3. **前端测试**：
   - 启动后端服务
   - 重新构建前端（已完成）
   - 打开会议编辑窗口
   - 查看"检测到的音频设备"下拉框是否显示设备列表

### 完整的改进建议（可选）

如果要增强功能，可以添加：

1. **平台检测**：
   ```go
   import "runtime"
   
   if runtime.GOOS != "darwin" {
       c.JSON(http.StatusOK, gin.H{"devices": []gin.H{}})
       return
   }
   ```

2. **错误处理**：
   ```go
   cmd := exec.Command("ffmpeg", "-f", "avfoundation", "-list_devices", "true", "-i", "")
   out, err := cmd.CombinedOutput()
   if err != nil && !strings.Contains(string(out), "AVFoundation") {
       c.JSON(http.StatusInternalServerError, gin.H{
           "error": "Failed to list devices",
           "devices": []gin.H{},
       })
       return
   }
   ```

3. **结果缓存**：
   ```go
   var deviceCache struct {
       devices   []gin.H
       timestamp time.Time
       mu        sync.RWMutex
   }
   
   // 缓存60秒
   if time.Since(deviceCache.timestamp) < 60*time.Second {
       deviceCache.mu.RLock()
       defer deviceCache.mu.RUnlock()
       c.JSON(http.StatusOK, gin.H{"devices": deviceCache.devices})
       return
   }
   ```

## 总结

添加此API后：
- ✅ 前端可以自动检测系统音频/视频设备
- ✅ 用户可以从下拉框快速选择设备
- ✅ 避免手动输入设备名称的错误
- ✅ 提升用户体验

**预计工作量**：约15-30分钟完成实现和测试。
