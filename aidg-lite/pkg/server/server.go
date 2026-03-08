// Package server provides the AIDG Lite HTTP server as an importable package.
// Usage:
//
//	cfg, _ := config.LoadConfig()
//	s, _ := server.New(cfg)
//	s.Start()
package server

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

	"github.com/houzhh15/aidg-lite/internal/api"
	"github.com/houzhh15/aidg-lite/internal/config"
	"github.com/houzhh15/aidg-lite/internal/domain/docslot"
	"github.com/houzhh15/aidg-lite/internal/domain/projects"
	"github.com/houzhh15/aidg-lite/internal/domain/taskdocs"
	"github.com/houzhh15/aidg-lite/internal/executionplan"
	"github.com/houzhh15/aidg-lite/internal/middleware"
	"github.com/houzhh15/aidg-lite/internal/services"
	"github.com/houzhh15/aidg-lite/internal/users"
	"github.com/houzhh15/aidg-lite/pkg/logger"
)

// DefaultUser is the fixed identity used in lite (single-user) mode.
const DefaultUser = "local"

// Server encapsulates the AIDG Lite HTTP server.
type Server struct {
	cfg       *config.Config
	router    *gin.Engine
	httpSrv   *http.Server
	logger    *slog.Logger
	startTime time.Time
}

