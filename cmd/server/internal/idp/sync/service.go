// Package sync 提供用户同步服务
package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/houzhh15/AIDG/cmd/server/internal/idp"
	ldapauth "github.com/houzhh15/AIDG/cmd/server/internal/idp/ldap"
	"github.com/houzhh15/AIDG/cmd/server/internal/users"
)

// Service 用户同步服务
type Service struct {
	idpManager  *idp.Manager
	userManager *users.Manager
	logDir      string

	mu         sync.Mutex
	schedulers map[string]*scheduler
	lastLogs   map[string]*idp.SyncLog
	inProgress map[string]bool
}

// scheduler 调度器
type scheduler struct {
	ticker *time.Ticker
	done   chan bool
}

// NewService 创建同步服务
func NewService(idpManager *idp.Manager, userManager *users.Manager, logDir string) *Service {
	// 确保日志目录存在
	os.MkdirAll(logDir, 0755)

	return &Service{
		idpManager:  idpManager,
		userManager: userManager,
		logDir:      logDir,
		schedulers:  make(map[string]*scheduler),
		lastLogs:    make(map[string]*idp.SyncLog),
		inProgress:  make(map[string]bool),
	}
}

// SyncNow 立即执行同步
func (s *Service) SyncNow(idpID string) (*idp.SyncLog, error) {
	s.mu.Lock()
	if s.inProgress[idpID] {
		s.mu.Unlock()
		return nil, idp.ErrSyncInProgress
	}
	s.inProgress[idpID] = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.inProgress, idpID)
		s.mu.Unlock()
	}()

	// 获取 IdP 配置
	provider, err := s.idpManager.Get(idpID)
	if err != nil {
		return nil, err
	}

	// 目前只支持 LDAP 同步
	if provider.Type != idp.TypeLDAP {
		return nil, fmt.Errorf("sync is only supported for LDAP identity providers")
	}

	config, err := provider.GetLDAPConfig()
	if err != nil {
		return nil, err
	}

	// 创建 LDAP 认证器
	auth, err := ldapauth.NewAuthenticator(config)
	if err != nil {
		return nil, err
	}

	// 开始同步
	syncLog := &idp.SyncLog{
		SyncID:    fmt.Sprintf("sync_%d", time.Now().UnixNano()),
		IdpID:     idpID,
		StartedAt: time.Now(),
		Status:    idp.SyncStatusRunning,
	}

	// 获取所有外部用户
	externalUsers, err := auth.FetchAllUsers()
	if err != nil {
		syncLog.Status = idp.SyncStatusFailed
		syncLog.Error = err.Error()
		syncLog.FinishedAt = time.Now()
		s.saveLog(syncLog)
		return syncLog, err
	}

	syncLog.Stats.TotalFetched = len(externalUsers)

	// 获取同步配置
	conflictPolicy := idp.ConflictPolicyOverride
	disableOnRemove := false
	if provider.Sync != nil {
		if provider.Sync.ConflictPolicy != "" {
			conflictPolicy = provider.Sync.ConflictPolicy
		}
		disableOnRemove = provider.Sync.DisableOnRemove
	}

	// 构建外部用户 ID 集合（用于检测删除）
	externalIDSet := make(map[string]bool)

	// 遍历外部用户
	for _, extUser := range externalUsers {
		externalIDSet[extUser.ExternalID] = true

		// 查找或创建用户
		user, created, err := s.userManager.FindOrCreateExternalUser(
			extUser.ExternalID,
			extUser.Username,
			extUser.Email,
			extUser.Fullname,
			idpID,
			config.DefaultScopes,
			config.AutoCreateUser,
		)

		if err != nil {
			// 根据冲突策略处理
			if conflictPolicy == idp.ConflictPolicyIgnore {
				syncLog.Stats.Skipped++
				continue
			}
			syncLog.Stats.Errors++
			continue
		}

		if created {
			syncLog.Stats.Created++
		} else if user != nil {
			// 更新现有用户
			if err := s.userManager.UpdateExternalUserInfo(user.Username, extUser.Email, extUser.Fullname); err != nil {
				syncLog.Stats.Errors++
			} else {
				syncLog.Stats.Updated++
			}
		}
	}

	// 检测并处理已删除的用户
	if disableOnRemove {
		localUsers := s.userManager.ListExternalUsers(idpID)
		for _, localUser := range localUsers {
			if !externalIDSet[localUser.ExternalID] {
				// 用户在外部系统中已不存在，禁用
				if err := s.userManager.DisableUser(localUser.Username); err == nil {
					syncLog.Stats.Disabled++
				}
			}
		}
	}

	// 完成同步
	syncLog.Status = idp.SyncStatusCompleted
	syncLog.FinishedAt = time.Now()

	// 保存日志
	s.saveLog(syncLog)

	// 更新最新日志
	s.mu.Lock()
	s.lastLogs[idpID] = syncLog
	s.mu.Unlock()

	return syncLog, nil
}

