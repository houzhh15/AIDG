package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/taskdocs"
	"github.com/houzhh15/AIDG/cmd/server/internal/executionplan"
	"github.com/houzhh15/AIDG/cmd/server/internal/models"
	"github.com/houzhh15/AIDG/cmd/server/internal/services"
	"github.com/houzhh15/AIDG/cmd/server/internal/simhash"
	"github.com/houzhh15/AIDG/cmd/server/internal/users"
)

// enrichTaskWithCompletionStatus 为任务信息添加4个文档槽位的完成状态
// 槽位: requirements(需求文档), design(设计文档), execution_plan(执行计划), test(测试文档)
// 状态: "" (空/未完成) 或 "completed" (已完成)
func enrichTaskWithCompletionStatus(task map[string]interface{}, projectID, taskID string) {
	completion := map[string]string{
		"requirements":   "",
		"design":         "",
		"execution_plan": "",
		"test":           "",
	}

	// 检查需求文档
	if checkDocSlotCompleted(projectID, taskID, "requirements") {
		completion["requirements"] = "completed"
	}

	// 检查设计文档
	if checkDocSlotCompleted(projectID, taskID, "design") {
		completion["design"] = "completed"
	}

	// 检查执行计划
	if checkExecutionPlanCompleted(projectID, taskID) {
		completion["execution_plan"] = "completed"
	}

	// 检查测试文档
	if checkDocSlotCompleted(projectID, taskID, "test") {
		completion["test"] = "completed"
	}

	task["completion"] = completion
}

// checkDocSlotCompleted 检查某个文档槽位是否已有内容
func checkDocSlotCompleted(projectID, taskID, docType string) bool {
	compiledPath, err := taskdocs.DocCompiledPath(projectID, taskID, docType)
	if err != nil {
		return false
	}
	data, err := os.ReadFile(compiledPath)
	if err != nil {
		// 尝试 legacy 路径
		legacyPath := filepath.Join(projectsRoot(), projectID, "tasks", taskID, docType+".md")
		data, err = os.ReadFile(legacyPath)
		if err != nil {
			return false
		}
	}
	return len(strings.TrimSpace(string(data))) > 0
}

// checkExecutionPlanCompleted 检查执行计划是否已有内容
func checkExecutionPlanCompleted(projectID, taskID string) bool {
	repo, err := executionplan.NewFileRepository(projectsRoot(), projectID, taskID)
	if err != nil {
		return false
	}
	_, err = repo.Read(context.Background())
	return err == nil
}

// getExecutionPlanStatus 获取执行计划的详细状态
// 返回值: "" (无计划), "plan_completed" (有计划但未全部执行完), "execution_completed" (所有步骤执行完成)
func getExecutionPlanStatus(projectID, taskID string) string {
	repo, err := executionplan.NewFileRepository(projectsRoot(), projectID, taskID)
	if err != nil {
		return ""
	}
	svc := services.NewExecutionPlanService(repo)
	plan, err := svc.Load(context.Background())
	if err != nil {
		return ""
	}
	if plan.Status == models.PlanStatusCompleted {
		return "execution_completed"
	}
	return "plan_completed"
}

