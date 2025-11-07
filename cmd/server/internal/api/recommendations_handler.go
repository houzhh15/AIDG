package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15/AIDG/pkg/similarity"
)

// PreviewRecommendationsRequest 写作前推荐请求参数
type PreviewRecommendationsRequest struct {
	QueryText string  `json:"query_text"` // 查询文本（任务描述或关键词）
	DocType   string  `json:"doc_type"`   // 可选：过滤文档类型 (requirements/design/test)
	TopK      int     `json:"top_k"`      // Top-K推荐数量，默认5
	Threshold float64 `json:"threshold"`  // 相似度阈值，默认0.6
}

// HandlePreviewRecommendations 处理写作前推荐请求
// POST /api/v1/projects/:id/tasks/:task_id/recommendations/preview
func HandlePreviewRecommendations(getSimService func(string) *similarity.SimilarityService) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")

		// 解析请求体
		var req PreviewRecommendationsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}

		// 验证 query_text
		if req.QueryText == "" {
			badRequestResponse(c, "query_text is required")
			return
		}

		// 限制查询文本长度
		queryRunes := []rune(req.QueryText)
		if len(queryRunes) > 1000 {
			badRequestResponse(c, "query_text exceeds 1000 characters")
			return
		}

		// 设置默认值
		if req.TopK <= 0 {
			req.TopK = 5
		}
		if req.Threshold <= 0 {
			req.Threshold = 0.6
		}

		// 获取项目的 similarity service
		simService := getSimService(projectID)

		// 调用相似度服务
		recommendations, err := simService.GetRecommendationsByQuery(
			context.Background(),
			projectID,
			req.QueryText,
			req.DocType,
			req.TopK,
			req.Threshold,
			"", // excludeTaskID 为空，不排除任何任务
		)
		if err != nil {
			internalErrorResponse(c, err)
			return
		}

		// 返回推荐结果
		c.JSON(http.StatusOK, gin.H{
			"recommendations": recommendations,
		})
	}
}

// LiveRecommendationsRequest 实时推荐请求参数
type LiveRecommendationsRequest struct {
	QueryText     string  `json:"query_text"`      // 查询文本（当前编辑内容）
	DocType       string  `json:"doc_type"`        // 可选：过滤文档类型
	TopK          int     `json:"top_k"`           // Top-K推荐数量，默认3
	Threshold     float64 `json:"threshold"`       // 相似度阈值，默认0.7
	ExcludeTaskID string  `json:"exclude_task_id"` // 可选：排除的任务ID（通常是当前任务）
}

// 注释：移除全局缓存，因为已经移除时间间隔限制
// var (
// 	lastSearchTimeCache sync.Map // key: "projectID:taskID", value: time.Time
// )

// HandleLiveRecommendations 处理实时推荐请求
// POST /api/v1/projects/:id/tasks/:task_id/recommendations/live
func HandleLiveRecommendations(getSimService func(string) *similarity.SimilarityService) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		_ = c.Param("task_id") // 暂时不使用，避免编译警告

		// 解析请求体
		var req LiveRecommendationsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}

		// 智能触发判断1：内容长度检查（≥50字符）
		queryRunes := []rune(req.QueryText)
		if len(queryRunes) < 50 {
			// 不满足条件，返回空数组（非错误）
			c.JSON(http.StatusOK, gin.H{
				"recommendations": []interface{}{},
				"reason":          "query_text too short (< 50 characters)",
			})
			return
		}

		// 注释掉时间间隔检查：前端已有防抖机制（3秒），后端10秒限制过于严格
		// cacheKey := fmt.Sprintf("%s:%s", projectID, taskID)
		// now := time.Now()
		// if lastTime, ok := lastSearchTimeCache.Load(cacheKey); ok {
		// 	if lastTimeTyped, ok := lastTime.(time.Time); ok {
		// 		elapsed := now.Sub(lastTimeTyped).Seconds()
		// 		if elapsed < 10 {
		// 			// 不满足条件，返回空数组
		// 			c.JSON(http.StatusOK, gin.H{
		// 				"recommendations": []interface{}{},
		// 				"reason":          fmt.Sprintf("too frequent (elapsed: %.1fs < 10s)", elapsed),
		// 			})
		// 			return
		// 		}
		// 	}
		// }

		// 设置默认值
		if req.TopK <= 0 {
			req.TopK = 3 // 实时推荐默认返回3个
		}
		if req.Threshold <= 0 {
			req.Threshold = 0.7 // 实时推荐使用更高阈值
		}

		// 获取 similarity service
		simService := getSimService(projectID)

		// 调用相似度服务
		recommendations, err := simService.GetRecommendationsByQuery(
			context.Background(),
			projectID,
			req.QueryText,
			req.DocType,
			req.TopK,
			req.Threshold,
			req.ExcludeTaskID,
		)
		if err != nil {
			internalErrorResponse(c, err)
			return
		}

		// 注释：移除缓存更新，因为已经移除时间间隔限制
		// lastSearchTimeCache.Store(cacheKey, now)

		// 返回推荐结果
		c.JSON(http.StatusOK, gin.H{
			"recommendations": recommendations,
		})
	}
}
