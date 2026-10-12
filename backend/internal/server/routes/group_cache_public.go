package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterPublicGroupCacheShowcaseRoutes(v1 *gin.RouterGroup, h *handler.Handlers, auth middleware.APIKeyAuthMiddleware, limiter *middleware.PanelRateLimiter) {
	g := v1.Group("/public/group-cache-showcase")
	g.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Writer.Header().Add("Vary", "Authorization, X-API-Key, X-Goog-API-Key")
		c.Next()
	})
	g.Use(limiter.PublicIP(), gin.HandlerFunc(auth), limiter.Global())
	g.GET("", h.GroupCacheShowcase.PublicView)
	g.HEAD("", h.GroupCacheShowcase.PublicView)
}
