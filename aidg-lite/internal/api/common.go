package api

import (
	"github.com/gin-gonic/gin"
)

// currentUser 获取当前用户
// 简化实现：若后续有鉴权中间件注入用户名，可在 context 中读取
// 当前返回固定占位符，避免空字符串
func currentUser(c *gin.Context) string {
	// 优先从 context 中获取用户信息 (由认证中间件设置)
	if user, exists := c.Get("user"); exists {
		if username, ok := user.(string); ok && username != "" {
			return username
		}
	}

	// 其次从 Header 中读取
	if u := c.GetHeader("X-User"); u != "" {
		return u
	}

	// 默认返回 system (避免空字符串)
	return "system"
}

// errorResponse 返回错误响应
func errorResponse(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{
		"error": message,
	})
}

// errorResponseWithDetail 返回带详情的错误响应
func errorResponseWithDetail(c *gin.Context, code int, message string, detail interface{}) {
	c.JSON(code, gin.H{
		"error":  message,
		"detail": detail,
	})
}

// successResponse 返回成功响应
func successResponse(c *gin.Context, data interface{}) {
	c.JSON(200, data)
}

// successResponseWithMessage 返回带消息的成功响应
func successResponseWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(200, gin.H{
		"message": message,
		"data":    data,
	})
}

// notFoundResponse 返回 404 响应
func notFoundResponse(c *gin.Context, resource string) {
	c.JSON(404, gin.H{
		"error": resource + " not found",
	})
}

// badRequestResponse 返回 400 响应
func badRequestResponse(c *gin.Context, message string) {
	c.JSON(400, gin.H{
		"error": message,
	})
}

// unauthorizedResponse 返回 401 响应
func unauthorizedResponse(c *gin.Context, message string) {
	if message == "" {
		message = "unauthorized"
	}
	c.JSON(401, gin.H{
		"error": message,
	})
}

// forbiddenResponse 返回 403 响应
func forbiddenResponse(c *gin.Context, message string) {
	if message == "" {
		message = "forbidden"
	}
	c.JSON(403, gin.H{
		"error": message,
	})
}

// internalErrorResponse 返回 500 响应
func internalErrorResponse(c *gin.Context, err error) {
	c.JSON(500, gin.H{
		"error":  "internal server error",
		"detail": err.Error(),
	})
}

// validationErrorResponse 返回验证错误响应
func validationErrorResponse(c *gin.Context, errors map[string]string) {
	c.JSON(400, gin.H{
		"error":  "validation failed",
		"fields": errors,
	})
}
