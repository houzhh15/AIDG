package main

import (
	// Standard library
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	// External dependencies
	"github.com/gin-gonic/gin"

	// Internal packages
	"github.com/houzhh15/AIDG/cmd/server/internal/api"
	"github.com/houzhh15/AIDG/cmd/server/internal/audit"
	"github.com/houzhh15/AIDG/cmd/server/internal/config"
	documents "github.com/houzhh15/AIDG/cmd/server/internal/documents"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/docslot"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/meetings"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
	syncdomain "github.com/houzhh15/AIDG/cmd/server/internal/domain/sync"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/taskdocs"
	executionplan "github.com/houzhh15/AIDG/cmd/server/internal/executionplan"
	"github.com/houzhh15/AIDG/cmd/server/internal/handlers"
	"github.com/houzhh15/AIDG/cmd/server/internal/idp"
	idpsync "github.com/houzhh15/AIDG/cmd/server/internal/idp/sync"
	"github.com/houzhh15/AIDG/cmd/server/internal/middleware"
	"github.com/houzhh15/AIDG/cmd/server/internal/orchestrator"
	"github.com/houzhh15/AIDG/cmd/server/internal/prompt"
	"github.com/houzhh15/AIDG/cmd/server/internal/resource"
	"github.com/houzhh15/AIDG/cmd/server/internal/services"
	"github.com/houzhh15/AIDG/cmd/server/internal/users"
	"github.com/houzhh15/AIDG/pkg/logger"
	"github.com/houzhh15/AIDG/pkg/similarity"
)

// generateRandomPassword generates a cryptographically secure random password
func generateRandomPassword(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("failed to generate random password: %v", err))
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}

// sectionServiceAdapter adapts taskdocs.SectionService to similarity.SectionServiceInterface
type sectionServiceAdapter struct {
	svc taskdocs.SectionService
}

func (a *sectionServiceAdapter) GetSections(projectID, taskID, docType string) (*similarity.SectionsResponse, error) {
	meta, err := a.svc.GetSections(projectID, taskID, docType)
	if err != nil {
		return nil, err
	}
	resp := &similarity.SectionsResponse{
		Sections: make([]similarity.SectionMeta, len(meta.Sections)),
	}
	for i, s := range meta.Sections {
		resp.Sections[i] = similarity.SectionMeta{
			ID:    s.ID,
			Title: s.Title,
		}
	}
	return resp, nil
}

func (a *sectionServiceAdapter) GetSection(projectID, taskID, docType, sectionID string, includeChildren bool) (*similarity.SectionResponse, error) {
	content, err := a.svc.GetSection(projectID, taskID, docType, sectionID, includeChildren)
	if err != nil {
		return nil, err
	}
	return &similarity.SectionResponse{
		ID:      content.ID,
		Title:   content.Title,
		Content: content.Content,
	}, nil
}