// StartScheduler 启动定时调度
func (s *Service) StartScheduler() {
	providers := s.idpManager.ListEnabled()

	for _, provider := range providers {
		if provider.Sync == nil || !provider.Sync.SyncEnabled {
			continue
		}

		interval, err := time.ParseDuration(provider.Sync.SyncInterval)
		if err != nil || interval < time.Minute {
			interval = time.Hour // 默认 1 小时
		}

		s.startSchedulerForIdP(provider.ID, interval)
	}
}

// startSchedulerForIdP 为单个 IdP 启动调度器
func (s *Service) startSchedulerForIdP(idpID string, interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果已存在调度器，先停止
	if existing, ok := s.schedulers[idpID]; ok {
		existing.ticker.Stop()
		close(existing.done)
	}

	ticker := time.NewTicker(interval)
	done := make(chan bool)

	s.schedulers[idpID] = &scheduler{
		ticker: ticker,
		done:   done,
	}

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				s.SyncNow(idpID)
			}
		}
	}()
}

// StopScheduler 停止所有调度器
func (s *Service) StopScheduler() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sched := range s.schedulers {
		sched.ticker.Stop()
		close(sched.done)
	}

	s.schedulers = make(map[string]*scheduler)
}

// StopSchedulerForIdP 停止单个 IdP 的调度器
func (s *Service) StopSchedulerForIdP(idpID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sched, ok := s.schedulers[idpID]; ok {
		sched.ticker.Stop()
		close(sched.done)
		delete(s.schedulers, idpID)
	}
}

// GetSyncStatus 获取同步状态
func (s *Service) GetSyncStatus(idpID string) (*idp.SyncLog, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否正在同步
	if s.inProgress[idpID] {
		return &idp.SyncLog{
			IdpID:     idpID,
			Status:    idp.SyncStatusRunning,
			StartedAt: time.Now(),
		}, true
	}

	// 返回最后一次同步日志
	if log, ok := s.lastLogs[idpID]; ok {
		return log, true
	}

	// 尝试从文件加载最新日志
	log, err := s.loadLatestLog(idpID)
	if err != nil {
		return nil, false
	}

	return log, true
}

// GetSyncLogs 获取同步日志列表
func (s *Service) GetSyncLogs(idpID string, limit int) ([]*idp.SyncLog, error) {
	if limit <= 0 {
		limit = 10
	}

	logDir := filepath.Join(s.logDir, idpID)
	entries, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*idp.SyncLog{}, nil
		}
		return nil, err
	}

	// 按时间倒序排列
	logs := make([]*idp.SyncLog, 0)
	for i := len(entries) - 1; i >= 0 && len(logs) < limit; i-- {
		entry := entries[i]
		if entry.IsDir() {
			continue
		}

		filePath := filepath.Join(logDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var log idp.SyncLog
		if err := json.Unmarshal(data, &log); err != nil {
			continue
		}

		logs = append(logs, &log)
	}

	return logs, nil
}

// saveLog 保存同步日志
func (s *Service) saveLog(log *idp.SyncLog) error {
	logDir := filepath.Join(s.logDir, log.IdpID)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s.json", log.SyncID)
	filePath := filepath.Join(logDir, filename)

	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

// loadLatestLog 加载最新的同步日志
func (s *Service) loadLatestLog(idpID string) (*idp.SyncLog, error) {
	logDir := filepath.Join(s.logDir, idpID)
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no logs found")
	}

	// 获取最新的日志文件
	latestEntry := entries[len(entries)-1]
	filePath := filepath.Join(logDir, latestEntry.Name())

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var log idp.SyncLog
	if err := json.Unmarshal(data, &log); err != nil {
		return nil, err
	}

	return &log, nil
}

// IsSyncing 检查是否正在同步
func (s *Service) IsSyncing(idpID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inProgress[idpID]
}

// RefreshSchedulers 刷新调度器（配置变更后调用）
func (s *Service) RefreshSchedulers() {
	s.StopScheduler()
	s.StartScheduler()
}
