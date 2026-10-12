package routes

import (
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGroupCacheShowcaseRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &handler.Handlers{GroupCacheShowcase: handler.NewGroupCacheShowcaseHandler(nil), GroupCacheTests: handler.NewGroupCacheTestHandler(nil)}
	// Use the actual user route registration and actual auth middleware; a nil
	// service would panic if any unauthenticated request reached the handler.
	RegisterUserRoutes(r.Group("/api/v1"), h, middleware.NewJWTAuthMiddleware(nil, nil, nil, nil), func(c *gin.Context) { c.Next() }, nil, nil)
	admin := r.Group("/api/v1/admin", gin.HandlerFunc(middleware.NewAdminAuthMiddleware(nil, nil, nil, nil)))
	registerGroupCacheShowcaseRoutes(admin, h)
	RegisterPublicGroupCacheShowcaseRoutes(r.Group("/api/v1"), h, middleware.NewAPIKeyAuthMiddleware(nil, nil, nil), nil)
	for _, tc := range []struct {
		method, path string
		code         int
	}{
		{"GET", "/api/v1/group-cache-showcase", 401},
		{"GET", "/api/v1/admin/group-cache-showcase/settings", 401},
		{"PUT", "/api/v1/admin/group-cache-showcase/settings", 401},
		{"GET", "/api/v1/admin/group-cache-showcase/items?group_id=7", 401},
		{"POST", "/api/v1/admin/group-cache-showcase/items", 401},
		{"DELETE", "/api/v1/admin/group-cache-showcase/items/1", 401},
		{"POST", "/api/v1/group-cache-showcase", 404},
		{"GET", "/api/v1/public/group-cache-showcase", 401},
		{"POST", "/api/v1/admin/group-cache-tests", 401},
		{"POST", "/api/v1/admin/group-cache-tests/1/run", 401},
		{"PUT", "/api/v1/admin/group-cache-tests/1", 401},
		{"DELETE", "/api/v1/admin/group-cache-tests/1", 401},
		{"GET", "/api/v1/admin/group-cache-test-results", 401},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			require.Equal(t, tc.code, w.Code)
		})
	}
}
