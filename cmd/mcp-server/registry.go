package main

import (
	"fmt"

	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
)

// tools/registry.go - 工具注册表
// Tool 接口定义和 ToolRegistry 实现

// ToolRegistry 工具注册表
// 管理所有可用工具，提供注册、查询和调用功能
type ToolRegistry struct {
	tools map[string]shared.Tool // 工具名称 -> 工具实例的映射
}

// NewToolRegistry 创建新的工具注册表实例
// 返回:
//   - *ToolRegistry: 初始化的注册表实例
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]shared.Tool),
	}
}

// Register 注册一个工具到注册表
// 参数:
//   - tool: 要注册的工具实例
//
// 注意: 如果工具名称已存在，会覆盖原有工具
func (r *ToolRegistry) Register(tool shared.Tool) {
	r.tools[tool.Name()] = tool
}

// Get 根据名称获取工具实例
// 参数:
//   - name: 工具名称
//
// 返回:
//   - Tool: 工具实例
//   - error: 如果工具不存在，返回错误
func (r *ToolRegistry) Get(name string) (shared.Tool, error) {
	tool, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool '%s' not found", name)
	}
	return tool, nil
}

// List 返回所有已注册工具的元数据列表
// 返回格式符合 MCP 协议的 tools/list 响应
// 返回:
//   - []map[string]interface{}: 工具元数据列表，每个元素包含 name, description, inputSchema
func (r *ToolRegistry) List() []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, map[string]interface{}{
			"name":        tool.Name(),
			"description": tool.Description(),
			"inputSchema": tool.InputSchema(),
		})
	}
	return result
}

// Execute 执行指定名称的工具
// 这是一个便捷方法，封装了工具查找和执行的流程
// 参数:
//   - name: 工具名称
//   - args: 工具参数
//   - clientToken: 客户端认证 token
//   - apiClient: API 客户端实例
//
// 返回:
//   - string: 工具执行结果
//   - error: 工具不存在或执行错误
func (r *ToolRegistry) Execute(name string, args map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	tool, err := r.Get(name)
	if err != nil {
		return "", err
	}
	return tool.Execute(args, clientToken, apiClient)
}
