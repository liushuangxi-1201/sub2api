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
	settings, err := h.showcase.Settings(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Failed to load cache showcase settings")
		return
	}
	response.Success(c, settings)
}
func (h *GroupCacheShowcaseHandler) UpdateSettings(c *gin.Context) {
	var req struct {
		Enabled       *bool `json:"enabled"`
		APIEnabled    *bool `json:"api_enabled"`
		MaxItems      *int  `json:"max_items"`
		AutoCleanup   *bool `json:"auto_cleanup"`
		RetentionDays *int  `json:"retention_days"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	if c.ShouldBindJSON(&req) != nil || req.Enabled == nil {
		response.BadRequest(c, "enabled is required")
		return
	}
	settings, err := h.showcase.Settings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	settings.Enabled = *req.Enabled
	if req.APIEnabled != nil {
		settings.APIEnabled = *req.APIEnabled
	}
	if req.MaxItems != nil {
		if *req.MaxItems < 1 || *req.MaxItems > 100 {
			response.BadRequest(c, "max_items must be 1–100")
			return
		}
		settings.MaxItems = *req.MaxItems
	}
	if req.AutoCleanup != nil {
		settings.AutoCleanup = *req.AutoCleanup
	}
	if req.RetentionDays != nil {
		if *req.RetentionDays < 1 || *req.RetentionDays > 90 {
			response.BadRequest(c, "retention_days must be 1–90")
			return
		}
		settings.RetentionDays = *req.RetentionDays
	}
	saved, err := h.showcase.UpdateSettings(c.Request.Context(), settings)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	slog.Info("cache showcase settings updated", "audit", true, "user_id", subject.UserID, "enabled", settings.Enabled, "api_enabled", settings.APIEnabled)
	response.Success(c, saved)
}
func (h *GroupCacheShowcaseHandler) PublicView(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	settings, err := h.showcase.Settings(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Failed to load cache showcase settings")
		return
	}
	if !settings.Enabled || !settings.APIEnabled {
		response.Forbidden(c, "Cache showcase API is disabled")
		return
	}
	h.View(c)
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