// HandleGetNextIncompleteTask GET /api/v1/projects/:id/tasks/next-incomplete
// 获取下一个有未完成文档的任务
// 可选参数 doc_type: requirements, design, plan, test
func HandleGetNextIncompleteTask(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		docTypeFilter := c.Query("doc_type") // 可选: requirements, design, plan, test

		// Verify project exists
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}

		projDir := filepath.Join(projectsRoot(), projectID)
		tasksFile := filepath.Join(projDir, "tasks.json")

		if _, err := os.Stat(tasksFile); os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": nil, "message": "no tasks found"})
			return
		}

		data, err := os.ReadFile(tasksFile)
		if err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to read tasks: %w", err))
			return
		}

		var taskList []map[string]interface{}
		if err := json.Unmarshal(data, &taskList); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": nil, "message": "no tasks found"})
			return
		}

		// 验证 doc_type 参数
		validDocTypes := map[string]bool{"requirements": true, "design": true, "plan": true, "test": true}
		if docTypeFilter != "" && !validDocTypes[docTypeFilter] {
			badRequestResponse(c, "invalid doc_type, must be one of: requirements, design, plan, test")
			return
		}

		// 遍历任务列表，找到第一个满足条件的任务
		for _, task := range taskList {
			taskID, ok := task["id"].(string)
			if !ok || taskID == "" {
				continue
			}

			// 计算各文档槽位的状态
			reqCompleted := checkDocSlotCompleted(projectID, taskID, "requirements")
			designCompleted := checkDocSlotCompleted(projectID, taskID, "design")
			planStatus := getExecutionPlanStatus(projectID, taskID)
			testCompleted := checkDocSlotCompleted(projectID, taskID, "test")

			// 判断是否满足筛选条件
			var matchesFilter bool
			switch docTypeFilter {
			case "requirements":
				matchesFilter = !reqCompleted
			case "design":
				matchesFilter = !designCompleted
			case "plan":
				matchesFilter = planStatus != "execution_completed"
			case "test":
				matchesFilter = !testCompleted
			default:
				// 无筛选时，任意一项未完成即匹配
				matchesFilter = !reqCompleted || !designCompleted || planStatus != "execution_completed" || !testCompleted
			}

			if !matchesFilter {
				continue
			}

			// 构建完成状态
			completion := map[string]string{
				"requirements": "",
				"design":       "",
				"plan":         planStatus, // "", "plan_completed", "execution_completed"
				"test":         "",
			}
			if reqCompleted {
				completion["requirements"] = "completed"
			}
			if designCompleted {
				completion["design"] = "completed"
			}
			if testCompleted {
				completion["test"] = "completed"
			}
			task["completion"] = completion

			// 推荐下一个要完成的内容，按优先级: requirements > design > plan > test
			recommended := ""
			if !reqCompleted {
				recommended = "requirements"
			} else if !designCompleted {
				recommended = "design"
			} else if planStatus != "execution_completed" {
				recommended = "plan"
			} else if !testCompleted {
				recommended = "test"
			}

			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"task":        task,
					"recommended": recommended,
				},
			})
			return
		}

		// 没有满足条件的任务
		c.JSON(http.StatusOK, gin.H{"success": true, "data": nil, "message": "all tasks completed"})
	}
}

// projectsRoot resolves to the configured projects directory
func projectsRoot() string {
	if strings.TrimSpace(projects.ProjectsRoot) != "" {
		return projects.ProjectsRoot
	}
	projects.InitPaths()
	return projects.ProjectsRoot
}

// hasProjectPermission 检查用户是否对指定项目有权限
// 通过检查用户是否在项目中有角色来判断
func hasProjectPermission(username, projectID string) bool {
	// 检查用户角色文件是否存在该项目的角色
	userRolesPath := filepath.Join(projectsRoot(), "user_roles", username+".json")
	data, err := os.ReadFile(userRolesPath)
	if err != nil {
		return false // 用户没有角色文件
	}

	var userRoles struct {
		Username string `json:"username"`
		Projects map[string]struct {
			RoleID   string `json:"role_id"`
			RoleName string `json:"role_name"`
		} `json:"projects"`
	}

	if err := json.Unmarshal(data, &userRoles); err != nil {
		return false
	}

	// 检查用户是否在该项目中有角色
	_, hasRole := userRoles.Projects[projectID]
	return hasRole
}

// hasProjectPermissionWithScopes 检查用户是否有权限访问项目（考虑用户的全局权限）
func hasProjectPermissionWithScopes(username, projectID string, userScopes interface{}) bool {
	// 将 userScopes 转换为 []string
	scopesSlice, ok := userScopes.([]string)
	if !ok {
		return false
	}

	// 检查用户是否有全局权限（可以看到所有项目）
	// user.manage, project.admin, project.doc.read 都可以看到所有项目
	for _, scope := range scopesSlice {
		if scope == users.ScopeUserManage || scope == users.ScopeProjectAdmin || scope == users.ScopeProjectDocRead {
			return true
		}
	}

	// 否则检查项目特定权限
	return hasProjectPermission(username, projectID)
}

// HandleListProjects GET /api/v1/projects
// 获取用户有权限访问的项目列表
func HandleListProjects(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取当前用户
		username, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// 获取用户权限
		userScopes, scopesExists := c.Get("scopes")
		if !scopesExists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		list := reg.List()

		// sort by created time
		sort.Slice(list, func(i, j int) bool {
			return list[i].CreatedAt.Before(list[j].CreatedAt)
		})

		out := []gin.H{}
		for _, p := range list {
			// 检查用户是否有该项目的权限
			if hasProjectPermissionWithScopes(username.(string), p.ID, userScopes) {
				out = append(out, gin.H{
					"id":           p.ID,
					"name":         p.Name,
					"product_line": p.ProductLine,
					"created_at":   p.CreatedAt,
					"updated_at":   p.UpdatedAt,
				})
			}
		}

		c.JSON(http.StatusOK, gin.H{"projects": out})
	}
}

