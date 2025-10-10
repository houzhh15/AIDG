package api

import (
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// HandleGetAVFoundationDevices 获取 macOS AVFoundation 设备列表
// list ffmpeg (avfoundation) devices (audio/video)
func HandleGetAVFoundationDevices() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否在 Docker 容器中运行
		if isRunningInDocker() {
			c.JSON(http.StatusOK, gin.H{
				"devices": []gin.H{},
				"warning": "Docker 容器无法直接访问音频设备",
				"message": "请使用以下方案之一：\n1. 前端浏览器录音（Web Audio API）\n2. 上传音频文件",
			})
			return
		}

		cmd := exec.Command("ffmpeg", "-f", "avfoundation", "-list_devices", "true", "-i", "")
		// ffmpeg prints device list to stderr
		out, _ := cmd.CombinedOutput() // ffmpeg returns non-zero because no input; ignore error
		lines := strings.Split(string(out), "\n")
		section := ""
		// 设备行格式通常为: "[0] MacBook Pro 麦克风" 或前面带一些前缀再跟 [index]
		// 原正则写成 `(\\d+)` 放在 raw string 中导致匹配字面 "\\d" 而不是数字，解析失败
		// 修正为匹配真正的数字分组; 允许前缀噪声，先提取方括号编号再提取名称
		reDev := regexp.MustCompile(`\[(\d+)\]\s+(.+)`) // [0] Name
		devices := []gin.H{}
		for _, ln := range lines {
			ltrim := strings.TrimSpace(ln)
			if strings.Contains(ltrim, "AVFoundation video devices") {
				section = "video"
				continue
			}
			if strings.Contains(ltrim, "AVFoundation audio devices") {
				section = "audio"
				continue
			}
			// 兼容整行里包含其它前缀的情况，先找第一个 "[n]"
			m := reDev.FindStringSubmatch(ltrim)
			if m == nil {
				// 尝试在未裁剪的原始行中匹配 (有时前缀里有颜色/日志信息)
				m = reDev.FindStringSubmatch(ln)
			}
			if m != nil && (section == "audio" || section == "video") {
				devices = append(devices, gin.H{"index": m[1], "name": m[2], "kind": section})
			}
		}
		c.JSON(http.StatusOK, gin.H{"devices": devices})
	}
}

// isRunningInDocker 检测是否在 Docker 容器中运行
func isRunningInDocker() bool {
	// 方法 1: 检查 /.dockerenv 文件
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// 方法 2: 检查 /proc/1/cgroup
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		return strings.Contains(string(data), "docker") || strings.Contains(string(data), "kubepods")
	}

	// 方法 3: 检查环境变量
	if os.Getenv("DOCKER_CONTAINER") == "true" {
		return true
	}

	return false
}
