package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"aidg-lite/internal/api"
	"aidg-lite/internal/config"
	"aidg-lite/internal/domain/docslot"
	"aidg-lite/internal/domain/projects"
	"aidg-lite/internal/domain/taskdocs"
	"aidg-lite/internal/executionplan"
	"aidg-lite/internal/middleware"
	"aidg-lite/internal/services"
	"aidg-lite/internal/users"
	"aidg-lite/pkg/logger"
)

const defaultUser = "local"

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	cfg.LiteMode = true

	if cfg.Security.JWTSecret == "" || len(cfg.Security.JWTSecret) < 32 {
		cfg.Security.JWTSecret = "lite-mode-default-secret-key-32chars!!"
	}
	if cfg.Security.AdminDefaultPassword == "" {
		cfg.Security.AdminDefaultPassword = "lite-admin-default"
	}

	logCfg := logger.Config{Environment: cfg.Server.Env, Level: cfg.Log.Level}
	appLogger, err := logger.Init(logCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		os.Exit(1)
	}
	appLogger.Info("AIDG Lite starting", "port", cfg.Server.Port)

	if cfg.Server.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	projectsRoot := cfg.Data.ProjectsDir
	if projectsRoot == "" {
		projectsRoot = "./data/projects"
	}

	usersDir := cfg.Data.UsersDir
	if usersDir == "" {
		usersDir = "./data/users"
	}
	os.MkdirAll(filepath.Join(usersDir, defaultUser), 0o755)

	projects.InitPaths()
	projectsReg := projects.NewProjectRegistry()
	if err := projects.LoadProjects(projectsReg); err != nil {
		appLogger.Warn("failed to load projects", "error", err)
	}
	appLogger.Info("loaded projects", "count", len(projectsReg.List()))

	taskDocSvc := taskdocs.NewDocService()
	sectionService := taskdocs.NewSectionService(projectsRoot)
	execPlanHandler := executionplan.NewHandler(projectsRoot)
	taskSummaryService := services.NewTaskSummaryService(projectsRoot)
	statisticsService := services.NewStatisticsService(projectsRoot)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger())

	r.Use(func(c *gin.Context) {
		c.Set("user", defaultUser)
		c.Set("scopes", []string{
			"meeting.read", "meeting.write",
			"project.doc.read", "project.doc.write",
			"project.admin", "project.delete",
			"task.read", "task.write",
			"user.manage",
		})
		c.Next()
	})

	startTime := time.Now()
	r.GET("/health", healthHandler(cfg, startTime))
	r.GET("/api/v1/health", healthHandler(cfg, startTime))

	r.GET("/api/v1/app/config", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"lite_mode": true, "version": "1.0.0"})
	})

	r.POST("/api/v1/login", func(c *gin.Context) {
		userSecret := []byte(cfg.Security.JWTSecret)
		userManager, err := users.NewManager(usersDir, userSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		_ = userManager.EnsureDefaultAdmin(cfg.Security.AdminDefaultPassword)
		token, _ := userManager.GenerateToken(defaultUser)
		c.JSON(http.StatusOK, gin.H{
			"token":    token,
			"username": defaultUser,
			"scopes": []string{
				"meeting.read", "meeting.write",
				"project.doc.read", "project.doc.write",
				"project.admin", "project.delete",
				"task.read", "task.write", "user.manage",
			},
		})
	})

	r.GET("/api/v1/user/current-task", api.HandleGetUserCurrentTask)
	r.PUT("/api/v1/user/current-task", handlePutUserCurrentTask())

	r.GET("/api/v1/user/projects", api.HandleGetUserProjects(projectsReg))
	r.PUT("/api/v1/user/projects", api.HandleUpdateUserProjects(projectsReg))

	r.GET("/api/v1/projects", api.HandleListProjects(projectsReg))
	r.POST("/api/v1/projects", api.HandleCreateProject(projectsReg))
	r.GET("/api/v1/projects/:id", api.HandleGetProject(projectsReg))
	r.PATCH("/api/v1/projects/:id", api.HandlePatchProject(projectsReg))
	r.DELETE("/api/v1/projects/:id", api.HandleDeleteProject(projectsReg))

	r.GET("/api/v1/projects/:id/feature-list", api.HandleGetFeatureList(projectsReg))
	r.PUT("/api/v1/projects/:id/feature-list", api.HandlePutProjectFeatureList(projectsReg))
	r.GET("/api/v1/projects/:id/feature-list.json", api.HandleGetFeatureListJSON(projectsReg))
	r.PUT("/api/v1/projects/:id/feature-list.json", api.HandlePutFeatureListJSON(projectsReg))
	r.GET("/api/v1/projects/:id/architecture-design", api.HandleGetArchitectureDesign(projectsReg))
	r.PUT("/api/v1/projects/:id/architecture-design", api.HandlePutProjectArchitectureDesign(projectsReg))

	r.GET("/api/v1/projects/:id/tasks", api.HandleListProjectTasks(projectsReg))
	r.GET("/api/v1/projects/:id/tasks/next-incomplete", api.HandleGetNextIncompleteTask(projectsReg))
	r.POST("/api/v1/projects/:id/tasks", api.HandleCreateProjectTask(projectsReg))
	r.GET("/api/v1/projects/:id/tasks/:task_id", api.HandleGetProjectTask(projectsReg))
	r.PUT("/api/v1/projects/:id/tasks/:task_id", api.HandleUpdateProjectTask(projectsReg))
	r.DELETE("/api/v1/projects/:id/tasks/:task_id", api.HandleDeleteProjectTask(projectsReg))

	taskDocHandler := makeTaskDocHandler(projectsRoot, projectsReg, taskDocSvc)
	for _, dt := range []string{"requirements", "design", "test"} {
		r.GET("/api/v1/projects/:id/tasks/:task_id/"+dt, taskDocHandler(dt))
		r.PUT("/api/v1/projects/:id/tasks/:task_id/"+dt, taskDocHandler(dt))
		r.POST("/api/v1/projects/:id/tasks/:task_id/"+dt+"/append", api.HandleAppendTaskDoc(taskDocSvc, dt))
		group := r.Group("/api/v1/projects/:id/tasks/:task_id/" + dt)
		api.RegisterSectionRoutes(group, sectionService)
	}

	execPlanHandler.RegisterRoutes(r)

	// Tag版本管理路由
	tagHandler := api.NewTagHandler(projectsRoot)
	for _, dt := range []string{"requirements", "design", "test"} {
		tagGroup := r.Group("/api/v1/projects/:id/tasks/:task_id/docs/" + dt + "/tags")
		tagGroup.POST("", tagHandler.CreateTagWithDocType(dt))
		tagGroup.GET("", tagHandler.ListTagsWithDocType(dt))
		tagGroup.POST("/:tagName/switch", tagHandler.SwitchTagWithDocType(dt))
		tagGroup.GET("/:tagName", tagHandler.GetTagInfoWithDocType(dt))
		tagGroup.DELETE("/:tagName", tagHandler.DeleteTagWithDocType(dt))
	}
	// execution-plan 的 tag 路由没有 /docs/ 前缀
	epTagGroup := r.Group("/api/v1/projects/:id/tasks/:task_id/execution-plan/tags")
	epTagGroup.POST("", tagHandler.CreateTagWithDocType("execution-plan"))
	epTagGroup.GET("", tagHandler.ListTagsWithDocType("execution-plan"))
	epTagGroup.POST("/:tagName/switch", tagHandler.SwitchTagWithDocType("execution-plan"))
	epTagGroup.GET("/:tagName", tagHandler.GetTagInfoWithDocType("execution-plan"))
	epTagGroup.DELETE("/:tagName", tagHandler.DeleteTagWithDocType("execution-plan"))

	// 项目文档统一路由 (export, sections, append, chunks, squash)
	unifiedDocSvc := docslot.NewUnifiedDocService(projectsRoot)
	apiGroup := r.Group("/api/v1")
	api.RegisterUnifiedDocRoutes(apiGroup, unifiedDocSvc)

	// Prompts 路由
	r.GET("/api/v1/projects/:id/tasks/:task_id/prompts", api.HandleGetProjectTaskPrompts(projectsReg))
	r.POST("/api/v1/projects/:id/tasks/:task_id/prompts", api.HandleCreateProjectTaskPrompt(projectsReg))

	// Remotes 存根路由（lite模式无远端）
	r.GET("/api/v1/remotes", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"remotes": []interface{}{}})
	})

	r.GET("/api/v1/projects/:id/tasks/:task_id/summaries", api.HandleGetTaskSummaries(projectsReg, taskSummaryService))
	r.POST("/api/v1/projects/:id/tasks/:task_id/summaries", api.HandleAddTaskSummary(projectsReg, taskSummaryService))
	r.PUT("/api/v1/projects/:id/tasks/:task_id/summaries/:summary_id", api.HandleUpdateTaskSummary(projectsReg, taskSummaryService))
	r.DELETE("/api/v1/projects/:id/tasks/:task_id/summaries/:summary_id", api.HandleDeleteTaskSummary(projectsReg, taskSummaryService))
	r.GET("/api/v1/projects/:id/summaries/by-week", api.HandleGetSummariesByWeek(projectsReg, taskSummaryService))
	r.GET("/api/v1/projects/:id/tasks/statistics", api.HandleGetTaskStatistics(projectsReg, statisticsService))

	frontendDistDir := cfg.Frontend.DistDir
	if frontendDistDir == "" {
		frontendDistDir = "./frontend/dist"
	}
	staticGroup := r.Group("/")
	staticGroup.Use(staticCacheMiddleware())
	staticGroup.Static("/assets", filepath.Join(frontendDistDir, "assets"))
	staticGroup.StaticFile("/index.html", filepath.Join(frontendDistDir, "index.html"))

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/internal/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "endpoint not found"})
			return
		}
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.File(filepath.Join(frontendDistDir, "index.html"))
	})

	serverAddr := fmt.Sprintf("127.0.0.1:%s", cfg.Server.Port)
	srv := &http.Server{Addr: serverAddr, Handler: r}

	go func() {
		appLogger.Info("lite server listening", "addr", serverAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-quit
	appLogger.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		appLogger.Error("forced shutdown", "error", err)
		os.Exit(1)
	}
	appLogger.Info("server stopped")
}