// HandleCreateProject POST /api/v1/projects
// 创建新项目
func HandleCreateProject(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			ID          *string `json:"id"`
			Name        string  `json:"name"`
			ProductLine string  `json:"product_line"`
			FromTaskID  string  `json:"from_task_id"`
		}

		if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Name) == "" {
			badRequestResponse(c, "invalid request")
			return
		}

		// Use project name as ID directly
		name := strings.TrimSpace(body.Name)

		// Validate project name
		if !projects.IsValidProjectName(name) {
			badRequestResponse(c, "invalid project name")
			return
		}

		// Ensure unique
		if reg.Get(name) != nil {
			badRequestResponse(c, "project name already exists")
			return
		}

		projDir := filepath.Join(projectsRoot(), name)
		if err := os.MkdirAll(projDir, 0o755); err != nil {
			internalErrorResponse(c, fmt.Errorf("mkdir failed: %w", err))
			return
		}

		now := time.Now()
		p := &projects.Project{
			ID:          name,
			Name:        name,
			ProductLine: strings.TrimSpace(body.ProductLine),
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		reg.Set(p)
		projects.SaveProjects(reg)

		// TODO: optional copy from task deliverables if body.FromTaskID != ""
		// This requires access to meetings.Registry which we don't have here
		// For now, skip this feature

		c.JSON(http.StatusOK, gin.H{
			"id":           p.ID,
			"name":         p.Name,
			"product_line": p.ProductLine,
			"created_at":   p.CreatedAt,
			"updated_at":   p.UpdatedAt,
		})
	}
}

// HandleGetProject GET /api/v1/projects/:id
// 获取单个项目信息
func HandleGetProject(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		p := reg.Get(id)

		if p == nil {
			notFoundResponse(c, "project")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":           p.ID,
			"name":         p.Name,
			"product_line": p.ProductLine,
			"created_at":   p.CreatedAt,
			"updated_at":   p.UpdatedAt,
		})
	}
}

// HandlePatchProject PATCH /api/v1/projects/:id
// 更新项目信息
func HandlePatchProject(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		p := reg.Get(id)
		if p == nil {
			notFoundResponse(c, "project")
			return
		}

		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}

		name := ""
		productLine := ""

		if v, ok := body["name"].(string); ok && strings.TrimSpace(v) != "" {
			name = strings.TrimSpace(v)
		}

		if v, ok := body["product_line"].(string); ok {
			productLine = strings.TrimSpace(v)
		}

		p = reg.Update(id, name, productLine)
		if p != nil {
			projects.SaveProjects(reg)
		}

		c.JSON(http.StatusOK, gin.H{
			"id":           p.ID,
			"name":         p.Name,
			"product_line": p.ProductLine,
			"created_at":   p.CreatedAt,
			"updated_at":   p.UpdatedAt,
		})
	}
}

// HandleDeleteProject DELETE /api/v1/projects/:id
// 删除项目
func HandleDeleteProject(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		p := reg.Delete(id)
		if p == nil {
			notFoundResponse(c, "project")
			return
		}

		projects.SaveProjects(reg)

		// Remove directory only under projectsRoot
		projDir := filepath.Join(projectsRoot(), id)
		absProj, _ := filepath.Abs(projDir)
		absRoot, _ := filepath.Abs(projectsRoot())

		removed := false
		if strings.HasPrefix(absProj, absRoot+string(os.PathSeparator)) {
			if err := os.RemoveAll(projDir); err == nil {
				removed = true
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"deleted":     id,
			"removed_dir": removed,
		})
	}
}

