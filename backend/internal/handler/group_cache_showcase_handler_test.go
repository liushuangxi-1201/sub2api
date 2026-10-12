package handler

import (
	"context"
	"encoding/json"
	"errors"
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
	err    error
}

func (s *cacheHandlerSettings) GetMultiple(context.Context, []string) (map[string]string, error) {
	return s.values, s.err
}

func TestGroupCacheShowcasePublicAPIRequiresBothSavedSwitches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := &cacheHandlerSettings{values: map[string]string{}}
	repo := &cacheHandlerRepo{}
	h := NewGroupCacheShowcaseHandler(service.NewGroupCacheShowcaseService(repo, service.NewSettingService(settings, &config.Config{}), nil, nil))
	request := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/group-cache-showcase", nil)
		h.PublicView(c)
		require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
		return w
	}
	for _, flags := range [][2]string{{"false", "false"}, {"true", "false"}, {"false", "true"}} {
		settings.values[service.SettingKeyGroupCacheShowcaseEnabled] = flags[0]
		settings.values[service.SettingKeyGroupCacheShowcaseAPIEnabled] = flags[1]
		require.Equal(t, http.StatusForbidden, request().Code)
	}
	require.Zero(t, repo.calls)
	settings.values[service.SettingKeyGroupCacheShowcaseEnabled] = "true"
	settings.values[service.SettingKeyGroupCacheShowcaseAPIEnabled] = "true"
	w := request()
	require.Equal(t, http.StatusOK, w.Code)
	var payload struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Items, 1)
	for _, field := range []string{"created_by", "account_id", "account_name", "api_key_id", "usage_log_id", "request_id", "prompt", "cost_usd", "run_id", "plan_id"} {
		require.NotContains(t, payload.Data.Items[0], field)
	}
	settings.err = errors.New("unavailable setting store")
	require.Equal(t, http.StatusInternalServerError, request().Code)
	require.Equal(t, 1, repo.calls)
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

func (s *cacheHandlerRepo) Summary(context.Context, time.Time, time.Time, int, time.Time) ([]*service.GroupCacheShowcaseItem, error) {
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