func makeTaskDocHandler(projectsRoot string, projectsReg *projects.ProjectRegistry, taskDocSvc *taskdocs.DocService) func(string) gin.HandlerFunc {
	return func(docType string) gin.HandlerFunc {
		return func(c *gin.Context) {
			projectID := c.Param("id")
			taskID := c.Param("task_id")
			if projectsReg.Get(projectID) == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
				return
			}
			taskDir := filepath.Join(projectsRoot, projectID, "tasks", taskID)
			legacyFile := filepath.Join(taskDir, fmt.Sprintf("%s.md", docType))

			migrateIfNeeded := func() error {
				meta, mErr := taskdocs.LoadOrInitMeta(projectID, taskID, docType)
				if mErr != nil {
					return mErr
				}
				if meta.Version > 0 {
					return nil
				}
				data, readErr := os.ReadFile(legacyFile)
				if readErr != nil {
					return nil
				}
				content := strings.TrimSpace(string(data))
				if content == "" {
					return nil
				}
				_, _, _, aErr := taskDocSvc.Append(projectID, taskID, docType, content, "migration", nil, "replace_full", "migration")
				return aErr
			}

			if c.Request.Method == http.MethodGet {
				_ = migrateIfNeeded()
				compiledPath, _ := taskdocs.DocCompiledPath(projectID, taskID, docType)
				b, _ := os.ReadFile(compiledPath)
				if len(b) == 0 {
					if lb, err2 := os.ReadFile(legacyFile); err2 == nil {
						b = lb
					}
				}
				meta, _ := taskdocs.LoadOrInitMeta(projectID, taskID, docType)
				c.JSON(http.StatusOK, gin.H{
					"exists":  len(b) > 0,
					"content": string(b),
					"version": meta.Version,
					"etag":    meta.ETag,
				})
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
				os.MkdirAll(taskDir, 0755)
				meta, _, duplicate, aErr := taskDocSvc.Append(projectID, taskID, docType, body.Content, "put_api", body.ExpectedVersion, "replace_full", "put")
				if aErr != nil {
					if aErr.Error() == "version_mismatch" {
						c.JSON(http.StatusConflict, gin.H{"error": "version_mismatch"})
						return
					}
					c.JSON(http.StatusInternalServerError, gin.H{"error": aErr.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"success": true, "version": meta.Version, "duplicate": duplicate, "etag": meta.ETag})
				return
			}
			c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
		}
	}
}

