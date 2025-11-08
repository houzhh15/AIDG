package main

import (
	"log"
	"sync"
)

// NotificationHub 管理 MCP Client 连接并推送变更通知
// 支持两种模式：
// 1. SSE 推送模式：通过注册的 channel 实时推送
// 2. 轮询模式：Client 通过 GetPendingNotifications 获取
type NotificationHub struct {
	// SSE 客户端通道（实时推送）
	sseClients map[chan interface{}]bool

	// 轮询客户端（兼容旧模式）
	clients         map[string]bool // clientID -> 是否活跃
	pendingNotifies []string        // 待发送的通知类型（如 "prompts_changed"）

	mu sync.RWMutex // 并发安全的读写锁
}

// NewNotificationHub 创建通知中心实例
func NewNotificationHub() *NotificationHub {
	return &NotificationHub{
		sseClients:      make(map[chan interface{}]bool),
		clients:         make(map[string]bool),
		pendingNotifies: []string{},
	}
}

// RegisterClient 注册 MCP Client（轮询模式）
func (nh *NotificationHub) RegisterClient(clientID string) {
	nh.mu.Lock()
	defer nh.mu.Unlock()

	nh.clients[clientID] = true
	log.Printf("✅ [NOTIFICATION] Client 已注册(轮询): %s (当前连接数: %d)", clientID, len(nh.clients))
}

// UnregisterClient 注销 MCP Client（轮询模式）
func (nh *NotificationHub) UnregisterClient(clientID string) {
	nh.mu.Lock()
	defer nh.mu.Unlock()

	if _, exists := nh.clients[clientID]; exists {
		delete(nh.clients, clientID)
		log.Printf("🔌 [NOTIFICATION] Client 已注销(轮询): %s (当前连接数: %d)", clientID, len(nh.clients))
	}
}

// RegisterSSEClient 注册 SSE 客户端（实时推送模式）
func (nh *NotificationHub) RegisterSSEClient(clientChan chan interface{}) {
	nh.mu.Lock()
	defer nh.mu.Unlock()

	nh.sseClients[clientChan] = true
	log.Printf("✅ [NOTIFICATION] SSE Client 已注册 (当前 SSE 连接数: %d)", len(nh.sseClients))
}

// UnregisterSSEClient 注销 SSE 客户端
func (nh *NotificationHub) UnregisterSSEClient(clientChan chan interface{}) {
	nh.mu.Lock()
	defer nh.mu.Unlock()

	if _, exists := nh.sseClients[clientChan]; exists {
		delete(nh.sseClients, clientChan)
		close(clientChan)
		log.Printf("🔌 [NOTIFICATION] SSE Client 已注销 (当前 SSE 连接数: %d)", len(nh.sseClients))
	}
}

// BroadcastPromptsChanged 广播 Prompts 变更通知
// 同时支持 SSE 推送和轮询模式
func (nh *NotificationHub) BroadcastPromptsChanged() {
	nh.mu.Lock()
	defer nh.mu.Unlock()

	// 1. 通过 SSE 实时推送
	for clientChan := range nh.sseClients {
		select {
		case clientChan <- "prompts_changed":
			// 成功发送
		default:
			// 通道满，跳过此客户端
			log.Printf("⚠️  [NOTIFICATION] SSE Client 通道已满，跳过通知")
		}
	}

	// 2. 记录待通知事件（供轮询）
	nh.pendingNotifies = append(nh.pendingNotifies, "prompts_changed")

	log.Printf("📢 [NOTIFICATION] Prompts 变更通知已广播 (SSE 客户端: %d, 轮询客户端: %d)",
		len(nh.sseClients), len(nh.clients))
	log.Printf("ℹ️  [NOTIFICATION] MCP 规范通知格式: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/prompts/list_changed\"}")
}

// GetPendingNotifications 获取待通知事件并清空（供 Client 轮询调用）
func (nh *NotificationHub) GetPendingNotifications() []string {
	nh.mu.Lock()
	defer nh.mu.Unlock()

	notifications := make([]string, len(nh.pendingNotifies))
	copy(notifications, nh.pendingNotifies)

	// 清空待通知列表
	nh.pendingNotifies = []string{}

	return notifications
}

// HasPromptsChanged 检查是否有 Prompts 变更（供 Client 快速查询）
func (nh *NotificationHub) HasPromptsChanged() bool {
	nh.mu.RLock()
	defer nh.mu.RUnlock()

	for _, notify := range nh.pendingNotifies {
		if notify == "prompts_changed" {
			return true
		}
	}

	return false
}
