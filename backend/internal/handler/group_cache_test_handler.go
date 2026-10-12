package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type GroupCacheTestHandler struct {
	svc *service.GroupCacheTestService
}

func NewGroupCacheTestHandler(svc *service.GroupCacheTestService) *GroupCacheTestHandler {
	return &GroupCacheTestHandler{svc: svc}
}
func (h *GroupCacheTestHandler) ListPlans(c *gin.Context) {
	items, err := h.svc.ListPlans(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}
func (h *GroupCacheTestHandler) SavePlan(c *gin.Context) {
	var id int64
	var err error
	if c.Param("id") != "" {
		id, err = strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "invalid plan id")
			return
		}
	}
	var input service.GroupCacheTestPlanInput
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	if c.ShouldBindJSON(&input) != nil {
		response.BadRequest(c, "invalid cache test plan")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	plan, err := h.svc.SavePlan(c.Request.Context(), id, subject.UserID, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	slog.Info("cache test plan saved", "audit", true, "user_id", subject.UserID, "plan_id", plan.ID)
	response.Success(c, plan)
}
func (h *GroupCacheTestHandler) DeletePlan(c *gin.Context) {
	id, ok := cacheTestID(c)
	if !ok {
		return
	}
	if err := h.svc.DeletePlan(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	slog.Info("cache test plan deleted", "audit", true, "user_id", subject.UserID, "plan_id", id)
	response.Success(c, gin.H{"deleted": true})
}
func (h *GroupCacheTestHandler) RunPlan(c *gin.Context) {
	id, ok := cacheTestID(c)
	if !ok {
		return
	}
	if err := h.svc.RunNow(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	slog.Info("cache test manual run", "audit", true, "user_id", subject.UserID, "plan_id", id)
	response.Success(c, gin.H{"started": true})
}
func (h *GroupCacheTestHandler) ListResults(c *gin.Context) {
	page, size := response.ParsePagination(c)
	planID := int64(0)
	if value := c.Query("plan_id"); value != "" {
		id, e := strconv.ParseInt(value, 10, 64)
		if e != nil || id < 0 {
			response.BadRequest(c, "invalid plan id")
			return
		}
		planID = id
	}
	items, total, err := h.svc.ListResults(c.Request.Context(), planID, page, size)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, size)
}
func (h *GroupCacheTestHandler) GetResult(c *gin.Context) {
	id, ok := cacheTestID(c)
	if !ok {
		return
	}
	item, e := h.svc.GetResult(c.Request.Context(), id)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, item)
}
func cacheTestID(c *gin.Context) (int64, bool) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil || id <= 0 {
		response.BadRequest(c, "invalid cache test id")
		return 0, false
	}
	return id, true
}