func handlePutUserCurrentTask() gin.HandlerFunc {
	return func(c *gin.Context) {
		username := defaultUser
		if u := c.GetString("user"); u != "" {
			username = u
		}
		var req struct {
			ProjectID string `json:"project_id"`
			TaskID    string `json:"task_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		if req.ProjectID == "" || req.TaskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "project_id and task_id are required"})
			return
		}
		projDir := filepath.Join(projects.ProjectsRoot, req.ProjectID)
		if _, err := os.Stat(projDir); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
			return
		}
		tasksFile := filepath.Join(projects.ProjectsRoot, req.ProjectID, "tasks.json")
		data, err := os.ReadFile(tasksFile)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "tasks not found"})
			return
		}
		var tasks []map[string]interface{}
		if err := json.Unmarshal(data, &tasks); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse tasks"})
			return
		}
		found := false
		for _, t := range tasks {
			if id, ok := t["id"].(string); ok && id == req.TaskID {
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		usersDir := os.Getenv("USERS_DIR")
		if usersDir == "" {
			usersDir = "./data/users"
		}
		userDir := filepath.Join(usersDir, username)
		os.MkdirAll(userDir, 0o755)
		taskData := map[string]interface{}{"project_id": req.ProjectID, "task_id": req.TaskID, "set_at": time.Now()}
		b, _ := json.MarshalIndent(taskData, "", "  ")
		if err := os.WriteFile(filepath.Join(userDir, "current_task.json"), b, 0o644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "current task updated"})
	}
}

type HealthCheckResponse struct {
	Status    string    `json:"status"`
	Service   string    `json:"service"`
	Version   string    `json:"version"`
	Uptime    string    `json:"uptime"`
	Timestamp time.Time `json:"timestamp"`
	Env       string    `json:"env"`
}

func healthHandler(cfg *config.Config, startTime time.Time) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, HealthCheckResponse{
			Status: "healthy", Service: "aidg-lite", Version: "1.0.0",
			Uptime: time.Since(startTime).String(), Timestamp: time.Now(), Env: cfg.Server.Env,
		})
	}
}

func staticCacheMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/assets/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else if strings.HasSuffix(p, ".html") {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		} else {
			c.Header("Cache-Control", "public, max-age=3600")
		}
		c.Next()
	}
}

// Suppress unused import warnings
var _ = slog.Info