// HandleListProjectTasks GET /api/v1/projects/:id/tasks
// 获取项目的任务列表，支持搜索查询和时间范围筛选
// 查询参数:
//   - q: 搜索关键词（支持搜索任务名和负责人）
//   - time_range: 时间筛选 (today, week, month)
//   - fuzzy: 是否启用模糊搜索 (true/false，默认false)
func HandleListProjectTasks(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		query := c.Query("q")                     // 搜索查询
		timeRange := c.Query("time_range")        // 时间筛选: today, week, month
		fuzzySearch := c.Query("fuzzy") == "true" // 模糊搜索开关，默认关闭

		// Verify project exists
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}

		projDir := filepath.Join(projectsRoot(), projectID)
		tasksFile := filepath.Join(projDir, "tasks.json")

		// If tasks.json doesn't exist, return empty list
		if _, err := os.Stat(tasksFile); os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": []map[string]interface{}{}})
			return
		}

		// Read tasks.json
		data, err := os.ReadFile(tasksFile)
		if err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to read tasks: %w", err))
			return
		}

		var taskList []map[string]interface{}
		if err := json.Unmarshal(data, &taskList); err != nil {
			// If unmarshal fails, return empty list
			c.JSON(http.StatusOK, gin.H{"success": true, "data": []map[string]interface{}{}})
			return
		}

		// 懒加载：为旧任务自动补全 simhash 字段
		needsSave := false
		for i, task := range taskList {
			// 如果任务有 name 但没有 simhash，自动计算
			if name, ok := task["name"].(string); ok && name != "" {
				if _, hasSimhash := task["simhash"]; !hasSimhash {
					hash := simhash.CalculateSimHash(name)
					task["simhash"] = hash
					taskList[i] = task
					needsSave = true
					fmt.Printf("DEBUG: Lazy-loaded simhash for task %s: %d (0x%x)\n",
						task["id"], hash, hash)
				}
			}
		}

		// 如果有任务更新了 simhash，保存回文件
		if needsSave {
			if data, err := json.MarshalIndent(taskList, "", "  "); err == nil {
				os.WriteFile(tasksFile, data, 0644)
				fmt.Printf("INFO: Saved lazy-loaded simhash for %s tasks in project %s\n",
					"some", projectID)
			}
		}

		originalCount := len(taskList)

		// 如果有搜索查询，根据 fuzzySearch 参数决定搜索模式
		// fuzzySearch=true: 使用 SimHash 语义搜索 + 字符串搜索（OR 关系）
		// fuzzySearch=false: 仅使用字符串搜索
		if query != "" {
			searchStart := time.Now()
			var queryHash uint64
			if fuzzySearch {
				queryHash = simhash.CalculateSimHash(query)
			}
			queryLower := strings.ToLower(query)

			// 存储匹配结果及其距离（distance用于排序）
			type matchedTask struct {
				task       map[string]interface{}
				distance   int    // SimHash汉明距离
				exactMatch bool   // 是否精确包含查询词
				matchType  string // "both", "simhash", "string"
			}
			matches := []matchedTask{}

			// 遍历所有任务
			for _, task := range taskList {
				name, nameOk := task["name"].(string)
				if !nameOk || name == "" {
					continue
				}

				nameLower := strings.ToLower(name)

				// 获取负责人字段
				assignee, _ := task["assignee"].(string)
				assigneeLower := strings.ToLower(assignee)

				// 检查字符串包含（任务名或负责人）
				stringMatch := strings.Contains(nameLower, queryLower) || strings.Contains(assigneeLower, queryLower)

				// 检查 SimHash 语义匹配（仅在 fuzzySearch=true 时启用）
				simhashMatch := false
				hammingDist := 999 // 默认最大距离

				if fuzzySearch {
					if simhashValue, hasSimhash := task["simhash"]; hasSimhash {
						// 处理 simhash 字段的多种类型（json可能序列化为float64）
						var taskHash uint64
						validHash := false

						switch v := simhashValue.(type) {
						case uint64:
							taskHash = v
							validHash = true
						case float64:
							taskHash = uint64(v)
							validHash = true
						case int64:
							taskHash = uint64(v)
							validHash = true
						case int:
							taskHash = uint64(v)
							validHash = true
						}

						if validHash {
							// 计算汉明距离
							hammingDist = simhash.HammingDistance(queryHash, taskHash)
							// 如果距离在阈值内，视为语义匹配
							if hammingDist <= simhash.SIMHASH_THRESHOLD {
								simhashMatch = true
							}
						}
					}
				}

				// 匹配逻辑：
				// fuzzySearch=true: OR 关系，满足任一条件即加入结果
				// fuzzySearch=false: 仅字符串匹配
				if stringMatch || simhashMatch {
					matchType := "string"
					if stringMatch && simhashMatch {
						matchType = "both" // 同时满足，优先级最高
					} else if simhashMatch {
						matchType = "simhash"
					}

					matches = append(matches, matchedTask{
						task:       task,
						distance:   hammingDist,
						exactMatch: stringMatch,
						matchType:  matchType,
					})
				}
			}

			// 智能排序：优先级 both > string > simhash，同优先级内按距离排序
			for i := 0; i < len(matches); i++ {
				for j := i + 1; j < len(matches); j++ {
					// 计算优先级分数（分数越小越靠前）
					scoreI := 0
					scoreJ := 0

					switch matches[i].matchType {
					case "both":
						scoreI = 0 // 最高优先级：同时满足
					case "string":
						scoreI = 1 // 次优先级：精确字符串匹配
					case "simhash":
						scoreI = 2 // 第三优先级：语义匹配
					}

					switch matches[j].matchType {
					case "both":
						scoreJ = 0
					case "string":
						scoreJ = 1
					case "simhash":
						scoreJ = 2
					}

					// 比较逻辑：先比较类型优先级，再比较汉明距离
					shouldSwap := false
					if scoreI > scoreJ {
						shouldSwap = true
					} else if scoreI == scoreJ {
						// 同类型时，距离小的排前面
						if matches[i].distance > matches[j].distance {
							shouldSwap = true
						}
					}

					if shouldSwap {
						matches[i], matches[j] = matches[j], matches[i]
					}
				}
			}

			// 提取排序后的任务列表
			filteredTasks := make([]map[string]interface{}, len(matches))
			for i, m := range matches {
				filteredTasks[i] = m.task
			}
			taskList = filteredTasks

			// 性能日志（统计匹配类型）
			searchElapsed := time.Since(searchStart)
			bothCount := 0
			stringCount := 0
			simhashCount := 0
			for _, m := range matches {
				switch m.matchType {
				case "both":
					bothCount++
				case "string":
					stringCount++
				case "simhash":
					simhashCount++
				}
			}
			searchMode := "exact"
			if fuzzySearch {
				searchMode = "fuzzy"
			}
			fmt.Printf("INFO: Task search (mode=%s) for query=%q in project %s: matched=%d (both=%d, string=%d, simhash=%d), elapsed=%v\n",
				searchMode, query, projectID, len(matches), bothCount, stringCount, simhashCount, searchElapsed)
		}

		// 如果有时间范围筛选，按updated_at字段过滤
		if timeRange != "" {
			now := time.Now()
			var startTime time.Time

			switch timeRange {
			case "today":
				// 今天：当天0点至23:59:59
				startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			case "week":
				// 本周：周一0点至今天23:59:59
				weekday := int(now.Weekday())
				if weekday == 0 {
					weekday = 7 // 将周日从0调整为7
				}
				daysToMonday := weekday - 1
				startTime = time.Date(now.Year(), now.Month(), now.Day()-daysToMonday, 0, 0, 0, 0, now.Location())
			case "month":
				// 本月：本月1号0点至今天23:59:59
				startTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			default:
				// 无效的时间范围，忽略筛选
				break
			}

			if !startTime.IsZero() {
				filteredTasks := []map[string]interface{}{}
				for _, task := range taskList {
					if updatedAtStr, ok := task["updated_at"].(string); ok {
						// 解析时间字符串（RFC3339格式）
						if updatedAt, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
							// 检查是否在时间范围内
							if updatedAt.After(startTime) || updatedAt.Equal(startTime) {
								filteredTasks = append(filteredTasks, task)
							}
						}
					}
				}
				taskList = filteredTasks
			}
		}

		// 日志记录筛选结果
		if query != "" || timeRange != "" {
			fmt.Printf("INFO: List tasks for project %s, query=%s, time_range=%s, filtered=%d (original=%d)\n",
				projectID, query, timeRange, len(taskList), originalCount)
		}

		// 为每个任务添加完成状态
		for _, task := range taskList {
			if id, ok := task["id"].(string); ok {
				enrichTaskWithCompletionStatus(task, projectID, id)
			}
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": taskList})
	}
}

