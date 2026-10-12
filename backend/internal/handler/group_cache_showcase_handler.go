package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type GroupCacheShowcaseHandler struct {
	showcase *service.GroupCacheShowcaseService
}

func NewGroupCacheShowcaseHandler(showcase *service.GroupCacheShowcaseService) *GroupCacheShowcaseHandler {
	return &GroupCacheShowcaseHandler{showcase: showcase}
}
func (h *GroupCacheShowcaseHandler) View(c *gin.Context) {
	// Closing the switch or withdrawing a result takes effect on the next read.
	c.Header("Cache-Control", "no-store")
	view, err := h.showcase.View(c.Request.Context(), time.Now())
	if err != nil {
		response.InternalError(c, "Failed to load cache showcase")
		return
	}
	response.Success(c, view)
}
func (h *GroupCacheShowcaseHandler) GetSettings(c *gin.Context) {
	enabled, err := h.showcase.Enabled(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Failed to load cache showcase settings")
		return
	}
	response.Success(c, gin.H{"enabled": enabled})
}
func (h *GroupCacheShowcaseHandler) UpdateSettings(c *gin.Context) {
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	if c.ShouldBindJSON(&req) != nil || req.Enabled == nil {
		response.BadRequest(c, "enabled is required")
		return
	}
	if err := h.showcase.SetEnabled(c.Request.Context(), *req.Enabled); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	slog.Info("cache showcase visibility updated", "audit", true, "user_id", subject.UserID, "enabled", *req.Enabled)
	response.Success(c, gin.H{"enabled": *req.Enabled})
}
func (h *GroupCacheShowcaseHandler) List(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Query("group_id"), 10, 64)
	if err != nil || groupID <= 0 {
		response.BadRequest(c, "group_id is required")
		return
	}
	items, err := h.showcase.List(c.Request.Context(), groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}
func (h *GroupCacheShowcaseHandler) Publish(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "authentication required")
		return
	}
	var req service.GroupCachePublishRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	if c.ShouldBindJSON(&req) != nil {
		response.BadRequest(c, "invalid publication")
		return
	}
	item, err := h.showcase.Publish(c.Request.Context(), subject.UserID, req, time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	slog.Info("cache test published", "audit", true, "user_id", subject.UserID, "snapshot_id", item.ID, "group_id", item.GroupID)
	response.Success(c, item)
}
func (h *GroupCacheShowcaseHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid publication id")
		return
	}
	if err := h.showcase.Remove(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	slog.Info("cache test withdrawn", "audit", true, "user_id", subject.UserID, "snapshot_id", id)
	response.Success(c, gin.H{"deleted": true})
}
