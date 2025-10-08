package main

import (
	"net/http"
	"strings"

	"github.com/houzhh15-hub/AIDG/cmd/mcp-server/shared"
)

// client.go - API 客户端
// 从 main.go 提取的 APIClient 和相关方法

// NewAPIClient 创建 API 客户端实例
// 参数:
//   - baseURL: 后端服务器基础URL，为空则使用默认值
//
// 返回:
//   - *shared.APIClient: 初始化的API客户端实例
func NewAPIClient(baseURL string) *shared.APIClient {
	if baseURL == "" {
		baseURL = shared.DEFAULT_SERVER_BASE_URL
	}

	return &shared.APIClient{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		Client:  &http.Client{},
	}
}
