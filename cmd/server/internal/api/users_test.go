package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/houzhh15/AIDG/cmd/server/internal/users"
)

func TestHandleCreateUser_UsesDefaultPasswordWhenOmitted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userManager, err := users.NewManager(t.TempDir(), []byte("test-secret"))
	require.NoError(t, err)

	body, err := json.Marshal(map[string]any{
		"username": "new-user",
		"scopes":   []string{},
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	HandleCreateUser(userManager)(c)

	assert.Equal(t, http.StatusOK, w.Code)
	_, err = userManager.Authenticate("new-user", "neteye@123")
	require.NoError(t, err)
}