func main() {
	logInstance, err := logger.Init(logger.Config{
		Level:       os.Getenv("LOG_LEVEL"),
		Environment: os.Getenv("ENV"),
		WithSource:  !strings.EqualFold(os.Getenv("ENV"), "prod"),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger init failed: %v\n", err)
		os.Exit(1)
	}
	appLogger := logInstance.With("component", "web-server")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		appLogger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Validate configuration
	if err := config.ValidateConfig(cfg); err != nil {
		appLogger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	appLogger.Info("configuration loaded", "env", cfg.Server.Env, "port", cfg.Server.Port)

	if cfg.Server.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize user secret
	userSecret := []byte(cfg.Security.JWTSecret)
	if len(userSecret) == 0 {
		userSecret = []byte("dev-secret-change-me")
	}

	// Initialize task document service
	taskDocSvc := taskdocs.NewDocService()
	appLogger.Info("task document service ready", "mode", "append")

	projectsRoot := cfg.Data.ProjectsDir
	if projectsRoot == "" {
		projectsRoot = "./projects"
	}

	// Data root directory for prompts and other data storage
	// Note: Prompts use a separate data root that includes users/, projects/, and prompts/
	dataRoot := "./data"

	// Initialize multi-level document handler
	docHandler := documents.NewHandler(projectsRoot)
	appLogger.Info("multi-level document handler ready", "baseDir", projectsRoot)

	// Initialize user manager
	userStoreDir := cfg.Data.UsersDir
	if userStoreDir == "" {
		userStoreDir = "users"
	}
	userManager, err := users.NewManager(userStoreDir, userSecret)
	if err != nil {
		appLogger.Error("user manager init failed", "error", err)
		os.Exit(1)
	}

	// Initialize prompt system (step-04)
	promptStorage := prompt.NewPromptStorage(dataRoot)
	promptPermChecker := prompt.NewPromptPermissionChecker(userManager)
	// 触发文件路径：MCP Server 会监听此文件变化来重新加载 Prompts
	promptsTriggerPath := filepath.Join(dataRoot, ".prompts_changed")
	promptsHandler := api.NewPromptsHandler(promptStorage, promptPermChecker, userManager, promptsTriggerPath)
	appLogger.Info("prompt system ready", "baseDir", dataRoot, "triggerPath", promptsTriggerPath)

	// Ensure default admin with config-based password
	adminPassword := cfg.Security.AdminDefaultPassword
	if adminPassword == "" {
		if cfg.Server.Env == "dev" {
			// Generate random password for dev environment
			adminPassword = generateRandomPassword(16)
			appLogger.Warn("generated random admin password", "password", adminPassword)
		} else {
			appLogger.Error("admin default password not set in production/staging")
			os.Exit(1)
		}
	}
	if err := userManager.EnsureDefaultAdmin(adminPassword); err != nil {
		appLogger.Warn("failed to ensure default admin", "error", err)
	}

	// Load task and project registries
	projects.InitPaths()
	meetings.InitPaths()
	syncdomain.InitPaths()
	meetingsReg := meetings.NewRegistry()
	if err := meetings.LoadTasks(meetingsReg); err != nil {
		appLogger.Warn("failed to load tasks", "error", err)
	}
	// Note: ScanTaskDirs not available in refactored code, tasks loaded from JSON

	projectsReg := projects.NewProjectRegistry()
	if err := projects.LoadProjects(projectsReg); err != nil {
		appLogger.Warn("failed to load projects", "error", err)
	}
	// Note: ScanProjectDirs not available in refactored code, projects loaded from JSON

	appLogger.Info("loaded registries", "tasks", len(meetingsReg.List()), "projects", len(projectsReg.List()))

	// Initialize new services for project status page
	baseDir := projectsRoot
	roadmapService := services.NewRoadmapService(baseDir)
	statisticsService := services.NewStatisticsService(baseDir)
	projectOverviewService := services.NewProjectOverviewService(baseDir, statisticsService)
	progressService := services.NewProgressService(baseDir)
	taskSummaryService := services.NewTaskSummaryService(baseDir)
	appLogger.Info("project status services ready")

	// Initialize audit logger
	auditLogsDir := filepath.Join(baseDir, "audit_logs")
	auditLogger, err := audit.NewFileAuditLogger(auditLogsDir)
	if err != nil {
		appLogger.Error("audit logger init failed", "error", err)
		os.Exit(1)
	}
	appLogger.Info("audit logger ready")

	// Initialize role management services
	rolesDir := filepath.Join(baseDir, "roles")
	userRolesDir := filepath.Join(baseDir, "user_roles")
	roleManager, err := services.NewRoleManager(rolesDir, auditLogger)
	if err != nil {
		appLogger.Error("role manager init failed", "error", err)
		os.Exit(1)
	}
	userRoleService, err := services.NewUserRoleService(userRolesDir, roleManager, auditLogger)
	if err != nil {
		appLogger.Error("user role service init failed", "error", err)
		os.Exit(1)
	}
	appLogger.Info("role management services ready")

	// Initialize ResourceManager
	resourceManager := resource.NewResourceManager(baseDir)
	if err := resourceManager.LoadAll(); err != nil {
		appLogger.Warn("failed to load resources, starting fresh", "error", err)
	}
	appLogger.Info("resource manager ready")

	// Initialize Identity Provider Manager
	idpStoreDir := filepath.Join(baseDir, "identity_providers")
	idpManager, err := idp.NewManager(idpStoreDir)
	if err != nil {
		appLogger.Warn("idp manager init failed, identity provider features disabled", "error", err)
	} else {
		appLogger.Info("idp manager ready", "count", idpManager.Count())
	}

	// Initialize IdP Sync Service
	idpSyncLogsDir := filepath.Join(baseDir, "identity_providers", "sync_logs")
	var idpSyncService *idpsync.Service
	if idpManager != nil {
		idpSyncService = idpsync.NewService(idpManager, userManager, idpSyncLogsDir)
		idpSyncService.StartScheduler()
		appLogger.Info("idp sync service ready")
	}

	// Initialize IdP Handler
	var idpHandler *api.IdPHandler
	if idpManager != nil {
		if idpSyncService != nil {
			idpHandler = api.NewIdPHandlerWithSync(idpManager, userManager, idpSyncService)
		} else {
			idpHandler = api.NewIdPHandler(idpManager, userManager)
		}
		appLogger.Info("idp handler ready")
	}

	// Initialize permission injector
	meetingsRoot := cfg.Data.MeetingsDir
	permissionInjector := services.NewPermissionInjector(baseDir, meetingsRoot)
	appLogger.Info("permission injector ready")

	// Initialize environment handler
	envHandler := handlers.NewEnvironmentHandler()
	appLogger.Info("environment handler ready")

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger())

	// Add health check endpoints (no authentication required)
	startTime := time.Now()
	r.GET("/health", healthCheckHandler(cfg, startTime))
	r.GET("/api/v1/health", healthCheckHandler(cfg, startTime)) // Alternative API path
	r.GET("/readiness", readinessCheckHandler(cfg))

	// Add Whisper service endpoints (no authentication required)
	r.GET("/api/v1/services/whisper/models", api.HandleGetWhisperModels())
	r.GET("/api/v1/services/whisper/models-extended", api.HandleGetWhisperModelsExtended())
	r.POST("/api/v1/services/whisper/models/download", api.HandleDownloadWhisperModel())

	// Services Status API (no authentication required)
	// 检查服务部署状态（whisper 和 deps-service）
	// 使用全局检查器，不依赖于具体的会议任务
	globalServiceChecker := api.NewGlobalServiceChecker()
	r.GET("/api/v1/services/status", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		status := globalServiceChecker.CheckAllServices(ctx)
		c.JSON(http.StatusOK, status)
	})

	// Debug endpoint (no authentication required for testing)
	r.POST("/api/v1/debug/tasks/:id/enqueue/:chunk_id", api.HandleDebugEnqueueChunk(meetingsReg))

	// Tag Handler for document version management
	tagHandler := api.NewTagHandler(projectsRoot)

	// Setup authentication and routes
	setupAuthMiddleware(r, userManager, userRoleService, permissionInjector, baseDir, logInstance.With("component", "auth-middleware"))
	setupRoutes(r, meetingsReg, projectsReg, docHandler, taskDocSvc, userManager, roadmapService, projectOverviewService, statisticsService, progressService, taskSummaryService, roleManager, userRoleService, permissionInjector, envHandler, projectsRoot, resourceManager, promptsHandler, tagHandler, idpHandler)

	// Check frontend dist directory
	frontendDistDir := cfg.Frontend.DistDir
	if frontendDistDir == "" {
		frontendDistDir = "./frontend/dist"
	}
	indexPath := filepath.Join(frontendDistDir, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		appLogger.Warn("frontend index.html not found - run 'npm run build' in frontend directory", "path", indexPath)
	} else {
		appLogger.Info("frontend dist directory ready", "path", frontendDistDir)
	}

	// Create HTTP server with graceful shutdown
	serverAddr := fmt.Sprintf(":%s", cfg.Server.Port)
	srv := &http.Server{
		Addr:    serverAddr,
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		protocol := strings.ToLower(cfg.Server.Protocol)
		appLogger.Info("server starting", "addr", serverAddr, "protocol", protocol, "env", cfg.Server.Env)

		var err error
		if protocol == "https" {
			// HTTPS mode
			if cfg.Server.TLSCert == "" || cfg.Server.TLSKey == "" {
				appLogger.Error("TLS certificate or key file not specified for HTTPS mode")
				os.Exit(1)
			}
			appLogger.Info("starting HTTPS server", "cert", cfg.Server.TLSCert, "key", cfg.Server.TLSKey)
			err = srv.ListenAndServeTLS(cfg.Server.TLSCert, cfg.Server.TLSKey)
		} else {
			// HTTP mode (default)
			err = srv.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			appLogger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-quit
	appLogger.Info("shutdown signal received, shutting down server...")

	// Shutdown IdP sync service
	if idpSyncService != nil {
		idpSyncService.StopScheduler()
		appLogger.Info("idp sync scheduler stopped")
	}

	// Shutdown resource manager
	resourceManager.Shutdown()

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		appLogger.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}
	appLogger.Info("server shutdown complete")
}

// hasAnyProjectPermission 检查用户是否在任何项目中拥有指定权限
func hasAnyProjectPermission(userRoleService services.UserRoleService, username, scope string) bool {
	// 获取用户的所有项目角色
	profile, err := userRoleService.GetUserProfile(username)
	if err != nil {
		return false
	}

	// 检查每个项目的权限
	for _, roleInfo := range profile.ProjectRoles {
		scopes, err := userRoleService.ComputeEffectiveScopes(username, roleInfo.ProjectID)
		if err != nil {
			continue // 跳过错误的项目
		}

		for _, userScope := range scopes {
			if userScope == scope {
				return true
			}
		}
	}
	return false
}

func setupAuthMiddleware(r *gin.Engine, userManager *users.Manager, userRoleService services.UserRoleService, injector services.PermissionInjector, projectsRoot string, authLogger *slog.Logger) {
	// Route scope mapping
	routeScopes := map[string][]string{
		"GET /api/v1/me/token": {users.ScopeMeetingRead},
		// GET /api/v1/users 不需要特殊权限，所有已认证用户都可以获取用户列表（用于任务分配等场景）
		"POST /api/v1/users": {users.ScopeUserManage},
		// Role Management - 需要用户管理权限 (query parameter style)
		"POST /api/v1/roles": {users.ScopeUserManage}, "GET /api/v1/roles": {users.ScopeUserManage},
		"GET /api/v1/roles/:role_id": {users.ScopeUserManage}, "PUT /api/v1/roles/:role_id": {users.ScopeUserManage}, "DELETE /api/v1/roles/:role_id": {users.ScopeUserManage},
		"POST /api/v1/users/roles": {users.ScopeUserManage}, "DELETE /api/v1/users/roles": {users.ScopeUserManage},
		"GET /api/v1/users/:username/permissions": {users.ScopeUserManage}, "GET /api/v1/users/:username/profile": {users.ScopeUserManage},
		// 当前用户档案 - 所有已登录用户都可访问
		"GET /api/v1/user/profile": {users.ScopeMeetingRead},
		// Role Management - RESTful style
		"GET /api/v1/projects/:id/roles": {users.ScopeUserManage}, "POST /api/v1/projects/:id/roles": {users.ScopeUserManage},
		"GET /api/v1/projects/:id/roles/:role_id": {users.ScopeUserManage}, "PUT /api/v1/projects/:id/roles/:role_id": {users.ScopeUserManage}, "DELETE /api/v1/projects/:id/roles/:role_id": {users.ScopeUserManage},
		"GET /api/v1/projects/:id/users/:username/roles": {users.ScopeUserManage}, "POST /api/v1/projects/:id/users/:username/roles": {users.ScopeUserManage},
		"DELETE /api/v1/projects/:id/users/:username/roles/:role_id": {users.ScopeUserManage}, "GET /api/v1/projects/:id/user-roles": {users.ScopeUserManage},
		// User Management
		"GET /api/v1/users/:username": {users.ScopeUserManage}, "PATCH /api/v1/users/:username": {users.ScopeUserManage}, "DELETE /api/v1/users/:username": {users.ScopeUserManage},
		"POST /api/v1/users/:username/password": {users.ScopeUserManage},
		// 项目基础API - GET /api/v1/projects 不需要权限（所有登录用户可访问）
		// "GET /api/v1/projects": 不设置权限要求，所有已登录用户都可以查看项目列表
		"POST /api/v1/projects":    {users.ScopeProjectAdmin},
		"GET /api/v1/projects/:id": {users.ScopeProjectDocRead}, "PATCH /api/v1/projects/:id": {users.ScopeProjectDocWrite},
		"DELETE /api/v1/projects/:id": {users.ScopeProjectDocWrite, users.ScopeProjectAdmin},
		// 项目文档API - 特性列表
		"GET /api/v1/projects/:id/feature-list": {users.ScopeProjectDocRead}, "PUT /api/v1/projects/:id/feature-list": {users.ScopeProjectDocWrite},
		"GET /api/v1/projects/:id/feature-list/history": {users.ScopeProjectDocRead}, "DELETE /api/v1/projects/:id/feature-list/history/:version": {users.ScopeProjectDocWrite},
		"GET /api/v1/projects/:id/feature-list.json": {users.ScopeProjectDocRead}, "PUT /api/v1/projects/:id/feature-list.json": {users.ScopeProjectDocWrite},
		// 项目文档API - 架构设计
		"GET /api/v1/projects/:id/architecture-design": {users.ScopeProjectDocRead}, "PUT /api/v1/projects/:id/architecture-design": {users.ScopeProjectDocWrite},
		"GET /api/v1/projects/:id/architecture-design/history": {users.ScopeProjectDocRead}, "DELETE /api/v1/projects/:id/architecture-design/history/:version": {users.ScopeProjectDocWrite},
		// 项目文档API - 技术设计
		"GET /api/v1/projects/:id/tech-design": {users.ScopeProjectDocRead}, "PUT /api/v1/projects/:id/tech-design": {users.ScopeProjectDocWrite},
		"GET /api/v1/projects/:id/tech-design/history": {users.ScopeProjectDocRead}, "DELETE /api/v1/projects/:id/tech-design/history/:version": {users.ScopeProjectDocWrite},
		"POST /api/v1/projects/:id/copy-from-task": {users.ScopeProjectDocWrite},
		"GET /api/v1/projects/:id/tasks":           {users.ScopeTaskRead}, "POST /api/v1/projects/:id/tasks": {users.ScopeTaskWrite},
		"GET /api/v1/projects/:id/tasks/:task_id": {users.ScopeTaskRead}, "PUT /api/v1/projects/:id/tasks/:task_id": {users.ScopeTaskWrite},
		"DELETE /api/v1/projects/:id/tasks/:task_id":           {users.ScopeTaskWrite},
		"GET /api/v1/projects/:id/tasks/:task_id/requirements": {users.ScopeTaskRead}, "PUT /api/v1/projects/:id/tasks/:task_id/requirements": {users.ScopeTaskWrite},
		"GET /api/v1/projects/:id/tasks/:task_id/design": {users.ScopeTaskRead}, "PUT /api/v1/projects/:id/tasks/:task_id/design": {users.ScopeTaskWrite},
		"GET /api/v1/projects/:id/tasks/:task_id/test": {users.ScopeTaskRead}, "PUT /api/v1/projects/:id/tasks/:task_id/test": {users.ScopeTaskWrite},
		"POST /api/v1/projects/:id/tasks/:task_id/requirements/append": {users.ScopeTaskWrite}, "GET /api/v1/projects/:id/tasks/:task_id/requirements/chunks": {users.ScopeTaskRead},
		"DELETE /api/v1/projects/:id/tasks/:task_id/requirements/chunks/:seq": {users.ScopeTaskWrite}, "GET /api/v1/projects/:id/tasks/:task_id/requirements/export": {users.ScopeTaskRead},
		"POST /api/v1/projects/:id/tasks/:task_id/design/append": {users.ScopeTaskWrite}, "GET /api/v1/projects/:id/tasks/:task_id/design/chunks": {users.ScopeTaskRead},
		"DELETE /api/v1/projects/:id/tasks/:task_id/design/chunks/:seq": {users.ScopeTaskWrite}, "GET /api/v1/projects/:id/tasks/:task_id/design/export": {users.ScopeTaskRead},
		"POST /api/v1/projects/:id/tasks/:task_id/test/append": {users.ScopeTaskWrite}, "GET /api/v1/projects/:id/tasks/:task_id/test/chunks": {users.ScopeTaskRead},
		"DELETE /api/v1/projects/:id/tasks/:task_id/test/chunks/:seq": {users.ScopeTaskWrite}, "GET /api/v1/projects/:id/tasks/:task_id/test/export": {users.ScopeTaskRead},
		// Requirements sections
		"GET /api/v1/projects/:id/tasks/:task_id/requirements/sections":                  {users.ScopeTaskRead},
		"GET /api/v1/projects/:id/tasks/:task_id/requirements/sections/:section_id":      {users.ScopeTaskRead},
		"PUT /api/v1/projects/:id/tasks/:task_id/requirements/sections/:section_id":      {users.ScopeTaskWrite},
		"PUT /api/v1/projects/:id/tasks/:task_id/requirements/sections/:section_id/full": {users.ScopeTaskWrite},
		"POST /api/v1/projects/:id/tasks/:task_id/requirements/sections":                 {users.ScopeTaskWrite},
		"DELETE /api/v1/projects/:id/tasks/:task_id/requirements/sections/:section_id":   {users.ScopeTaskWrite},
		"PATCH /api/v1/projects/:id/tasks/:task_id/requirements/sections/reorder":        {users.ScopeTaskWrite},
		"POST /api/v1/projects/:id/tasks/:task_id/requirements/sections/sync":            {users.ScopeTaskWrite},
		// Design sections
		"GET /api/v1/projects/:id/tasks/:task_id/design/sections":                  {users.ScopeTaskRead},
		"GET /api/v1/projects/:id/tasks/:task_id/design/sections/:section_id":      {users.ScopeTaskRead},
		"PUT /api/v1/projects/:id/tasks/:task_id/design/sections/:section_id":      {users.ScopeTaskWrite},
		"PUT /api/v1/projects/:id/tasks/:task_id/design/sections/:section_id/full": {users.ScopeTaskWrite},
		"POST /api/v1/projects/:id/tasks/:task_id/design/sections":                 {users.ScopeTaskWrite},
		"DELETE /api/v1/projects/:id/tasks/:task_id/design/sections/:section_id":   {users.ScopeTaskWrite},
		"PATCH /api/v1/projects/:id/tasks/:task_id/design/sections/reorder":        {users.ScopeTaskWrite},
		"POST /api/v1/projects/:id/tasks/:task_id/design/sections/sync":            {users.ScopeTaskWrite},
		// Test sections
		"GET /api/v1/projects/:id/tasks/:task_id/test/sections":                  {users.ScopeTaskRead},
		"GET /api/v1/projects/:id/tasks/:task_id/test/sections/:section_id":      {users.ScopeTaskRead},
		"PUT /api/v1/projects/:id/tasks/:task_id/test/sections/:section_id":      {users.ScopeTaskWrite},
		"PUT /api/v1/projects/:id/tasks/:task_id/test/sections/:section_id/full": {users.ScopeTaskWrite},
		"POST /api/v1/projects/:id/tasks/:task_id/test/sections":                 {users.ScopeTaskWrite},
		"DELETE /api/v1/projects/:id/tasks/:task_id/test/sections/:section_id":   {users.ScopeTaskWrite},
		"PATCH /api/v1/projects/:id/tasks/:task_id/test/sections/reorder":        {users.ScopeTaskWrite},
		"POST /api/v1/projects/:id/tasks/:task_id/test/sections/sync":            {users.ScopeTaskWrite},
		"GET /api/v1/projects/:id/tasks/:task_id/prompts":                        {users.ScopeTaskRead}, "POST /api/v1/projects/:id/tasks/:task_id/prompts": {users.ScopeTaskWrite},
		// Prompts Management API - 使用 meeting.read 作为基础权限（更宽松）
		"POST /api/v1/prompts":                           {users.ScopeMeetingWrite},
		"GET /api/v1/prompts":                            {users.ScopeMeetingRead},
		"GET /api/v1/prompts/:prompt_id":                 {users.ScopeMeetingRead},
		"PUT /api/v1/prompts/:prompt_id":                 {users.ScopeMeetingWrite},
		"DELETE /api/v1/prompts/:prompt_id":              {users.ScopeMeetingWrite},
		"POST /api/v1/projects/:id/prompts":              {users.ScopeMeetingWrite},
		"GET /api/v1/projects/:id/prompts":               {users.ScopeMeetingRead},
		"GET /api/v1/projects/:id/prompts/:prompt_id":    {users.ScopeMeetingRead},
		"PUT /api/v1/projects/:id/prompts/:prompt_id":    {users.ScopeMeetingWrite},
		"DELETE /api/v1/projects/:id/prompts/:prompt_id": {users.ScopeMeetingWrite},
		"GET /api/v1/user/current-task":                  {users.ScopeTaskRead},
		"PUT /api/v1/user/current-task":                  {users.ScopeTaskWrite},
		"POST /api/v1/user/resources/refresh":            {users.ScopeMeetingRead}, // Manual refresh MCP resources, only requires basic access
		"GET /api/v1/tasks":                              {users.ScopeMeetingRead}, "POST /api/v1/tasks": {users.ScopeMeetingWrite},
		"GET /api/v1/tasks/:id": {users.ScopeMeetingRead}, "DELETE /api/v1/tasks/:id": {users.ScopeMeetingWrite},
		"POST /api/v1/tasks/:id/start": {users.ScopeMeetingWrite}, "POST /api/v1/tasks/:id/stop": {users.ScopeMeetingWrite},
		"POST /api/v1/tasks/:id/reprocess": {users.ScopeMeetingWrite}, "GET /api/v1/tasks/:id/reprocess": {users.ScopeMeetingWrite},
		"POST /api/v1/tasks/:id/resume": {users.ScopeMeetingWrite}, "POST /api/v1/tasks/:id/merge_only": {users.ScopeMeetingWrite},
		"POST /api/v1/tasks/:id/regenerate_merged": {users.ScopeMeetingWrite}, "POST /api/v1/tasks/:id/generate_polish": {users.ScopeMeetingWrite},
		"PATCH /api/v1/tasks/:id/config": {users.ScopeMeetingWrite}, "GET /api/v1/tasks/:id/config": {users.ScopeMeetingRead},
		"GET /api/v1/tasks/:id/status": {users.ScopeMeetingRead}, "GET /api/v1/tasks/:id/chunks": {users.ScopeMeetingRead},
		"GET /api/v1/tasks/:id/files": {users.ScopeMeetingRead}, "GET /api/v1/tasks/:id/files/*": {users.ScopeMeetingRead},
		"POST /api/v1/tasks/:id/chunks/:cid/merge": {users.ScopeMeetingWrite}, "GET /api/v1/tasks/:id/chunks/:cid/debug": {users.ScopeMeetingRead},
		"POST /api/v1/tasks/:id/chunks/:cid/redo/speakers": {users.ScopeMeetingWrite}, "POST /api/v1/tasks/:id/chunks/:cid/redo/embeddings": {users.ScopeMeetingWrite},
		"POST /api/v1/tasks/:id/chunks/:cid/redo/mapped": {users.ScopeMeetingWrite}, "GET /api/v1/tasks/:id/chunks/:cid/:kind": {users.ScopeMeetingRead},
		"PUT /api/v1/tasks/:id/chunks/:cid/segments": {users.ScopeMeetingWrite}, "POST /api/v1/tasks/:id/chunks/:cid/asr_once": {users.ScopeMeetingWrite},
		"GET /api/v1/tasks/:id/merged": {users.ScopeMeetingRead}, "GET /api/v1/tasks/:id/merged_all": {users.ScopeMeetingRead},
		"PUT /api/v1/tasks/:id/merged_all": {users.ScopeMeetingWrite},
		"GET /api/v1/tasks/:id/polish":     {users.ScopeMeetingRead}, "PUT /api/v1/tasks/:id/polish": {users.ScopeMeetingWrite},
		// 会议文档（保留 meeting.* 权限，因为这些是会议记录的一部分）
		"GET /api/v1/tasks/:id/feature-list": {users.ScopeMeetingRead}, "PUT /api/v1/tasks/:id/feature-list": {users.ScopeMeetingWrite},
		"GET /api/v1/tasks/:id/architecture": {users.ScopeMeetingRead}, "PUT /api/v1/tasks/:id/architecture": {users.ScopeMeetingWrite},
		"GET /api/v1/tasks/:id/tech-design": {users.ScopeMeetingRead}, "PUT /api/v1/tasks/:id/tech-design": {users.ScopeMeetingWrite},
		"GET /api/v1/tasks/:id/meeting-summary": {users.ScopeMeetingRead}, "PUT /api/v1/tasks/:id/meeting-summary": {users.ScopeMeetingWrite},
		"GET /api/v1/tasks/:id/meeting-context": {users.ScopeMeetingRead}, "PUT /api/v1/tasks/:id/meeting-context": {users.ScopeMeetingWrite},
		"GET /api/v1/tasks/:id/topic": {users.ScopeMeetingRead}, "PUT /api/v1/tasks/:id/topic": {users.ScopeMeetingWrite},
		"GET /api/v1/tasks/:id/polish-annotations": {users.ScopeMeetingRead}, "PUT /api/v1/tasks/:id/polish-annotations": {users.ScopeMeetingWrite},
		"GET /api/v1/tasks/:id/audio":    {users.ScopeMeetingRead},
		"PATCH /api/v1/tasks/:id/rename": {users.ScopeMeetingWrite},
		// Audio upload routes - 浏览器录音上传
		"POST /api/v1/meetings/:meeting_id/audio/upload":                                          {users.ScopeMeetingWrite},
		"POST /api/v1/meetings/:meeting_id/audio/upload-file":                                     {users.ScopeMeetingWrite},
		"GET /api/v1/devices/avfoundation":                                                        {users.ScopeMeetingRead},
		"GET /internal/api/v1/projects/:project_id/tasks/:task_id/execution-plan":                 {users.ScopeTaskRead},
		"POST /internal/api/v1/projects/:project_id/tasks/:task_id/execution-plan":                {users.ScopeTaskWrite},
		"PUT /internal/api/v1/projects/:project_id/tasks/:task_id/execution-plan":                 {users.ScopeTaskWrite},
		"POST /internal/api/v1/projects/:project_id/tasks/:task_id/execution-plan/steps/:step_id": {users.ScopeTaskWrite},
		"PUT /internal/api/v1/projects/:project_id/tasks/:task_id/execution-plan/steps/:step_id":  {users.ScopeTaskWrite},
		"GET /internal/api/v1/projects/:project_id/tasks/:task_id/execution-plan/next-step":       {users.ScopeTaskWrite},
		"GET /api/v1/projects/:id/tasks/:task_id/execution-plan":                                  {users.ScopeTaskRead},
		"PUT /api/v1/projects/:id/tasks/:task_id/execution-plan":                                  {users.ScopeTaskWrite},
		"POST /api/v1/projects/:id/tasks/:task_id/execution-plan/submit":                          {users.ScopeTaskWrite},
		"POST /api/v1/projects/:id/tasks/:task_id/execution-plan/approve":                         {users.ScopeTaskWrite},
		"POST /api/v1/projects/:id/tasks/:task_id/execution-plan/reject":                          {users.ScopeTaskWrite},
		"GET /api/v1/sync/prepare":                                                                {users.ScopeUserManage}, "POST /api/v1/sync": {users.ScopeUserManage},
		"POST /api/v1/admin/reload": {users.ScopeUserManage},
		// 文档管理API - 使用 project.doc.* 权限
		"POST /api/v1/projects/:id/documents/nodes": {users.ScopeProjectDocWrite}, "GET /api/v1/projects/:id/documents/tree": {users.ScopeProjectDocRead},
		"PUT /api/v1/projects/:id/documents/nodes/:node_id/move": {users.ScopeProjectDocWrite}, "PATCH /api/v1/projects/:id/documents/nodes/:node_id": {users.ScopeProjectDocWrite},
		"DELETE /api/v1/projects/:id/documents/nodes/:node_id": {users.ScopeProjectDocWrite},
		"POST /api/v1/projects/:id/documents/relationships":    {users.ScopeProjectDocWrite}, "GET /api/v1/projects/:id/documents/relationships": {users.ScopeProjectDocRead},
		"DELETE /api/v1/projects/:id/documents/relationships/:from_id/:to_id": {users.ScopeProjectDocWrite},
		"POST /api/v1/projects/:id/documents/references":                      {users.ScopeProjectDocWrite},
		"DELETE /api/v1/projects/:id/documents/references/:ref_id":            {users.ScopeProjectDocWrite},
		"GET /api/v1/projects/:id/tasks/:task_id/references":                  {users.ScopeProjectDocRead}, "GET /api/v1/projects/:id/documents/:doc_id/references": {users.ScopeProjectDocRead},
		"PUT /api/v1/projects/:id/references/:id/status":     {users.ScopeProjectDocWrite},
		"PUT /api/v1/projects/:id/documents/:doc_id/content": {users.ScopeProjectDocWrite}, "GET /api/v1/projects/:id/documents/:doc_id/content": {users.ScopeProjectDocRead},
		"GET /api/v1/projects/:id/documents/:doc_id/versions": {users.ScopeProjectDocRead}, "GET /api/v1/projects/:id/documents/:doc_id/versions/:version": {users.ScopeProjectDocRead},
		"GET /api/v1/projects/:id/documents/:doc_id/diff": {users.ScopeProjectDocRead}, "GET /api/v1/projects/:id/documents/:doc_id/impact": {users.ScopeProjectDocRead},
		"POST /api/v1/projects/:id/documents/search": {users.ScopeProjectDocRead}, "GET /api/v1/projects/:id/documents/search/suggestions": {users.ScopeProjectDocRead},
		// 项目状态页API - 使用 project.doc.* 权限
		"GET /api/v1/projects/:id/roadmap":                                 {users.ScopeProjectDocRead},
		"POST /api/v1/projects/:id/roadmap/nodes":                          {users.ScopeProjectDocWrite},
		"PUT /api/v1/projects/:id/roadmap/nodes/:node_id":                  {users.ScopeProjectDocWrite},
		"DELETE /api/v1/projects/:id/roadmap/nodes/:node_id":               {users.ScopeProjectDocWrite},
		"GET /api/v1/projects/:id/overview":                                {users.ScopeProjectDocRead},
		"PATCH /api/v1/projects/:id/metadata":                              {users.ScopeProjectDocWrite},
		"GET /api/v1/projects/:id/tasks/statistics":                        {users.ScopeTaskRead},
		"GET /api/v1/projects/:id/progress/week/:week_number":              {users.ScopeProjectDocRead},
		"PUT /api/v1/projects/:id/progress/week/:week_number":              {users.ScopeProjectDocWrite},
		"GET /api/v1/projects/:id/progress/year/:year":                     {users.ScopeProjectDocRead},
		"GET /api/v1/projects/:id/tasks/:task_id/summaries":                {users.ScopeTaskRead},
		"POST /api/v1/projects/:id/tasks/:task_id/summaries":               {users.ScopeTaskWrite},
		"PUT /api/v1/projects/:id/tasks/:task_id/summaries/:summary_id":    {users.ScopeTaskWrite},
		"DELETE /api/v1/projects/:id/tasks/:task_id/summaries/:summary_id": {users.ScopeTaskWrite},
		"GET /api/v1/projects/:id/summaries/by-week":                       {users.ScopeTaskRead},
		// Tag版本管理API - 文档标签
		"POST /api/v1/projects/:id/tasks/:task_id/docs/:docType/tags":                 {users.ScopeTaskWrite},
		"GET /api/v1/projects/:id/tasks/:task_id/docs/:docType/tags":                  {users.ScopeTaskRead},
		"POST /api/v1/projects/:id/tasks/:task_id/docs/:docType/tags/:tagName/switch": {users.ScopeTaskWrite},
		"GET /api/v1/projects/:id/tasks/:task_id/docs/:docType/tags/:tagName":         {users.ScopeTaskRead},
		"DELETE /api/v1/projects/:id/tasks/:task_id/docs/:docType/tags/:tagName":      {users.ScopeTaskWrite},
		// Tag版本管理API - 执行计划标签
		"POST /api/v1/projects/:id/tasks/:task_id/execution-plan/tags":                 {users.ScopeTaskWrite},
		"GET /api/v1/projects/:id/tasks/:task_id/execution-plan/tags":                  {users.ScopeTaskRead},
		"POST /api/v1/projects/:id/tasks/:task_id/execution-plan/tags/:tagName/switch": {users.ScopeTaskWrite},
		"GET /api/v1/projects/:id/tasks/:task_id/execution-plan/tags/:tagName":         {users.ScopeTaskRead},
		"DELETE /api/v1/projects/:id/tasks/:task_id/execution-plan/tags/:tagName":      {users.ScopeTaskWrite},
	}

	matchRouteKey := func(method, path string) (scopes []string, ok bool) {
		for k, sc := range routeScopes {
			parts := strings.SplitN(k, " ", 2)
			if len(parts) != 2 || parts[0] != method {
				continue
			}
			pattern := parts[1]
			if pattern == path {
				return sc, true
			}
			rx := regexp.MustCompile(`:[^/]+`)
			reg := rx.ReplaceAllString(pattern, `[^/]+`)
			reg = strings.ReplaceAll(reg, `*`, `.*`)
			reg = "^" + reg + "$"
			if ok, _ := regexp.MatchString(reg, path); ok {
				return sc, true
			}
		}
		return nil, false
	}

	r.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		// 跳过不需要认证的路由：登录、健康检查、服务状态、OPTIONS请求、非API路由、公开身份源列表、OIDC回调
		if path == "/api/v1/login" ||
			path == "/api/v1/health" ||
			path == "/api/v1/services/status" ||
			path == "/api/v1/identity-providers/public" ||
			strings.HasPrefix(path, "/auth/") ||
			c.Request.Method == http.MethodOptions ||
			!strings.HasPrefix(path, "/api/") {
			c.Next()
			return
		}
		auth := c.GetHeader("Authorization")
		if len(auth) < 8 || !strings.HasPrefix(auth, "Bearer ") {
			authPreview := auth
			if len(authPreview) > 20 {
				authPreview = authPreview[:20] + "..."
			}
			authLogger.Warn("missing bearer token",
				"method", c.Request.Method,
				"path", path,
				"auth_preview", authPreview,
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		claims, err := userManager.ParseToken(auth[7:])
		if err != nil {
			authLogger.Warn("invalid token",
				"method", c.Request.Method,
				"path", path,
				"error", err,
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("user", claims.Username)

		// 权限计算：区分全局权限和项目权限
		// 全局权限白名单：只有这些权限可以跨项目使用
		globalScopesWhitelist := map[string]bool{
			"user.manage":       true, // 用户管理
			"meeting.read":      true, // 会议记录读取
			"meeting.write":     true, // 会议记录写入
			"project.doc.read":  true, // 项目文档读取（允许查看项目列表）
			"project.doc.write": true, // 项目文档写入（全局编辑）
			"project.admin":     true, // 项目全局管理（创建/全局配置）
			"project.delete":    true, // 项目删除（允许删除项目和任务）
			"feature.read":      true, // 旧版特性读取（向后兼容）
			"feature.write":     true, // 旧版特性写入（向后兼容）
		}

		// 提取 project_id, task_id, meeting_id (支持多种路由模式)
		// 注意：路由定义使用 :id 作为项目参数，:task_id 作为任务参数

		// 判断是会议任务还是项目任务
		isMeetingTask := false
		if len(path) > 15 && path[:15] == "/api/v1/tasks/" {
			isMeetingTask = true
		}

		projectID := ""
		if !isMeetingTask {
			// 只有非会议任务路由才提取项目ID
			projectID = c.Param("id")
			if projectID == "" {
				projectID = c.Param("project_id") // 向后兼容
			}
		} else {
			// 会议任务路由从其他参数提取项目ID（如果需要）
			projectID = c.Param("project_id")
		}

		// 对于某些路径，从请求体中解析 project_id
		// 例如：PUT /api/v1/user/current-task
		if projectID == "" && c.Request.Method == "PUT" && path == "/api/v1/user/current-task" {
			// 读取请求体以获取 project_id
			var bodyData map[string]interface{}
			if c.Request.Body != nil {
				bodyBytes, _ := io.ReadAll(c.Request.Body)
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // 恢复请求体
				if len(bodyBytes) > 0 {
					json.Unmarshal(bodyBytes, &bodyData)
					if pid, ok := bodyData["project_id"].(string); ok {
						projectID = pid
					}
				}
			}
		}

		taskID := c.Param("task_id")
		meetingID := c.Param("meeting_id")

		var effectiveScopes []string

		// 1. 添加允许的全局权限
		for _, scope := range claims.Scopes {
			if globalScopesWhitelist[scope] {
				effectiveScopes = append(effectiveScopes, scope)
			}
		}

		// 2. 对于项目相关的API，添加项目角色权限
		if projectID != "" {
			projectScopes, err := userRoleService.ComputeEffectiveScopes(claims.Username, projectID)
			if err != nil {
				authLogger.Warn("failed to compute project scopes",
					"user", claims.Username,
					"project_id", projectID,
					"error", err,
				)
			} else {
				// 合并项目权限，去重
				for _, scope := range projectScopes {
					found := false
					for _, existing := range effectiveScopes {
						if existing == scope {
							found = true
							break
						}
					}
					if !found {
						effectiveScopes = append(effectiveScopes, scope)
					}
				}
				authLogger.Info("project scopes resolved",
					"user", claims.Username,
					"project_id", projectID,
					"scopes", projectScopes,
				)
			}
		} else {
			// 3. 对于非项目特定的API，需要特殊处理
			// 检查用户是否在任何项目中拥有相关权限
			path := c.Request.URL.Path
			if path == "/api/v1/projects" {
				// 项目列表API：如果用户在任何项目中有 project.doc.read 权限，则允许访问
				// 同时也检查旧的 feature.read 权限以保持向后兼容
				if hasAnyProjectPermission(userRoleService, claims.Username, "project.doc.read") ||
					hasAnyProjectPermission(userRoleService, claims.Username, "feature.read") {
					effectiveScopes = append(effectiveScopes, "project.doc.read")
					effectiveScopes = append(effectiveScopes, "feature.read") // 向后兼容
				}
			} else if path == "/api/v1/user/current-task" {
				// 当前任务API：根据HTTP方法注入相应权限
				// GET 需要 task.read，PUT 需要 task.write
				method := c.Request.Method
				if method == "GET" && hasAnyProjectPermission(userRoleService, claims.Username, "task.read") {
					effectiveScopes = append(effectiveScopes, "task.read")
				}
				if method == "PUT" && hasAnyProjectPermission(userRoleService, claims.Username, "task.write") {
					effectiveScopes = append(effectiveScopes, "task.write")
				}
			}
		}

		// 3. 任务负责人权限注入
		if projectID != "" && taskID != "" {
			injectedScopes, err := injector.InjectTaskOwnerPermissions(claims.Username, projectID, taskID, effectiveScopes)
			if err != nil {
				authLogger.Warn("failed to inject task owner permissions",
					"user", claims.Username,
					"project_id", projectID,
					"task_id", taskID,
					"error", err,
				)
			} else {
				effectiveScopes = injectedScopes
			}
		}

		// 4. 会议创建者权限注入
		if meetingID != "" {
			injectedScopes, err := injector.InjectMeetingOwnerPermissions(claims.Username, meetingID, effectiveScopes)
			if err != nil {
				authLogger.Warn("failed to inject meeting owner permissions",
					"user", claims.Username,
					"meeting_id", meetingID,
					"error", err,
				)
			} else {
				effectiveScopes = injectedScopes
			}

			// 额外检查会议ACL授权
			hasRead, hasWrite, err := injector.CheckMeetingACL(claims.Username, meetingID)
			if err != nil {
				authLogger.Warn("failed to check meeting ACL",
					"user", claims.Username,
					"meeting_id", meetingID,
					"error", err,
				)
			} else {
				if hasRead && !users.HasScope(effectiveScopes, "meeting.read") {
					effectiveScopes = append(effectiveScopes, "meeting.read")
				}
				if hasWrite && !users.HasScope(effectiveScopes, "meeting.write") {
					effectiveScopes = append(effectiveScopes, "meeting.write")
				}
			}
		}

		c.Set("scopes", effectiveScopes)
		full := c.FullPath()
		if full == "" {
			full = path
		}
		authLogger.Info("checking permissions",
			"method", c.Request.Method,
			"path", path,
			"full_path", full,
		)
		if scs, ok := matchRouteKey(c.Request.Method, full); ok && len(scs) > 0 {
			authLogger.Info("route matched",
				"required_scopes", scs,
				"user_scopes", effectiveScopes,
			)
			allowed := false
			for _, need := range scs {
				if users.HasScope(effectiveScopes, need) {
					allowed = true
					break
				}
			}
			if !allowed {
				authLogger.Warn("permission denied",
					"method", c.Request.Method,
					"path", path,
					"full_path", full,
					"required_scopes", scs,
					"user_scopes", effectiveScopes,
				)
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		} else {
			authLogger.Debug("no route match or no scopes required")
		}
		c.Next()
	})
}

func setupRoutes(r *gin.Engine, meetingsReg *meetings.Registry, projectsReg *projects.ProjectRegistry, docHandler *documents.Handler, taskDocSvc *taskdocs.DocService, userManager *users.Manager, roadmapService *services.RoadmapService, projectOverviewService services.ProjectOverviewService, statisticsService services.StatisticsService, progressService services.ProgressService, taskSummaryService services.TaskSummaryService, roleManager services.RoleManager, userRoleService services.UserRoleService, permissionInjector services.PermissionInjector, envHandler *handlers.EnvironmentHandler, projectsRoot string, resourceManager *resource.ResourceManager, promptsHandler *api.PromptsHandler, tagHandler *api.TagHandler, idpHandler *api.IdPHandler) {
	// ========== Environment Check ==========
	r.GET("/api/v1/environment/status", func(c *gin.Context) {
		envHandler.GetStatus(c.Writer, c.Request)
	})

	// ========== Authentication & Admin ==========
	// Login (supports both local and IdP authentication)
	r.POST("/api/v1/login", func(c *gin.Context) {
		var cred struct {
			Username string `json:"username"`
			Password string `json:"password"`
			IdpID    string `json:"idp_id,omitempty"` // Optional: for IdP-based login
		}
		if err := c.ShouldBindJSON(&cred); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		// Check if IdP login is requested
		if cred.IdpID != "" && idpHandler != nil {
			idpHandler.HandleLDAPLogin(c, cred.IdpID, cred.Username, cred.Password)
			return
		}

		// Local authentication
		u, err := userManager.Authenticate(cred.Username, cred.Password)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		token, _ := userManager.GenerateToken(u.Username)
		c.JSON(http.StatusOK, gin.H{"token": token, "username": u.Username, "scopes": u.Scopes})
	})

	// ========== Identity Provider Routes ==========
	if idpHandler != nil {
		// Public routes (no authentication required)
		r.GET("/api/v1/identity-providers/public", idpHandler.HandleListPublicIdPs)

		// OIDC authentication routes (no authentication required)
		r.GET("/auth/oidc/:idp_id/login", idpHandler.HandleOIDCLogin)
		r.GET("/auth/callback", idpHandler.HandleOIDCCallback)

		// Identity provider management routes (require idp.read/idp.write)
		r.GET("/api/v1/identity-providers", idpHandler.HandleListIdPs)
		r.GET("/api/v1/identity-providers/:id", idpHandler.HandleGetIdP)
		r.POST("/api/v1/identity-providers", idpHandler.HandleCreateIdP)
		r.PUT("/api/v1/identity-providers/:id", idpHandler.HandleUpdateIdP)
		r.DELETE("/api/v1/identity-providers/:id", idpHandler.HandleDeleteIdP)
		r.POST("/api/v1/identity-providers/test", idpHandler.HandleTestConnection)

		// Sync routes (require idp.read/idp.write)
		r.POST("/api/v1/identity-providers/:id/sync", idpHandler.HandleTriggerSync)
		r.GET("/api/v1/identity-providers/:id/sync/status", idpHandler.HandleGetSyncStatus)
		r.GET("/api/v1/identity-providers/:id/sync/logs", idpHandler.HandleGetSyncLogs)
	}

	// Admin reload
	r.POST("/api/v1/admin/reload", func(c *gin.Context) {
		// Clear and reload registries
		newMeetingsReg := meetings.NewRegistry()
		meetings.LoadTasks(newMeetingsReg)
		newProjectsReg := projects.NewProjectRegistry()
		projects.LoadProjects(newProjectsReg)
		c.JSON(http.StatusOK, gin.H{"reloaded": true, "tasks": len(newMeetingsReg.List()), "projects": len(newProjectsReg.List())})
	})

	// ========== User Management ==========
	// Me endpoint - Generate new token for current user
	r.GET("/api/v1/me/token", api.HandleGetMyToken(userManager))

	// User management
	r.GET("/api/v1/users", api.HandleListUsers(userManager))
	r.GET("/api/v1/users/:username", api.HandleGetUser(userManager))
	r.POST("/api/v1/users", api.HandleCreateUser(userManager))
	r.PATCH("/api/v1/users/:username", api.HandleUpdateUser(userManager))
	r.DELETE("/api/v1/users/:username", api.HandleDeleteUser(userManager))
	r.POST("/api/v1/users/:username/password", api.HandleChangePassword(userManager))

	// ========== Prompts Management (step-04) ==========
	r.POST("/api/v1/prompts", promptsHandler.CreatePrompt)
	r.GET("/api/v1/prompts", promptsHandler.ListPrompts)
	r.GET("/api/v1/prompts/:prompt_id", promptsHandler.GetPrompt)
	r.PUT("/api/v1/prompts/:prompt_id", promptsHandler.UpdatePrompt)
	r.DELETE("/api/v1/prompts/:prompt_id", promptsHandler.DeletePrompt)
	// Project-scoped prompts routes
	r.POST("/api/v1/projects/:id/prompts", promptsHandler.CreatePrompt)
	r.GET("/api/v1/projects/:id/prompts", promptsHandler.ListPrompts)
	r.GET("/api/v1/projects/:id/prompts/:prompt_id", promptsHandler.GetPrompt)
	r.PUT("/api/v1/projects/:id/prompts/:prompt_id", promptsHandler.UpdatePrompt)
	r.DELETE("/api/v1/projects/:id/prompts/:prompt_id", promptsHandler.DeletePrompt)

	// ========== User Task Management ==========
	r.GET("/api/v1/user/current-task", api.HandleGetUserCurrentTask)
	r.PUT("/api/v1/user/current-task", api.HandlePutUserCurrentTask(userRoleService, resourceManager, docHandler))

	// ========== Resource Management API ==========
	resourceHandler := api.NewResourceHandler(resourceManager)
	r.GET("/api/v1/users/:username/resources", resourceHandler.GetUserResources)
	r.POST("/api/v1/users/:username/resources", resourceHandler.AddCustomResource)
	r.PUT("/api/v1/users/:username/resources/:resource_id", resourceHandler.UpdateResource)
	r.DELETE("/api/v1/users/:username/resources/:resource_id", resourceHandler.DeleteResource)
	r.GET("/api/v1/users/:username/resources/:resource_id", resourceHandler.GetResourceByID)

	// Manual refresh resources based on current task
	r.POST("/api/v1/user/resources/refresh", api.HandleRefreshUserResources(resourceManager, docHandler))

	// ========== Role Management ==========
	// Role CRUD (query parameter style)
	r.POST("/api/v1/roles", api.HandleCreateRole(roleManager))
	r.GET("/api/v1/roles", api.HandleListRoles(roleManager))
	r.GET("/api/v1/roles/:role_id", api.HandleGetRole(roleManager))
	r.PUT("/api/v1/roles/:role_id", api.HandleUpdateRole(roleManager))
	r.DELETE("/api/v1/roles/:role_id", api.HandleDeleteRole(roleManager))

	// Role CRUD (RESTful style)
	r.GET("/api/v1/projects/:id/roles", api.HandleListProjectRoles(roleManager))
	r.GET("/api/v1/projects/:id/roles/:role_id", api.HandleGetProjectRole(roleManager))
	r.POST("/api/v1/projects/:id/roles", api.HandleCreateProjectRole(roleManager))
	r.PUT("/api/v1/projects/:id/roles/:role_id", api.HandleUpdateProjectRole(roleManager))
	r.DELETE("/api/v1/projects/:id/roles/:role_id", api.HandleDeleteProjectRole(roleManager))

	// User Role Assignment (query parameter style)
	r.POST("/api/v1/users/roles", api.HandleAssignRoles(userRoleService))
	r.DELETE("/api/v1/users/roles", api.HandleRevokeRoles(userRoleService))
	r.GET("/api/v1/users/:username/permissions", api.HandleGetUserPermissions(userRoleService))
	r.GET("/api/v1/users/:username/profile", api.HandleGetUserProfile(userRoleService))
	// 当前用户档案 - 包含项目角色和基础权限
	r.GET("/api/v1/user/profile", api.HandleGetCurrentUserProfile(userRoleService, userManager))

	// User Role Assignment (RESTful style)
	r.GET("/api/v1/projects/:id/users/:username/roles", api.HandleGetProjectUserRoles(userRoleService))
	r.POST("/api/v1/projects/:id/users/:username/roles", api.HandleAssignProjectUserRole(userRoleService))
	r.DELETE("/api/v1/projects/:id/users/:username/roles/:role_id", api.HandleRemoveProjectUserRole(userRoleService))
	r.GET("/api/v1/projects/:id/user-roles", api.HandleGetProjectUserRolesList(userRoleService))

	// ========== Meetings API (29 endpoints) ==========
	r.GET("/api/v1/tasks", api.HandleListTasks(meetingsReg))
	r.GET("/api/v1/tasks/:id", api.HandleGetTask(meetingsReg))
	r.POST("/api/v1/tasks", api.HandleCreateTask(meetingsReg))
	r.DELETE("/api/v1/tasks/:id", api.HandleDeleteTask(meetingsReg))
	r.PATCH("/api/v1/tasks/:id/rename", api.HandleRenameTask(meetingsReg))
	r.POST("/api/v1/tasks/:id/start", api.HandleStartTask(meetingsReg))
	r.POST("/api/v1/tasks/:id/stop", api.HandleStopTask(meetingsReg))
	r.POST("/api/v1/tasks/:id/reprocess", api.HandleReprocessTask(meetingsReg))
	r.GET("/api/v1/tasks/:id/reprocess", api.HandleReprocessTask(meetingsReg)) // alias
	r.POST("/api/v1/tasks/:id/resume", api.HandleResumeTask(meetingsReg))
	r.POST("/api/v1/tasks/:id/merge_only", api.HandleMergeOnlyTask(meetingsReg))
	r.POST("/api/v1/tasks/:id/regenerate_merged", api.HandleRegenerateMerged(meetingsReg))
	// r.POST("/api/v1/tasks/:id/generate_polish", api.HandleGeneratePolish(meetingsReg)) // Removed: function not found
	r.POST("/api/v1/tasks/:id/chunks/:cid/merge", api.HandleMergeChunk(meetingsReg))
	r.GET("/api/v1/tasks/:id/chunks/:cid/debug", api.HandleChunkDebug(meetingsReg))
	r.POST("/api/v1/tasks/:id/chunks/:cid/redo/speakers", api.HandleRedoSpeakers(meetingsReg))
	r.POST("/api/v1/tasks/:id/chunks/:cid/redo/embeddings", api.HandleRedoEmbeddings(meetingsReg))
	r.POST("/api/v1/tasks/:id/chunks/:cid/redo/mapped", api.HandleRedoMapped(meetingsReg))
	r.PATCH("/api/v1/tasks/:id/config", api.HandleUpdateTaskConfig(meetingsReg))
	r.GET("/api/v1/tasks/:id/config", api.HandleGetTaskConfig(meetingsReg))
	r.GET("/api/v1/tasks/:id/status", api.HandleGetTaskStatus(meetingsReg))
	r.GET("/api/v1/tasks/:id/chunks", api.HandleListChunks(meetingsReg))
	r.GET("/api/v1/tasks/:id/files", api.HandleListTaskFiles(meetingsReg))
	r.GET("/api/v1/tasks/:id/files/:filename", api.HandleGetTaskFile(meetingsReg))
	r.GET("/api/v1/tasks/:id/chunks/:cid/:kind", api.HandleGetChunkFile(meetingsReg))
	r.PUT("/api/v1/tasks/:id/chunks/:cid/segments", api.HandleUpdateChunkSegments(meetingsReg))
	r.POST("/api/v1/tasks/:id/chunks/:cid/asr_once", api.HandleASROnce(meetingsReg))
	r.GET("/api/v1/tasks/:id/merged", api.HandleGetMerged(meetingsReg))

	// ========== Whisper Service Health API ==========
	// 动态获取运行中的orchestrator实例
	r.GET("/api/v1/services/whisper/health", func(c *gin.Context) {
		// 从meetingsReg获取任意一个正在运行的task的orchestrator
		var activeOrch *orchestrator.Orchestrator
		for _, task := range meetingsReg.List() {
			if task.Orch != nil && task.State == orchestrator.StateRunning {
				activeOrch = task.Orch
				break
			}
		}

		// 如果没有运行中的task,返回未初始化错误
		if activeOrch == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"error":   "No active Whisper service found. Start a task first.",
			})
			return
		}

		// 调用health handler
		handler := api.HandleWhisperHealthCheck(
			activeOrch.GetDegradationController(),
			activeOrch.GetHealthChecker(),
		)
		handler(c)
	})

	r.GET("/api/v1/tasks/:id/merged_all", api.HandleGetMergedAll(meetingsReg))
	r.GET("/api/v1/tasks/:id/polish", api.HandleGetTaskPolish(meetingsReg))

	// ========== Meeting Task Document APIs for /tasks/:id (Legacy format) ==========
	// Register Legacy routes for /tasks/:id/ format
	r.GET("/api/v1/tasks/:id/meeting-summary", api.HandleGetTaskMeetingSummary(meetingsReg))
	r.PUT("/api/v1/tasks/:id/meeting-summary", api.HandleUpdateTaskMeetingSummary(meetingsReg))
	r.GET("/api/v1/tasks/:id/meeting-context", api.HandleGetTaskMeetingContext(meetingsReg))
	r.PUT("/api/v1/tasks/:id/meeting-context", api.HandleUpdateTaskMeetingContext(meetingsReg))
	r.GET("/api/v1/tasks/:id/topic", api.HandleGetTaskTopic(meetingsReg))
	r.PUT("/api/v1/tasks/:id/topic", api.HandleUpdateTaskTopic(meetingsReg))
	r.GET("/api/v1/tasks/:id/polish-annotations", api.HandleGetTaskPolishAnnotations(meetingsReg))
	r.PUT("/api/v1/tasks/:id/polish-annotations", api.HandleUpdateTaskPolishAnnotations(meetingsReg))
	r.GET("/api/v1/tasks/:id/feature-list", api.HandleGetTaskFeatureList(meetingsReg))
	r.PUT("/api/v1/tasks/:id/feature-list", api.HandleUpdateTaskFeatureList(meetingsReg))
	r.GET("/api/v1/tasks/:id/architecture-design", api.HandleGetTaskArchitecture(meetingsReg))
	r.PUT("/api/v1/tasks/:id/architecture-design", api.HandleUpdateTaskArchitecture(meetingsReg))
	r.GET("/api/v1/tasks/:id/tech-design", api.HandleGetTaskTechDesign(meetingsReg))
	r.PUT("/api/v1/tasks/:id/tech-design", api.HandleUpdateTaskTechDesign(meetingsReg))
	r.GET("/api/v1/tasks/:id/audio", api.HandleGetTaskAudio(meetingsReg))
	r.PUT("/api/v1/tasks/:id/polish", api.HandleUpdateTaskPolish(meetingsReg))
	r.PUT("/api/v1/tasks/:id/merged_all", api.HandleUpdateTaskMergedAll(meetingsReg))

	// Audio upload routes
	r.POST("/api/v1/meetings/:meeting_id/audio/upload", api.HandleAudioUpload(meetingsReg))
	r.POST("/api/v1/meetings/:meeting_id/audio/upload-file", api.HandleAudioFileUpload(meetingsReg))

	// Meeting document copy routes
	r.POST("/api/v1/tasks/:id/copy-feature-list", api.HandleCopyFeatureList(meetingsReg))
	r.POST("/api/v1/tasks/:id/copy-architecture-design", api.HandleCopyArchitectureDesign(meetingsReg))
	r.POST("/api/v1/tasks/:id/copy-tech-design", api.HandleCopyTechDesign(meetingsReg))

	// Meeting document history routes
	r.GET("/api/v1/tasks/:id/meeting-summary/history", api.HandleGetMeetingSummaryHistory(meetingsReg))
	r.DELETE("/api/v1/tasks/:id/meeting-summary/history/:version", api.HandleDeleteMeetingSummaryHistory(meetingsReg))
	r.GET("/api/v1/tasks/:id/topic/history", api.HandleGetTopicHistory(meetingsReg))
	r.DELETE("/api/v1/tasks/:id/topic/history/:version", api.HandleDeleteTopicHistory(meetingsReg))
	r.GET("/api/v1/tasks/:id/feature-list/history", api.HandleGetMeetingFeatureListHistory(meetingsReg))
	r.DELETE("/api/v1/tasks/:id/feature-list/history/:version", api.HandleDeleteMeetingFeatureListHistory(meetingsReg))
	r.GET("/api/v1/tasks/:id/architecture-design/history", api.HandleGetMeetingArchitectureHistory(meetingsReg))
	r.DELETE("/api/v1/tasks/:id/architecture-design/history/:version", api.HandleDeleteMeetingArchitectureHistory(meetingsReg))
	r.GET("/api/v1/tasks/:id/tech-design/history", api.HandleGetMeetingTechDesignHistory(meetingsReg))
	r.DELETE("/api/v1/tasks/:id/tech-design/history/:version", api.HandleDeleteMeetingTechDesignHistory(meetingsReg))
	r.GET("/api/v1/tasks/:id/polish/history", api.HandleGetPolishHistory(meetingsReg))
	r.DELETE("/api/v1/tasks/:id/polish/history/:version", api.HandleDeletePolishHistory(meetingsReg))

	// ========== Devices API (1 endpoint) ==========
	r.GET("/api/v1/devices/avfoundation", api.HandleGetAVFoundationDevices())

	// ========== Projects API (6 endpoints) ==========
	r.GET("/api/v1/projects", api.HandleListProjects(projectsReg))
	r.POST("/api/v1/projects", api.HandleCreateProject(projectsReg))
	r.GET("/api/v1/projects/:id", api.HandleGetProject(projectsReg))
	r.PATCH("/api/v1/projects/:id", api.HandlePatchProject(projectsReg))
	r.DELETE("/api/v1/projects/:id", api.HandleDeleteProject(projectsReg))

	// Get project tasks
	r.GET("/api/v1/projects/:id/tasks", api.HandleListProjectTasks(projectsReg))
	r.POST("/api/v1/projects/:id/tasks", api.HandleCreateProjectTask(projectsReg))
	r.GET("/api/v1/projects/:id/tasks/:task_id", api.HandleGetProjectTask(projectsReg))
	r.PUT("/api/v1/projects/:id/tasks/:task_id", api.HandleUpdateProjectTask(projectsReg))
	r.DELETE("/api/v1/projects/:id/tasks/:task_id", api.HandleDeleteProjectTask(projectsReg))

	// Task prompts
	r.GET("/api/v1/projects/:id/tasks/:task_id/prompts", api.HandleGetProjectTaskPrompts(projectsReg))
	r.POST("/api/v1/projects/:id/tasks/:task_id/prompts", api.HandleCreateProjectTaskPrompt(projectsReg))

	// ========== Project Deliverables ==========
	r.GET("/api/v1/projects/:id/feature-list", api.HandleGetFeatureList(projectsReg))
	r.GET("/api/v1/projects/:id/feature-list.json", api.HandleGetFeatureListJSON(projectsReg))
	r.PUT("/api/v1/projects/:id/feature-list.json", api.HandlePutFeatureListJSON(projectsReg))
	r.PUT("/api/v1/projects/:id/feature-list", api.HandlePutProjectFeatureList(projectsReg))
	r.GET("/api/v1/projects/:id/architecture-design", api.HandleGetArchitectureDesign(projectsReg))
	r.PUT("/api/v1/projects/:id/architecture-design", api.HandlePutProjectArchitectureDesign(projectsReg))
	r.GET("/api/v1/projects/:id/tech-design", api.HandleGetTechDesign(projectsReg))
	r.PUT("/api/v1/projects/:id/tech-design", api.HandlePutProjectTechDesign(projectsReg))

	// Legacy documents (for reference documents)
	r.GET("/api/v1/projects/:id/legacy-documents/:doc_id", api.HandleGetLegacyDocument(projectsReg))

	// Project document copy route
	r.POST("/api/v1/projects/:id/copy-from-task", api.HandleCopyFromTask(projectsReg, meetingsReg))

	// Project document history routes
	r.GET("/api/v1/projects/:id/feature-list/history", api.HandleGetProjectFeatureListHistory(projectsReg))
	r.DELETE("/api/v1/projects/:id/feature-list/history/:version", api.HandleDeleteProjectFeatureListHistory(projectsReg))
	r.GET("/api/v1/projects/:id/architecture-design/history", api.HandleGetProjectArchitectureHistory(projectsReg))
	r.DELETE("/api/v1/projects/:id/architecture-design/history/:version", api.HandleDeleteProjectArchitectureHistory(projectsReg))
	r.GET("/api/v1/projects/:id/tech-design/history", api.HandleGetProjectTechDesignHistory(projectsReg))
	r.DELETE("/api/v1/projects/:id/tech-design/history/:version", api.HandleDeleteProjectTechDesignHistory(projectsReg))

	// ========== Task Documents API (18 endpoints) ==========
	// Requirements (6 endpoints)
	r.POST("/api/v1/projects/:id/tasks/:task_id/requirements/append", api.HandleAppendTaskDoc(taskDocSvc, "requirements"))
	r.GET("/api/v1/projects/:id/tasks/:task_id/requirements/chunks", api.HandleListTaskDocChunks(taskDocSvc, "requirements"))
	r.DELETE("/api/v1/projects/:id/tasks/:task_id/requirements/chunks/:seq", api.HandleDeleteTaskDocChunk(taskDocSvc, "requirements"))
	r.PATCH("/api/v1/projects/:id/tasks/:task_id/requirements/chunks/:seq/toggle", api.HandleToggleTaskDocChunk(taskDocSvc, "requirements"))
	r.POST("/api/v1/projects/:id/tasks/:task_id/requirements/squash", api.HandleSquashTaskDoc(taskDocSvc, "requirements"))
	r.GET("/api/v1/projects/:id/tasks/:task_id/requirements/export", api.HandleExportTaskDoc(taskDocSvc, "requirements"))
	// Design (6 endpoints)
	r.POST("/api/v1/projects/:id/tasks/:task_id/design/append", api.HandleAppendTaskDoc(taskDocSvc, "design"))
	r.GET("/api/v1/projects/:id/tasks/:task_id/design/chunks", api.HandleListTaskDocChunks(taskDocSvc, "design"))
	r.DELETE("/api/v1/projects/:id/tasks/:task_id/design/chunks/:seq", api.HandleDeleteTaskDocChunk(taskDocSvc, "design"))
	r.PATCH("/api/v1/projects/:id/tasks/:task_id/design/chunks/:seq/toggle", api.HandleToggleTaskDocChunk(taskDocSvc, "design"))
	r.POST("/api/v1/projects/:id/tasks/:task_id/design/squash", api.HandleSquashTaskDoc(taskDocSvc, "design"))
	r.GET("/api/v1/projects/:id/tasks/:task_id/design/export", api.HandleExportTaskDoc(taskDocSvc, "design"))
	// Test (6 endpoints)
	r.POST("/api/v1/projects/:id/tasks/:task_id/test/append", api.HandleAppendTaskDoc(taskDocSvc, "test"))
	r.GET("/api/v1/projects/:id/tasks/:task_id/test/chunks", api.HandleListTaskDocChunks(taskDocSvc, "test"))
	r.DELETE("/api/v1/projects/:id/tasks/:task_id/test/chunks/:seq", api.HandleDeleteTaskDocChunk(taskDocSvc, "test"))
	r.PATCH("/api/v1/projects/:id/tasks/:task_id/test/chunks/:seq/toggle", api.HandleToggleTaskDocChunk(taskDocSvc, "test"))
	r.POST("/api/v1/projects/:id/tasks/:task_id/test/squash", api.HandleSquashTaskDoc(taskDocSvc, "test"))
	r.GET("/api/v1/projects/:id/tasks/:task_id/test/export", api.HandleExportTaskDoc(taskDocSvc, "test"))

	// ========== Tag Version Management API (10 endpoints) ==========
	// Document tags (5 endpoints for requirements/design/test)
	r.POST("/api/v1/projects/:id/tasks/:task_id/docs/:docType/tags", tagHandler.CreateTag)
	r.GET("/api/v1/projects/:id/tasks/:task_id/docs/:docType/tags", tagHandler.ListTags)
	r.POST("/api/v1/projects/:id/tasks/:task_id/docs/:docType/tags/:tagName/switch", tagHandler.SwitchTag)
	r.GET("/api/v1/projects/:id/tasks/:task_id/docs/:docType/tags/:tagName", tagHandler.GetTagInfo)
	r.DELETE("/api/v1/projects/:id/tasks/:task_id/docs/:docType/tags/:tagName", tagHandler.DeleteTag)
	// Execution plan tags (5 endpoints)
	r.POST("/api/v1/projects/:id/tasks/:task_id/execution-plan/tags", func(c *gin.Context) {
		c.Params = append(c.Params, gin.Param{Key: "docType", Value: "execution-plan"})
		tagHandler.CreateTag(c)
	})
	r.GET("/api/v1/projects/:id/tasks/:task_id/execution-plan/tags", func(c *gin.Context) {
		c.Params = append(c.Params, gin.Param{Key: "docType", Value: "execution-plan"})
		tagHandler.ListTags(c)
	})
	r.POST("/api/v1/projects/:id/tasks/:task_id/execution-plan/tags/:tagName/switch", func(c *gin.Context) {
		c.Params = append(c.Params, gin.Param{Key: "docType", Value: "execution-plan"})
		tagHandler.SwitchTag(c)
	})
	r.GET("/api/v1/projects/:id/tasks/:task_id/execution-plan/tags/:tagName", func(c *gin.Context) {
		c.Params = append(c.Params, gin.Param{Key: "docType", Value: "execution-plan"})
		tagHandler.GetTagInfo(c)
	})
	r.DELETE("/api/v1/projects/:id/tasks/:task_id/execution-plan/tags/:tagName", func(c *gin.Context) {
		c.Params = append(c.Params, gin.Param{Key: "docType", Value: "execution-plan"})
		tagHandler.DeleteTag(c)
	})

	// ========== Similarity Service (Semantic Recommendations) ==========
	// Initialize NLP client and similarity service before taskDocHandler
	nlpServiceURL := os.Getenv("NLP_SERVICE_URL")
	if nlpServiceURL == "" {
		nlpServiceURL = "http://localhost:5001" // Default to localhost:5001
	}
	nlpClient := similarity.NewNLPClient(nlpServiceURL, 5*time.Second)
	log.Printf("[INFO] NLP Service URL: %s", nlpServiceURL)

	// Create adapter instance (using sectionService defined below)
	var sectionAdapter *sectionServiceAdapter

	// Create a factory function to get or create similarity service for a project
	projectVectorManagers := make(map[string]*similarity.VectorIndexManager)
	var vectorManagerMutex sync.RWMutex

	getSimilarityService := func(projectID string) *similarity.SimilarityService {
		vectorManagerMutex.Lock()
		defer vectorManagerMutex.Unlock()

		if _, ok := projectVectorManagers[projectID]; !ok {
			// Pass data root directory, not projects root
			// VectorIndexManager will append "projects/{projectID}" internally
			dataRoot := "./data"
			projectVectorManagers[projectID] = similarity.NewVectorIndexManager(projectID, dataRoot)
			log.Printf("[INFO] Created VectorIndexManager for project: %s", projectID)
		}

		return similarity.NewSimilarityService(
			projectVectorManagers[projectID],
			nlpClient,
			sectionAdapter,
		)
	}

	// ========== Legacy Task Document Endpoints (Compatibility) ==========
	// These endpoints provide backward compatibility for the frontend
	// Task document handler (兼容 GET/PUT)
	taskDocHandler := func(docType string) gin.HandlerFunc {
		return func(c *gin.Context) {
			projectID := c.Param("id")
			taskID := c.Param("task_id")

			// Get project directory
			if projectsReg.Get(projectID) == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
				return
			}
			dir := filepath.Join(projectsRoot, projectID)
			taskDir := filepath.Join(dir, "tasks", taskID)
			legacyFile := filepath.Join(taskDir, fmt.Sprintf("%s.md", docType))

			// Helper: ensure migration if needed
			migrateIfNeeded := func() error {
				meta, mErr := taskdocs.LoadOrInitMeta(projectID, taskID, docType)
				if mErr != nil {
					return mErr
				}
				// Already initialized if Version>0
				if meta.Version > 0 {
					return nil
				}
				// Legacy file? -> import
				data, readErr := os.ReadFile(legacyFile)
				if readErr != nil {
					return nil // No legacy content
				}
				content := strings.TrimSpace(string(data))
				if content == "" {
					return nil
				}
				// Append as replace_full (initial migration)
				_, _, _, aErr := taskDocSvc.Append(projectID, taskID, docType, content, "migration", nil, "replace_full", "migration")
				return aErr
			}

			if c.Request.Method == http.MethodGet {
				_ = migrateIfNeeded()
				compiledPath, _ := taskdocs.DocCompiledPath(projectID, taskID, docType)
				b, _ := os.ReadFile(compiledPath)
				if len(b) == 0 { // Fallback to legacy (maybe empty after failed migration)
					if lb, err2 := os.ReadFile(legacyFile); err2 == nil {
						b = lb
					}
				}
				meta, _ := taskdocs.LoadOrInitMeta(projectID, taskID, docType)
				exists := len(b) > 0

				response := gin.H{
					"exists":  exists,
					"content": string(b),
					"version": meta.Version,
					"etag":    meta.ETag,
				}

				// ⭐ New: Optional recommendations feature
				includeRec := c.Query("include_recommendations") == "true"
				if includeRec && exists {
					simService := getSimilarityService(projectID)
					recs, err := simService.GetRecommendations(c.Request.Context(), projectID, taskID, docType, 5)
					if err != nil {
						log.Printf("[WARN] Failed to get recommendations for %s/%s/%s: %v", projectID, taskID, docType, err)
						// Recommendation failure doesn't affect main flow
					} else {
						response["recommendations"] = recs
					}
				}

				c.JSON(http.StatusOK, response)
				return
			}

			if c.Request.Method == http.MethodPut {
				var body struct {
					Content         string `json:"content"`
					ExpectedVersion *int   `json:"expected_version"`
				}
				if err := c.ShouldBindJSON(&body); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
					return
				}
				// 允许空内容（用户可能想清空文档）
				// Migrate legacy first (ensures directory)
				_ = os.MkdirAll(taskDir, 0755)
				// 如果传 expected_version 则严格检查；否则兼容旧 PUT 忽略版本
				meta, _, duplicate, aErr := taskDocSvc.Append(projectID, taskID, docType, body.Content, "put_api", body.ExpectedVersion, "replace_full", "put")
				if aErr != nil {
					if aErr.Error() == "version_mismatch" {
						c.JSON(http.StatusConflict, gin.H{"error": "version_mismatch"})
						return
					}
					c.JSON(http.StatusInternalServerError, gin.H{"error": aErr.Error()})
					return
				}

				// ⭐ Trigger async vectorization after successful save
				simService := getSimilarityService(projectID)
				if vErr := simService.VectorizeDocument(context.Background(), projectID, taskID, docType); vErr != nil {
					log.Printf("[WARN] Failed to trigger vectorization for %s/%s/%s: %v", projectID, taskID, docType, vErr)
					// Non-blocking: continue with success response even if vectorization fails
				}

				c.JSON(http.StatusOK, gin.H{"success": true, "version": meta.Version, "duplicate": duplicate, "etag": meta.ETag})
				return
			}

			c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
		}
	}

	r.GET("/api/v1/projects/:id/tasks/:task_id/requirements", taskDocHandler("requirements"))
	r.PUT("/api/v1/projects/:id/tasks/:task_id/requirements", taskDocHandler("requirements"))
	r.GET("/api/v1/projects/:id/tasks/:task_id/design", taskDocHandler("design"))
	r.PUT("/api/v1/projects/:id/tasks/:task_id/design", taskDocHandler("design"))
	r.GET("/api/v1/projects/:id/tasks/:task_id/test", taskDocHandler("test"))
	r.PUT("/api/v1/projects/:id/tasks/:task_id/test", taskDocHandler("test"))

	// ========== Task Document Sections API (21 endpoints) ==========
	// Section management for requirements, design, and test documents
	sectionService := taskdocs.NewSectionService(projectsRoot)

	// Initialize the adapter with sectionService
	sectionAdapter = &sectionServiceAdapter{svc: sectionService}

	// Requirements sections (7 endpoints)
	requirementsGroup := r.Group("/api/v1/projects/:id/tasks/:task_id/requirements")
	api.RegisterSectionRoutes(requirementsGroup, sectionService)

	// Design sections (7 endpoints)
	designGroup := r.Group("/api/v1/projects/:id/tasks/:task_id/design")
	api.RegisterSectionRoutes(designGroup, sectionService)

	// Test sections (7 endpoints)
	testGroup := r.Group("/api/v1/projects/:id/tasks/:task_id/test")
	api.RegisterSectionRoutes(testGroup, sectionService)

	// ========== Execution Plan ==========
	execPlanHandler := executionplan.NewHandler(projectsRoot)
	execPlanHandler.RegisterRoutes(r)

	// ========== Unified Document Service API (step-07) ==========
	// 统一项目、会议、任务三类文档的 API
	unifiedDocSvc := docslot.NewUnifiedDocService(projectsRoot)
	unifiedDocsGroup := r.Group("/api/v1")
	api.RegisterUnifiedDocRoutes(unifiedDocsGroup, unifiedDocSvc)

	// ========== Documents API (20 endpoints) ==========
	r.POST("/api/v1/projects/:id/documents/nodes", docHandler.CreateNode)
	r.GET("/api/v1/projects/:id/documents/tree", docHandler.GetTree)
	r.PUT("/api/v1/projects/:id/documents/nodes/:node_id/move", docHandler.MoveNode)
	r.PATCH("/api/v1/projects/:id/documents/nodes/:node_id", docHandler.UpdateNode)
	r.DELETE("/api/v1/projects/:id/documents/nodes/:node_id", docHandler.DeleteNode)
	r.POST("/api/v1/projects/:id/documents/relationships", docHandler.CreateRelationship)
	r.GET("/api/v1/projects/:id/documents/relationships", docHandler.GetRelationships)
	r.DELETE("/api/v1/projects/:id/documents/relationships/:from_id/:to_id", docHandler.RemoveRelationship)
	r.POST("/api/v1/projects/:id/documents/references", docHandler.CreateReference)
	r.DELETE("/api/v1/projects/:id/documents/references/:ref_id", docHandler.DeleteReferenceHandler)
	r.GET("/api/v1/projects/:id/tasks/:task_id/references", docHandler.GetTaskReferences)
	r.GET("/api/v1/projects/:id/documents/:doc_id/references", docHandler.GetDocumentReferences)
	r.PUT("/api/v1/projects/:id/references/:id/status", docHandler.UpdateReferenceStatus)
	r.PUT("/api/v1/projects/:id/documents/:doc_id/content", docHandler.UpdateDocumentContent)
	r.GET("/api/v1/projects/:id/documents/:doc_id/content", docHandler.GetDocumentContent)
	r.GET("/api/v1/projects/:id/documents/:doc_id/versions", docHandler.GetDocumentVersions)
	r.GET("/api/v1/projects/:id/documents/:doc_id/versions/:version", docHandler.GetDocumentVersion)
	r.GET("/api/v1/projects/:id/documents/:doc_id/diff", docHandler.CompareDocumentVersions)
	r.GET("/api/v1/projects/:id/documents/:doc_id/impact", docHandler.AnalyzeDocumentImpact)
	r.POST("/api/v1/projects/:id/documents/search", docHandler.SearchDocuments)
	r.GET("/api/v1/projects/:id/documents/search/suggestions", docHandler.GetSearchSuggestions)

	// ========== Project Status Page APIs (15 endpoints) ==========
	// Roadmap (4 endpoints)
	roadmapHandler := api.NewRoadmapHandler(roadmapService)
	r.GET("/api/v1/projects/:id/roadmap", roadmapHandler.HandleGetRoadmap)
	r.POST("/api/v1/projects/:id/roadmap/nodes", roadmapHandler.HandleAddNode)
	r.PUT("/api/v1/projects/:id/roadmap/nodes/:node_id", roadmapHandler.HandleUpdateNode)
	r.DELETE("/api/v1/projects/:id/roadmap/nodes/:node_id", roadmapHandler.HandleDeleteNode)

	// Project Overview & Statistics (3 endpoints)
	r.GET("/api/v1/projects/:id/overview", api.HandleGetProjectOverview(projectsReg, statisticsService))
	r.PATCH("/api/v1/projects/:id/metadata", api.HandleUpdateProjectMetadata(projectsReg, projectOverviewService))
	r.GET("/api/v1/projects/:id/tasks/statistics", api.HandleGetTaskStatistics(projectsReg, statisticsService))

	// Time Progress (3 endpoints)
	r.GET("/api/v1/projects/:id/progress/week/:week_number", api.HandleGetWeekProgress(projectsReg, progressService))
	r.PUT("/api/v1/projects/:id/progress/week/:week_number", api.HandleUpdateWeekProgress(projectsReg, progressService))
	r.GET("/api/v1/projects/:id/progress/year/:year", api.HandleGetYearProgress(projectsReg, progressService))

	// Task Summary (5 endpoints)
	r.GET("/api/v1/projects/:id/tasks/:task_id/summaries", api.HandleGetTaskSummaries(projectsReg, taskSummaryService))
	r.POST("/api/v1/projects/:id/tasks/:task_id/summaries", api.HandleAddTaskSummary(projectsReg, taskSummaryService))
	r.PUT("/api/v1/projects/:id/tasks/:task_id/summaries/:summary_id", api.HandleUpdateTaskSummary(projectsReg, taskSummaryService))
	r.DELETE("/api/v1/projects/:id/tasks/:task_id/summaries/:summary_id", api.HandleDeleteTaskSummary(projectsReg, taskSummaryService))
	r.GET("/api/v1/projects/:id/summaries/by-week", api.HandleGetSummariesByWeek(projectsReg, taskSummaryService))

	// ========== Frontend Static Files (Must be last) ==========
	// Apply cache control middleware for static resources
	staticGroup := r.Group("/")
	staticGroup.Use(staticCacheMiddleware())
	{
		// Serve frontend static files with cache optimization
		staticGroup.Static("/assets", "./frontend/dist/assets")
		staticGroup.StaticFile("/index.html", "./frontend/dist/index.html")

		// Explicitly serve config.js with correct MIME type and no-cache header
		staticGroup.GET("/config.js", func(c *gin.Context) {
			c.Header("Content-Type", "application/javascript; charset=utf-8")
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
			c.File("./frontend/dist/config.js")
		})
	}

	// Fallback to index.html for SPA routing (must be last)
	r.NoRoute(func(c *gin.Context) {
		// If request path starts with /api/, return 404 JSON
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "endpoint not found"})
			return
		}
		// Otherwise serve index.html for frontend SPA (with no-cache)
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.File("./frontend/dist/index.html")
	})
}

