package services

import (
	"encoding/json"
	"fmt"
	"github.com/houzhh15/AIDG/cmd/server/internal/constants"
	"github.com/houzhh15/AIDG/cmd/server/internal/models"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PermissionInjector 权限注入器接口
type PermissionInjector interface {
	// InjectTaskOwnerPermissions 注入任务负责人权限
	InjectTaskOwnerPermissions(username, projectID, taskID string, baseScopes []string) ([]string, error)
	// InjectMeetingOwnerPermissions 注入会议创建者权限
	InjectMeetingOwnerPermissions(username, meetingID string, baseScopes []string) ([]string, error)
	// CheckMeetingACL 检查会议ACL授权
	CheckMeetingACL(username, meetingID string) (hasRead, hasWrite bool, err error)
}

// DefaultPermissionInjector 默认权限注入器实现
type DefaultPermissionInjector struct {
	projectsRoot string
	meetingsRoot string
}

var safeMeetingID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// NewPermissionInjector 创建权限注入器实例
func NewPermissionInjector(projectsRoot, meetingsRoot string) PermissionInjector {
	return &DefaultPermissionInjector{
		projectsRoot: projectsRoot,
		meetingsRoot: meetingsRoot,
	}
}

// InjectTaskOwnerPermissions 注入任务负责人权限
// 如果用户是任务负责人,自动注入 task.read 和 task.write
func (dpi *DefaultPermissionInjector) InjectTaskOwnerPermissions(
	username, projectID, taskID string,
	baseScopes []string,
) ([]string, error) {
	// 1. 查询任务负责人
	assignee, err := dpi.getTaskAssignee(projectID, taskID)
	if err != nil {
		// 如果任务不存在,返回原权限,不报错
		if os.IsNotExist(err) {
			return baseScopes, nil
		}
		return nil, fmt.Errorf("failed to get task assignee: %w", err)
	}

	// 2. 如果不是负责人,返回原权限
	if assignee != username {
		return baseScopes, nil
	}

	// 3. 注入任务负责人默认权限
	injectedScopes := make([]string, 0, len(baseScopes)+2)
	injectedScopes = append(injectedScopes, baseScopes...)

	// 去重添加 task.read 和 task.write
	if !contains(injectedScopes, constants.ScopeTaskRead) {
		injectedScopes = append(injectedScopes, constants.ScopeTaskRead)
	}
	if !contains(injectedScopes, constants.ScopeTaskWrite) {
		injectedScopes = append(injectedScopes, constants.ScopeTaskWrite)
	}

	return injectedScopes, nil
}

// InjectMeetingOwnerPermissions 注入会议创建者权限
// 如果用户是会议创建者,自动注入 meeting.read 和 meeting.write
func (dpi *DefaultPermissionInjector) InjectMeetingOwnerPermissions(
	username, meetingID string,
	baseScopes []string,
) ([]string, error) {
	if username == "" || !validMeetingDirectory(dpi.meetingsRoot, meetingID) {
		return baseScopes, nil
	}
	// 1. 查询会议 owner
	owner, err := dpi.getMeetingOwner(meetingID)
	if err != nil {
		// 如果会议不存在,返回原权限,不报错
		if os.IsNotExist(err) {
			return baseScopes, nil
		}
		return nil, fmt.Errorf("failed to get meeting owner: %w", err)
	}

	// 2. 如果不是 owner,返回原权限
	if owner != username {
		return baseScopes, nil
	}

	// 3. 注入会议创建者默认权限
	injectedScopes := make([]string, 0, len(baseScopes)+2)
	injectedScopes = append(injectedScopes, baseScopes...)

	// 去重添加 meeting.read 和 meeting.write
	if !contains(injectedScopes, constants.ScopeMeetingRead) {
		injectedScopes = append(injectedScopes, constants.ScopeMeetingRead)
	}
	if !contains(injectedScopes, constants.ScopeMeetingWrite) {
		injectedScopes = append(injectedScopes, constants.ScopeMeetingWrite)
	}

	return injectedScopes, nil
}

// CheckMeetingACL 检查会议ACL授权
// 查询会议ACL列表,返回用户的读写权限
func (dpi *DefaultPermissionInjector) CheckMeetingACL(username, meetingID string) (hasRead, hasWrite bool, err error) {
	if username == "" || !validMeetingDirectory(dpi.meetingsRoot, meetingID) {
		return false, false, nil
	}
	// 1. 读取会议ACL文件
	aclPath := filepath.Join(dpi.meetingsRoot, meetingID, "acl.json")
	relative, pathErr := filepath.Rel(dpi.meetingsRoot, aclPath)
	if pathErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false, false, nil
	}
	data, readErr := readPermissionFile(aclPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			// ACL文件不存在,无额外授权
			return false, false, nil
		}
		return false, false, fmt.Errorf("failed to read meeting ACL: %w", readErr)
	}

	// 2. 解析ACL
	var acl models.MeetingACL
	if err := json.Unmarshal(data, &acl); err != nil {
		return false, false, fmt.Errorf("failed to unmarshal meeting ACL: %w", err)
	}

	// 3. 查找用户的授权
	for _, entry := range acl.ACL {
		if entry.Username == username {
			if entry.Permission == "write" {
				return true, true, nil // write 隐含 read
			}
			if entry.Permission == "read" {
				hasRead = true
			}
		}
	}

	return hasRead, hasWrite, nil
}