// New creates and configures a new Server from the given config.
// All routes are registered during construction; call Start or ListenAndServe to begin accepting requests.
func New(cfg *config.Config) (*Server, error) {
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
		return nil, fmt.Errorf("init logger: %w", err)
	}

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
	os.MkdirAll(filepath.Join(usersDir, DefaultUser), 0o755)

	projects.InitPaths()
	projectsReg := projects.NewProjectRegistry()
	if err := projects.LoadProjects(projectsReg); err != nil {
		appLogger.Warn("failed to load projects", "error", err)
	}

	taskDocSvc := taskdocs.NewDocService()
	sectionService := taskdocs.NewSectionService(projectsRoot)
	execPlanHandler := executionplan.NewHandler(projectsRoot)
	taskSummaryService := services.NewTaskSummaryService(projectsRoot)
	statisticsService := services.NewStatisticsService(projectsRoot)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger())

	// Lite mode: inject default user with full scopes, no auth required.
	r.Use(func(c *gin.Context) {
		c.Set("user", DefaultUser)
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

	// ── Health ────────────────────────────────────────────────────────────
	r.GET("/health", buildHealthHandler(cfg, startTime))
	r.GET("/api/v1/health", buildHealthHandler(cfg, startTime))

	// ── App config ────────────────────────────────────────────────────────
	r.GET("/api/v1/app/config", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"lite_mode": true, "version": "1.0.0"})
	})

	// ── Auth ──────────────────────────────────────────────────────────────
	r.POST("/api/v1/login", buildLoginHandler(cfg, usersDir))

	// ── User / current-task ───────────────────────────────────────────────
	r.GET("/api/v1/user/current-task", api.HandleGetUserCurrentTask)
	r.PUT("/api/v1/user/current-task", buildPutCurrentTaskHandler(usersDir, projectsRoot))

	// ── User projects ─────────────────────────────────────────────────────
	r.GET("/api/v1/user/projects", api.HandleGetUserProjects(projectsReg))
	r.PUT("/api/v1/user/projects", api.HandleUpdateUserProjects(projectsReg))

	// ── Projects CRUD ─────────────────────────────────────────────────────
	r.GET("/api/v1/projects", api.HandleListProjects(projectsReg))
	r.POST("/api/v1/projects", api.HandleCreateProject(projectsReg))
	r.GET("/api/v1/projects/:id", api.HandleGetProject(projectsReg))
	r.PATCH("/api/v1/projects/:id", api.HandlePatchProject(projectsReg))
	r.DELETE("/api/v1/projects/:id", api.HandleDeleteProject(projectsReg))

	// ── Project documents ─────────────────────────────────────────────────
	r.GET("/api/v1/projects/:id/feature-list", api.HandleGetFeatureList(projectsReg))
	r.PUT("/api/v1/projects/:id/feature-list", api.HandlePutProjectFeatureList(projectsReg))
	r.GET("/api/v1/projects/:id/feature-list.json", api.HandleGetFeatureListJSON(projectsReg))
	r.PUT("/api/v1/projects/:id/feature-list.json", api.HandlePutFeatureListJSON(projectsReg))
	r.GET("/api/v1/projects/:id/architecture-design", api.HandleGetArchitectureDesign(projectsReg))
	r.PUT("/api/v1/projects/:id/architecture-design", api.HandlePutProjectArchitectureDesign(projectsReg))

	// ── Tasks ─────────────────────────────────────────────────────────────
	r.GET("/api/v1/projects/:id/tasks", api.HandleListProjectTasks(projectsReg))
	r.GET("/api/v1/projects/:id/tasks/next-incomplete", api.HandleGetNextIncompleteTask(projectsReg))
	r.POST("/api/v1/projects/:id/tasks", api.HandleCreateProjectTask(projectsReg))
	r.GET("/api/v1/projects/:id/tasks/:task_id", api.HandleGetProjectTask(projectsReg))
	r.PUT("/api/v1/projects/:id/tasks/:task_id", api.HandleUpdateProjectTask(projectsReg))
	r.DELETE("/api/v1/projects/:id/tasks/:task_id", api.HandleDeleteProjectTask(projectsReg))

	// ── Task docs (requirements / design / test) ───────────────────────────
	taskDocHandler := buildTaskDocHandler(projectsRoot, projectsReg, taskDocSvc)
	for _, dt := range []string{"requirements", "design", "test"} {
		r.GET("/api/v1/projects/:id/tasks/:task_id/"+dt, taskDocHandler(dt))
		r.PUT("/api/v1/projects/:id/tasks/:task_id/"+dt, taskDocHandler(dt))
		r.POST("/api/v1/projects/:id/tasks/:task_id/"+dt+"/append", api.HandleAppendTaskDoc(taskDocSvc, dt))
		group := r.Group("/api/v1/projects/:id/tasks/:task_id/" + dt)
		api.RegisterSectionRoutes(group, sectionService)
	}

	// ── Execution plan ────────────────────────────────────────────────────
	execPlanHandler.RegisterRoutes(r)

	// ── Tags ──────────────────────────────────────────────────────────────
	tagHandler := api.NewTagHandler(projectsRoot)
	for _, dt := range []string{"requirements", "design", "test"} {
		tg := r.Group("/api/v1/projects/:id/tasks/:task_id/docs/" + dt + "/tags")
		tg.POST("", tagHandler.CreateTagWithDocType(dt))
		tg.GET("", tagHandler.ListTagsWithDocType(dt))
		tg.POST("/:tagName/switch", tagHandler.SwitchTagWithDocType(dt))
		tg.GET("/:tagName", tagHandler.GetTagInfoWithDocType(dt))
		tg.DELETE("/:tagName", tagHandler.DeleteTagWithDocType(dt))
	}
	// execution-plan tags have no /docs/ prefix
	epTg := r.Group("/api/v1/projects/:id/tasks/:task_id/execution-plan/tags")
	epTg.POST("", tagHandler.CreateTagWithDocType("execution-plan"))
	epTg.GET("", tagHandler.ListTagsWithDocType("execution-plan"))
	epTg.POST("/:tagName/switch", tagHandler.SwitchTagWithDocType("execution-plan"))
	epTg.GET("/:tagName", tagHandler.GetTagInfoWithDocType("execution-plan"))
	epTg.DELETE("/:tagName", tagHandler.DeleteTagWithDocType("execution-plan"))

	// ── Unified doc routes (export / sections / append / chunks / squash) ─
	unifiedDocSvc := docslot.NewUnifiedDocService(projectsRoot)
	api.RegisterUnifiedDocRoutes(r.Group("/api/v1"), unifiedDocSvc)

	// ── Prompts ───────────────────────────────────────────────────────────
	r.GET("/api/v1/projects/:id/tasks/:task_id/prompts", api.HandleGetProjectTaskPrompts(projectsReg))
	r.POST("/api/v1/projects/:id/tasks/:task_id/prompts", api.HandleCreateProjectTaskPrompt(projectsReg))

	// ── Remotes stub (lite mode has no remotes) ───────────────────────────
	r.GET("/api/v1/remotes", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"remotes": []interface{}{}})
	})

	// ── Task summaries & statistics ───────────────────────────────────────
	r.GET("/api/v1/projects/:id/tasks/:task_id/summaries", api.HandleGetTaskSummaries(projectsReg, taskSummaryService))
	r.POST("/api/v1/projects/:id/tasks/:task_id/summaries", api.HandleAddTaskSummary(projectsReg, taskSummaryService))
	r.PUT("/api/v1/projects/:id/tasks/:task_id/summaries/:summary_id", api.HandleUpdateTaskSummary(projectsReg, taskSummaryService))
	r.DELETE("/api/v1/projects/:id/tasks/:task_id/summaries/:summary_id", api.HandleDeleteTaskSummary(projectsReg, taskSummaryService))
	r.GET("/api/v1/projects/:id/summaries/by-week", api.HandleGetSummariesByWeek(projectsReg, taskSummaryService))
	r.GET("/api/v1/projects/:id/tasks/statistics", api.HandleGetTaskStatistics(projectsReg, statisticsService))

	// ── Frontend static files ─────────────────────────────────────────────
	frontendDistDir := cfg.Frontend.DistDir
	if frontendDistDir == "" {
		frontendDistDir = "./frontend/dist"
	}
	sg := r.Group("/")
	sg.Use(staticCacheMiddleware())
	sg.Static("/assets", filepath.Join(frontendDistDir, "assets"))
	sg.StaticFile("/index.html", filepath.Join(frontendDistDir, "index.html"))

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/internal/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "endpoint not found"})
			return
		}
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.File(filepath.Join(frontendDistDir, "index.html"))
	})

	serverAddr := fmt.Sprintf("127.0.0.1:%s", cfg.Server.Port)

	return &Server{
		cfg:       cfg,
		router:    r,
		httpSrv:   &http.Server{Addr: serverAddr, Handler: r},
		logger:    appLogger,
		startTime: startTime,
	}, nil
}