// HealthCheckResponse represents the response from the health check endpoint
type HealthCheckResponse struct {
	Status    string    `json:"status"`
	Service   string    `json:"service"`
	Version   string    `json:"version"`
	Uptime    string    `json:"uptime"`
	Timestamp time.Time `json:"timestamp"`
	Env       string    `json:"env"`
}

// ReadinessCheckResponse represents the response from the readiness check endpoint
type ReadinessCheckResponse struct {
	Ready     bool             `json:"ready"`
	Checks    []ReadinessCheck `json:"checks"`
	Timestamp time.Time        `json:"timestamp"`
}

// ReadinessCheck represents a single readiness check
type ReadinessCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "ok" or "fail"
	Error  string `json:"error,omitempty"`
}

// healthCheckHandler returns the liveness probe handler
func healthCheckHandler(cfg *config.Config, startTime time.Time) gin.HandlerFunc {
	return func(c *gin.Context) {
		response := HealthCheckResponse{
			Status:    "healthy",
			Service:   "aidg-web-server",
			Version:   "1.0.0",
			Uptime:    time.Since(startTime).String(),
			Timestamp: time.Now(),
			Env:       cfg.Server.Env,
		}
		c.JSON(http.StatusOK, response)
	}
}

// readinessCheckHandler returns the readiness probe handler
func readinessCheckHandler(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		checks := []ReadinessCheck{}
		allReady := true

		// Check projects directory
		projectsCheck := ReadinessCheck{Name: "projects_dir", Status: "ok"}
		projectsDir := cfg.Data.ProjectsDir
		if projectsDir == "" {
			projectsDir = "projects"
		}
		if !checkDataDirAccessible(projectsDir) {
			projectsCheck.Status = "fail"
			projectsCheck.Error = "projects directory not accessible"
			allReady = false
		}
		checks = append(checks, projectsCheck)

		// Check users directory
		usersCheck := ReadinessCheck{Name: "users_dir", Status: "ok"}
		usersDir := cfg.Data.UsersDir
		if usersDir == "" {
			usersDir = "users"
		}
		if !checkDataDirAccessible(usersDir) {
			usersCheck.Status = "fail"
			usersCheck.Error = "users directory not accessible"
			allReady = false
		}
		checks = append(checks, usersCheck)

		// Check meetings directory
		meetingsCheck := ReadinessCheck{Name: "meetings_dir", Status: "ok"}
		meetingsDir := cfg.Data.MeetingsDir
		if meetingsDir == "" {
			meetingsDir = "meetings"
		}
		if !checkDataDirAccessible(meetingsDir) {
			meetingsCheck.Status = "fail"
			meetingsCheck.Error = "meetings directory not accessible"
			allReady = false
		}
		checks = append(checks, meetingsCheck)

		response := ReadinessCheckResponse{
			Ready:     allReady,
			Checks:    checks,
			Timestamp: time.Now(),
		}

		httpStatus := http.StatusOK
		if !allReady {
			httpStatus = http.StatusServiceUnavailable
		}

		c.JSON(httpStatus, response)
	}
}

// checkDataDirAccessible checks if a directory is accessible
func checkDataDirAccessible(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// staticCacheMiddleware adds cache control headers for static resources
func staticCacheMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Long-term cache for immutable assets (JS, CSS, fonts, images with hash)
		if strings.HasPrefix(path, "/assets/") {
			// Assets in /assets/ are typically versioned/hashed, safe for long-term caching
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else if strings.HasSuffix(path, ".html") {
			// HTML files should not be cached to ensure users get latest version
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		} else {
			// Default: short-term cache for other resources
			c.Header("Cache-Control", "public, max-age=3600")
		}

		c.Next()
	}
}