// HandleCreateProjectTask POST /api/v1/projects/:id/tasks
func HandleCreateProjectTask(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")

		// Verify project exists
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}

		var taskData map[string]interface{}
		if err := c.ShouldBindJSON(&taskData); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}

		// Generate task ID (using nanosecond timestamp to avoid collisions)
		taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())
		taskData["id"] = taskID
		taskData["created_at"] = time.Now().Format(time.RFC3339)
		taskData["updated_at"] = time.Now().Format(time.RFC3339)

		// Set default status if not provided
		if _, ok := taskData["status"]; !ok {
			taskData["status"] = "todo"
		}

		// 预计算 SimHash 指纹（如果有 name 字段）
		if name, ok := taskData["name"].(string); ok && name != "" {
			hash := simhash.CalculateSimHash(name)
			taskData["simhash"] = hash
			fmt.Printf("DEBUG: Calculated simhash for task %s: %d (0x%x)\n", taskID, hash, hash)
		}

		projDir := filepath.Join(projectsRoot(), projectID)
		tasksFile := filepath.Join(projDir, "tasks.json")
		taskDir := filepath.Join(projDir, "tasks", taskID)

		// Create task directory
		if err := os.MkdirAll(taskDir, 0755); err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to create task directory: %w", err))
			return
		}

		// Generate default execution plan template
		repo, err := executionplan.NewFileRepository(projectsRoot(), projectID, taskID)
		if err == nil {
			generator := executionplan.NewTemplateGenerator()
			opts := executionplan.TemplateOptions{Force: false}
			if err := generator.Ensure(c.Request.Context(), repo, taskID, opts); err != nil {
				// Log warning but don't block task creation
				if !errors.Is(err, executionplan.ErrPlanExists) {
					fmt.Printf("[WARN] Failed to generate execution plan template for task %s: %v\n", taskID, err)
				}
			} else {
				fmt.Printf("[INFO] Generated execution plan template for task %s\n", taskID)
			}
		} else {
			fmt.Printf("[WARN] Failed to create execution plan repository for task %s: %v\n", taskID, err)
		}

		// Read existing tasks or create new list
		var taskList []map[string]interface{}
		if data, err := os.ReadFile(tasksFile); err == nil {
			json.Unmarshal(data, &taskList)
		} // Add new task
		taskList = append(taskList, taskData)

		// Save tasks.json
		data, _ := json.MarshalIndent(taskList, "", "  ")
		if err := os.WriteFile(tasksFile, data, 0644); err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to save task: %w", err))
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": taskData})
	}
}