// Handler returns the underlying http.Handler (gin engine).
// Useful for embedding in custom HTTP servers or testing.
func (s *Server) Handler() http.Handler { return s.router }

// ListenAndServe starts the HTTP listener and blocks until the server closes.
// It does NOT install signal handlers; use Start for the full daemon behaviour.
func (s *Server) ListenAndServe() error {
	s.logger.Info("aidg-lite listening", "addr", s.httpSrv.Addr)
	if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully drains in-flight requests within the given context deadline.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}

// Start runs the server in a background goroutine and blocks until SIGINT / SIGTERM /
// SIGQUIT is received, then performs a graceful 10-second shutdown.
// This is the typical entry point for a standalone binary.
func (s *Server) Start() {
	s.logger.Info("AIDG Lite starting", "port", s.cfg.Server.Port)

	go func() {
		if err := s.ListenAndServe(); err != nil {
			s.logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-quit
	s.logger.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		s.logger.Error("forced shutdown", "error", err)
		os.Exit(1)
	}
	s.logger.Info("server stopped")
}

// ─── private helpers ──────────────────────────────────────────────────────────

type healthCheckResponse struct {
	Status    string    `json:"status"`
	Service   string    `json:"service"`
	Version   string    `json:"version"`
	Uptime    string    `json:"uptime"`
	Timestamp time.Time `json:"timestamp"`
	Env       string    `json:"env"`
}

func buildHealthHandler(cfg *config.Config, startTime time.Time) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, healthCheckResponse{
			Status: "healthy", Service: "aidg-lite", Version: "1.0.0",
			Uptime: time.Since(startTime).String(), Timestamp: time.Now(), Env: cfg.Server.Env,
		})
	}
}

func buildLoginHandler(cfg *config.Config, usersDir string) gin.HandlerFunc {
	userSecret := []byte(cfg.Security.JWTSecret)
	return func(c *gin.Context) {
		userManager, err := users.NewManager(usersDir, userSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		_ = userManager.EnsureDefaultAdmin(cfg.Security.AdminDefaultPassword)
		token, _ := userManager.GenerateToken(DefaultUser)
		c.JSON(http.StatusOK, gin.H{
			"token":    token,
			"username": DefaultUser,
			"scopes": []string{
				"meeting.read", "meeting.write",
				"project.doc.read", "project.doc.write",
				"project.admin", "project.delete",
				"task.read", "task.write", "user.manage",
			},
		})
	}
}

func buildPutCurrentTaskHandler(usersDir, projectsRoot string) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := DefaultUser
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

func buildTaskDocHandler(projectsRoot string, projectsReg *projects.ProjectRegistry, taskDocSvc *taskdocs.DocService) func(string) gin.HandlerFunc {
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

func staticCacheMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		switch {
		case strings.HasPrefix(p, "/assets/"):
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		case strings.HasSuffix(p, ".html"):
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		default:
			c.Header("Cache-Control", "public, max-age=3600")
		}
		c.Next()
	}
}

// Suppress unused import
var _ = slog.Info
