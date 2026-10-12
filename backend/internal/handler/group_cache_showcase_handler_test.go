package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type cacheHandlerSettings struct {
	service.SettingRepository
	values map[string]string
}

func (s *cacheHandlerSettings) GetMultiple(context.Context, []string) (map[string]string, error) {
	return s.values, nil
}
func (s *cacheHandlerSettings) SetMultiple(_ context.Context, values map[string]string) error {
	for k, v := range values {
		s.values[k] = v
	}
	return nil
}

type cacheHandlerRepo struct {
	service.GroupCacheShowcaseRepository
	calls int
}

func (s *cacheHandlerRepo) Summary(context.Context, time.Time, time.Time) ([]*service.GroupCacheShowcaseItem, error) {
	s.calls++
	return []*service.GroupCacheShowcaseItem{{GroupID: 7, GroupName: "Demo"}}, nil
}

func TestGroupCacheShowcaseHandlerVisibilityAndInputBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := &cacheHandlerSettings{values: map[string]string{}}
	repo := &cacheHandlerRepo{}
	h := NewGroupCacheShowcaseHandler(service.NewGroupCacheShowcaseService(repo, service.NewSettingService(settings, &config.Config{}), nil, nil))
	request := func(method, body string, handle gin.HandlerFunc) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(method, "/test", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		handle(c)
		return w
	}
	w := request(http.MethodGet, "", h.View)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	require.Zero(t, repo.calls)
	for _, body := range []string{`{}`, `{"enabled":"yes"}`, `{"enabled":true,"extra":"` + strings.Repeat("x", 1200) + `"}`} {
		w = request(http.MethodPut, body, h.UpdateSettings)
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
	w = request(http.MethodPut, `{"enabled":true}`, h.UpdateSettings)
	require.Equal(t, http.StatusOK, w.Code)
	w = request(http.MethodGet, "", h.View)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, repo.calls)
	var payload struct {
		Data service.GroupCacheShowcaseView `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Items, 1)
	w = request(http.MethodPut, `{"enabled":false}`, h.UpdateSettings)
	require.Equal(t, http.StatusOK, w.Code)
	w = request(http.MethodGet, "", h.View)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Empty(t, payload.Data.Items)
	w = request(http.MethodPost, `{}`, h.Publish)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	w = request(http.MethodGet, "", h.List)
	require.Equal(t, http.StatusBadRequest, w.Code)
}