// HandleGetProjectTask GET /api/v1/projects/:id/tasks/:task_id
func HandleGetProjectTask(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		taskID := c.Param("task_id")

		// Verify project exists
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}

		projDir := filepath.Join(projectsRoot(), projectID)
		tasksFile := filepath.Join(projDir, "tasks.json")

		// Read tasks.json
		data, err := os.ReadFile(tasksFile)
		if err != nil {
			notFoundResponse(c, "task not found")
			return
		}

		var taskList []map[string]interface{}
		if err := json.Unmarshal(data, &taskList); err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to parse tasks: %w", err))
			return
		}

		// Find the task
		for _, task := range taskList {
			if task["id"] == taskID {
				enrichTaskWithCompletionStatus(task, projectID, taskID)
				c.JSON(http.StatusOK, gin.H{"success": true, "data": task})
				return
			}
		}

		notFoundResponse(c, "task not found")
	}
}

// HandleUpdateProjectTask PUT /api/v1/projects/:id/tasks/:task_id
func HandleUpdateProjectTask(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		taskID := c.Param("task_id")

		// Verify project exists
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}

		var updateData map[string]interface{}
		if err := c.ShouldBindJSON(&updateData); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}

		projDir := filepath.Join(projectsRoot(), projectID)
		tasksFile := filepath.Join(projDir, "tasks.json")

		// Read tasks.json
		data, err := os.ReadFile(tasksFile)
		if err != nil {
			notFoundResponse(c, "task not found")
			return
		}

		var taskList []map[string]interface{}
		if err := json.Unmarshal(data, &taskList); err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to parse tasks: %w", err))
			return
		}

		// Find and update the task
		found := false
		for i, task := range taskList {
			if task["id"] == taskID {
				// Update fields (don't allow updating id or created_at)
				nameChanged := false
				oldName := ""
				if oldNameValue, ok := task["name"].(string); ok {
					oldName = oldNameValue
				}

				for k, v := range updateData {
					if k != "id" && k != "created_at" {
						task[k] = v
						if k == "name" {
							nameChanged = true
						}
					}
				}
				task["updated_at"] = time.Now().Format(time.RFC3339)

				// 如果 name 改变了，重新计算 SimHash
				if nameChanged {
					if newName, ok := task["name"].(string); ok && newName != "" && newName != oldName {
						hash := simhash.CalculateSimHash(newName)
						task["simhash"] = hash
						fmt.Printf("DEBUG: Recalculated simhash for task %s: %d (0x%x)\n", taskID, hash, hash)
					}
				}

				taskList[i] = task
				found = true
				break
			}
		}

		if !found {
			notFoundResponse(c, "task not found")
			return
		}

		// Save tasks
		data, _ = json.MarshalIndent(taskList, "", "  ")
		if err := os.WriteFile(tasksFile, data, 0644); err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to save task: %w", err))
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// HandleDeleteProjectTask DELETE /api/v1/projects/:id/tasks/:task_id
func HandleDeleteProjectTask(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		taskID := c.Param("task_id")

		// Verify project exists
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}

		projDir := filepath.Join(projectsRoot(), projectID)
		tasksFile := filepath.Join(projDir, "tasks.json")

		// Read tasks.json
		data, err := os.ReadFile(tasksFile)
		if err != nil {
			notFoundResponse(c, "task not found")
			return
		}

		var taskList []map[string]interface{}
		if err := json.Unmarshal(data, &taskList); err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to parse tasks: %w", err))
			return
		}

		// Find and remove the task
		found := false
		newTaskList := []map[string]interface{}{}
		for _, task := range taskList {
			if task["id"] == taskID {
				found = true
				// Don't add this task to the new list
			} else {
				newTaskList = append(newTaskList, task)
			}
		}

		if !found {
			notFoundResponse(c, "task not found")
			return
		}

		// Save tasks
		data, _ = json.MarshalIndent(newTaskList, "", "  ")
		if err := os.WriteFile(tasksFile, data, 0644); err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to save tasks: %w", err))
			return
		}

		// Optionally, delete task directory
		taskDir := filepath.Join(projDir, "tasks", taskID)
		os.RemoveAll(taskDir)

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// HandleGetProjectTaskPrompts GET /api/v1/projects/:id/tasks/:task_id/prompts
func HandleGetProjectTaskPrompts(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		taskID := c.Param("task_id")

		// Verify project exists
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}

		projDir := filepath.Join(projectsRoot(), projectID)
		promptsFile := filepath.Join(projDir, "tasks", taskID, "prompts.json")

		// If prompts.json doesn't exist, return empty array
		if _, err := os.Stat(promptsFile); os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": []map[string]interface{}{}})
			return
		}

		data, err := os.ReadFile(promptsFile)
		if err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to read prompts: %w", err))
			return
		}

		var prompts []map[string]interface{}
		if err := json.Unmarshal(data, &prompts); err != nil {
			// If unmarshal fails, return empty array
			c.JSON(http.StatusOK, gin.H{"success": true, "data": []map[string]interface{}{}})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": prompts})
	}
}