// ========== 内部辅助方法 ==========

// getTaskAssignee 获取任务负责人
func (dpi *DefaultPermissionInjector) getTaskAssignee(projectID, taskID string) (string, error) {
	// 读取项目的 tasks.json
	tasksPath := filepath.Join(dpi.projectsRoot, projectID, "tasks.json")
	data, err := os.ReadFile(tasksPath)
	if err != nil {
		return "", err
	}

	// 解析任务列表
	var tasks []map[string]interface{}
	if err := json.Unmarshal(data, &tasks); err != nil {
		return "", fmt.Errorf("unmarshal tasks: %w", err)
	}

	// 查找目标任务
	for _, task := range tasks {
		if id, ok := task["id"].(string); ok && id == taskID {
			if assignee, ok := task["assignee"].(string); ok {
				return assignee, nil
			}
			// 任务存在但无 assignee 字段
			return "", nil
		}
	}

	// 任务不存在
	return "", os.ErrNotExist
}

// getMeetingOwner 获取会议创建者
func (dpi *DefaultPermissionInjector) getMeetingOwner(meetingID string) (string, error) {
	if !validMeetingID(meetingID) {
		return "", os.ErrNotExist
	}
	// 方案1: 从会议 metadata.json 读取 owner (推荐)
	metadataPath := filepath.Join(dpi.meetingsRoot, meetingID, "metadata.json")
	relative, err := filepath.Rel(dpi.meetingsRoot, metadataPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", os.ErrNotExist
	}
	data, err := readPermissionFile(metadataPath)
	if err != nil {
		// 如果 metadata 不存在,尝试从 ACL 读取
		if os.IsNotExist(err) {
			return dpi.getMeetingOwnerFromACL(meetingID)
		}
		return "", err
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return "", fmt.Errorf("unmarshal metadata: %w", err)
	}

	if owner, ok := metadata["owner"].(string); ok {
		return owner, nil
	}

	// metadata 存在但无 owner 字段,尝试从 ACL 读取
	return dpi.getMeetingOwnerFromACL(meetingID)
}

// getMeetingOwnerFromACL 从 ACL 文件读取会议 owner
func (dpi *DefaultPermissionInjector) getMeetingOwnerFromACL(meetingID string) (string, error) {
	if !validMeetingID(meetingID) {
		return "", os.ErrNotExist
	}
	aclPath := filepath.Join(dpi.meetingsRoot, meetingID, "acl.json")
	data, err := readPermissionFile(aclPath)
	if err != nil {
		return "", err
	}

	var acl models.MeetingACL
	if err := json.Unmarshal(data, &acl); err != nil {
		return "", fmt.Errorf("unmarshal ACL: %w", err)
	}

	return acl.Owner, nil
}

// contains 检查切片中是否包含指定元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func validMeetingID(meetingID string) bool {
	return safeMeetingID.MatchString(meetingID) && meetingID != "." && meetingID != ".."
}

func validMeetingDirectory(root, meetingID string) bool {
	if !validMeetingID(meetingID) {
		return false
	}
	info, err := os.Lstat(filepath.Join(root, meetingID))
	return err == nil && info.Mode()&os.ModeSymlink == 0 && info.IsDir()
}

func readPermissionFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, fmt.Errorf("invalid permission file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) || openedInfo.Size() > 1<<20 {
		return nil, fmt.Errorf("permission file changed during open")
	}
	limited := &io.LimitedReader{R: file, N: (1 << 20) + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("permission file too large")
	}
	return data, nil
}
