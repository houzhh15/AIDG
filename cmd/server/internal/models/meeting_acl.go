package models

import (
	"time"
)

// MeetingACL 会议访问控制列表 - 存储在 meetings/{project_id}/{meeting_id}_acl.json
type MeetingACL struct {
	MeetingID string     `json:"meeting_id"` // 会议 ID
	Owner     string     `json:"owner"`      // 会议创建者
	ACL       []ACLEntry `json:"acl"`        // 访问控制列表
	UpdatedAt time.Time  `json:"updated_at"` // 最后更新时间
}

// ACLEntry 访问控制条目
type ACLEntry struct {
	Username   string    `json:"username"`   // 用户名
	Permission string    `json:"permission"` // 权限类型 (如: "read", "write")
	GrantedAt  time.Time `json:"granted_at"` // 授权时间
}

// ACLRequest 会议 ACL 修改请求 - 用于 API 接口
type ACLRequest struct {
	MeetingID  string `json:"meeting_id"` // 会议 ID
	Username   string `json:"username"`   // 用户名
	Permission string `json:"permission"` // 权限类型
}

// MeetingACLResponse 会议 ACL 响应 - 用于 API 返回
type MeetingACLResponse struct {
	MeetingID string     `json:"meeting_id"` // 会议 ID
	Owner     string     `json:"owner"`      // 会议创建者
	ACL       []ACLEntry `json:"acl"`        // 访问控制列表
}

// ToResponse 转换为响应格式
func (m *MeetingACL) ToResponse() MeetingACLResponse {
	return MeetingACLResponse{
		MeetingID: m.MeetingID,
		Owner:     m.Owner,
		ACL:       m.ACL,
	}
}