// HandleCreateProjectTaskPrompt POST /api/v1/projects/:id/tasks/:task_id/prompts
func HandleCreateProjectTaskPrompt(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		taskID := c.Param("task_id")

		// Verify project exists
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}

		var prompt map[string]interface{}
		if err := c.ShouldBindJSON(&prompt); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}

		// Add metadata
		prompt["id"] = fmt.Sprintf("prompt_%d", time.Now().Unix())
		prompt["created_at"] = time.Now().Format(time.RFC3339)

		// Ensure required fields exist
		if prompt["username"] == nil {
			prompt["username"] = "unknown"
		}
		if prompt["content"] == nil {
			badRequestResponse(c, "content is required")
			return
		}

		projDir := filepath.Join(projectsRoot(), projectID)
		taskDir := filepath.Join(projDir, "tasks", taskID)
		os.MkdirAll(taskDir, 0755)
		promptsFile := filepath.Join(taskDir, "prompts.json")

		var prompts []map[string]interface{}
		// Load existing prompts
		if data, err := os.ReadFile(promptsFile); err == nil {
			json.Unmarshal(data, &prompts)
		}

		// Add new prompt
		prompts = append(prompts, prompt)

		// Save prompts
		data, _ := json.MarshalIndent(prompts, "", "  ")
		if err := os.WriteFile(promptsFile, data, 0644); err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to save prompt: %w", err))
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": prompt})
	}
}

// ========== Project Deliverables Handlers ==========

// Helper function to get project directory
func getProjectDir(reg *projects.ProjectRegistry, projectID string) (string, error) {
	p := reg.Get(projectID)
	if p == nil {
		return "", fmt.Errorf("project not found")
	}
	dir := filepath.Join(projectsRoot(), projectID)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("directory missing")
	}
	return dir, nil
}

// HandleGetFeatureList GET /api/v1/projects/:id/feature-list
func HandleGetFeatureList(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		dir, err := getProjectDir(reg, projectID)
		if err != nil {
			notFoundResponse(c, "project not found")
			return
		}

		// Try new path first, fallback to old path for backward compatibility
		filePath := filepath.Join(dir, "docs/feature_list.md")
		data, err := os.ReadFile(filePath)
		if err != nil {
			// Fallback to old path
			filePath = filepath.Join(dir, "feature_list.md")
			data, err = os.ReadFile(filePath)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{"content": "", "exists": false})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"content": string(data), "exists": true})
	}
}

// HandleGetFeatureListJSON GET /api/v1/projects/:id/feature-list.json
func HandleGetFeatureListJSON(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		dir, err := getProjectDir(reg, projectID)
		if err != nil {
			notFoundResponse(c, "project not found")
			return
		}

		filePath := filepath.Join(dir, "feature_list.json")
		data, err := os.ReadFile(filePath)
		if err != nil {
			notFoundResponse(c, "feature_list.json not found")
			return
		}

		// Parse JSON to validate and return structured data
		var featureData interface{}
		if err := json.Unmarshal(data, &featureData); err != nil {
			internalErrorResponse(c, fmt.Errorf("invalid JSON format: %w", err))
			return
		}

		c.JSON(http.StatusOK, featureData)
	}
}

// HandlePutFeatureListJSON PUT /api/v1/projects/:id/feature-list.json
func HandlePutFeatureListJSON(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		dir, err := getProjectDir(reg, projectID)
		if err != nil {
			notFoundResponse(c, "project not found")
			return
		}

		var requestBody struct {
			Content interface{} `json:"content" binding:"required"`
		}
		if err := c.ShouldBindJSON(&requestBody); err != nil {
			badRequestResponse(c, "invalid request body: "+err.Error())
			return
		}

		// Validate that content is valid JSON by marshaling
		jsonData, err := json.Marshal(requestBody.Content)
		if err != nil {
			badRequestResponse(c, "invalid JSON content")
			return
		}

		// Create project directory if it doesn't exist
		if err := os.MkdirAll(dir, 0755); err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to create project directory: %w", err))
			return
		}

		// Write to feature_list.json file
		filePath := filepath.Join(dir, "feature_list.json")
		if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to write feature_list.json: %w", err))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":    "feature_list.json updated successfully",
			"project_id": projectID,
			"file_path":  filePath,
		})
	}
}

// HandleGetArchitectureDesign GET /api/v1/projects/:id/architecture-design
func HandleGetArchitectureDesign(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		dir, err := getProjectDir(reg, projectID)
		if err != nil {
			notFoundResponse(c, "project not found")
			return
		}

		// Try new path first, fallback to old path for backward compatibility
		filePath := filepath.Join(dir, "docs/architecture_design.md")
		data, err := os.ReadFile(filePath)
		if err != nil {
			// Fallback to old path
			filePath = filepath.Join(dir, "architecture_new.md")
			data, err = os.ReadFile(filePath)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{"content": "", "exists": false})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"content": string(data), "exists": true})
	}
}

// HandleGetTechDesign GET /api/v1/projects/:id/tech-design
func HandleGetTechDesign(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		dir, err := getProjectDir(reg, projectID)
		if err != nil {
			notFoundResponse(c, "project not found")
			return
		}

		// Try new path first, fallback to old path for backward compatibility
		filePath := filepath.Join(dir, "docs/tech_design.md")
		data, err := os.ReadFile(filePath)
		if err != nil {
			// Fallback to old path (glob pattern)
			files, _ := filepath.Glob(filepath.Join(dir, "tech_design_*.md"))
			if len(files) == 0 {
				c.JSON(http.StatusOK, gin.H{"content": "", "exists": false})
				return
			}
			data, err = os.ReadFile(files[0])
			if err != nil {
				c.JSON(http.StatusOK, gin.H{"content": "", "exists": false})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"content": string(data), "exists": true})
	}
}

// HandleGetLegacyDocument GET /api/v1/projects/:id/legacy-documents/:doc_id
// 获取旧文档系统中的文档内容（用于引用文档）
func HandleGetLegacyDocument(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		docID := c.Param("doc_id")

		dir, err := getProjectDir(reg, projectID)
		if err != nil {
			notFoundResponse(c, "project not found")
			return
		}

		// 读取旧文档路径: {project_dir}/documents/{doc_id}.md
		filePath := filepath.Join(dir, "documents", docID+".md")
		data, err := os.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				notFoundResponse(c, "document not found")
			} else {
				internalErrorResponse(c, fmt.Errorf("failed to read document: %w", err))
			}
			return
		}

		successResponse(c, gin.H{
			"content": string(data),
		})
	}
}
