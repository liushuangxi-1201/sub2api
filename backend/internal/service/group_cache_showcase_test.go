package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type cacheShowcaseRepoStub struct {
	GroupCacheShowcaseRepository
	saved        *GroupCacheSnapshot
	summaryCalls int
}

func (r *cacheShowcaseRepoStub) Publish(_ context.Context, item *GroupCacheSnapshot) error {
	r.saved = item
	return nil
}
func (r *cacheShowcaseRepoStub) Summary(_ context.Context, _, _ time.Time) ([]*GroupCacheShowcaseItem, error) {
	r.summaryCalls++
	return []*GroupCacheShowcaseItem{{GroupID: 7, GroupName: "Public group"}}, nil
}

type cacheShowcaseUsageStub struct {
	UsageLogRepository
	logs map[int64]*UsageLog
}

func (r *cacheShowcaseUsageStub) GetByID(_ context.Context, id int64) (*UsageLog, error) {
	return r.logs[id], nil
}

type cacheShowcaseGroupStub struct {
	GroupRepository
	group *Group
}

func (r *cacheShowcaseGroupStub) GetByID(context.Context, int64) (*Group, error) { return r.group, nil }
func cacheShowcaseFixture() (*GroupCacheShowcaseService, *cacheShowcaseRepoStub, *cacheShowcaseUsageStub, GroupCachePublishRequest, time.Time) {
	now := time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)
	groupID, latency, endpoint := int64(7), 4700, "/v1/responses"
	usage := &cacheShowcaseUsageStub{logs: map[int64]*UsageLog{}}
	req := GroupCachePublishRequest{RunID: "test-run", GroupID: groupID, APIKeyID: 10, Model: "demo-model", Protocol: "responses"}
	for i := int64(1); i <= 3; i++ {
		requestID := "client:round-" + string(rune('0'+i))
		usage.logs[i] = &UsageLog{ID: i, UserID: 1, APIKeyID: 10, GroupID: &groupID, RequestID: requestID, Model: "mapped-model", RequestedModel: "demo-model", InboundEndpoint: &endpoint, Stream: true, InputTokens: 78, CacheReadTokens: 922, OutputTokens: 1, FirstTokenMs: &latency, CreatedAt: now.Add(time.Duration(i-4) * time.Minute)}
		req.Rounds = append(req.Rounds, GroupCachePublishRound{UsageLogID: i, RequestID: requestID})
	}
	repo := &cacheShowcaseRepoStub{}
	settings := &showcaseRuntimeSettingRepo{values: map[string]string{}}
	return NewGroupCacheShowcaseService(repo, &SettingService{settingRepo: settings}, usage, &cacheShowcaseGroupStub{group: &Group{ID: groupID, Name: "Demo", Status: StatusActive}}), repo, usage, req, now
}
func TestGroupCacheShowcasePublicationUsesServerMeasurements(t *testing.T) {
	svc, repo, usage, req, now := cacheShowcaseFixture()
	usage.logs[2].CacheCreationTokens = 100
	usage.logs[3].FirstTokenMs = nil
	item, err := svc.Publish(context.Background(), 1, req, now)
	require.NoError(t, err)
	require.Same(t, item, repo.saved)
	require.Len(t, item.Rounds, 3)
	require.EqualValues(t, 1100, item.Rounds[1].InputTokens)
	require.EqualValues(t, 922, item.Rounds[1].CacheReadTokens)
	require.Nil(t, item.Rounds[2].FirstTokenMs)
	require.Equal(t, usage.logs[3].CreatedAt, item.TestedAt)
	enabled, err := svc.Enabled(context.Background())
	require.NoError(t, err)
	require.False(t, enabled, "publishing must not open the showcase")
}
func TestGroupCacheShowcaseRejectsUnverifiablePublications(t *testing.T) {
	cases := map[string]func(*GroupCacheShowcaseService, *cacheShowcaseUsageStub, *GroupCachePublishRequest){
		"other user": func(_ *GroupCacheShowcaseService, u *cacheShowcaseUsageStub, _ *GroupCachePublishRequest) {
			u.logs[2].UserID = 2
		},
		"other key": func(_ *GroupCacheShowcaseService, u *cacheShowcaseUsageStub, _ *GroupCachePublishRequest) {
			u.logs[2].APIKeyID = 11
		},
		"other group": func(_ *GroupCacheShowcaseService, u *cacheShowcaseUsageStub, _ *GroupCachePublishRequest) {
			id := int64(8)
			u.logs[2].GroupID = &id
		},
		"missing group": func(_ *GroupCacheShowcaseService, u *cacheShowcaseUsageStub, _ *GroupCachePublishRequest) {
			u.logs[2].GroupID = nil
		},
		"other model": func(_ *GroupCacheShowcaseService, u *cacheShowcaseUsageStub, _ *GroupCachePublishRequest) {
			u.logs[2].RequestedModel = "other"
		},
		"other endpoint": func(_ *GroupCacheShowcaseService, u *cacheShowcaseUsageStub, _ *GroupCachePublishRequest) {
			e := "/v1/messages"
			u.logs[2].InboundEndpoint = &e
		},
		"absent endpoint": func(_ *GroupCacheShowcaseService, u *cacheShowcaseUsageStub, _ *GroupCachePublishRequest) {
			u.logs[2].InboundEndpoint = nil
		},
		"nonstream": func(_ *GroupCacheShowcaseService, u *cacheShowcaseUsageStub, _ *GroupCachePublishRequest) {
			u.logs[2].Stream = false
		},
		"no output": func(_ *GroupCacheShowcaseService, u *cacheShowcaseUsageStub, _ *GroupCachePublishRequest) {
			u.logs[2].OutputTokens = 0
		},
		"bad request id": func(_ *GroupCacheShowcaseService, _ *cacheShowcaseUsageStub, r *GroupCachePublishRequest) {
			r.Rounds[1].RequestID = "client:other"
		},
		"missing bill": func(_ *GroupCacheShowcaseService, u *cacheShowcaseUsageStub, _ *GroupCachePublishRequest) {
			delete(u.logs, 2)
		},
		"duplicate": func(_ *GroupCacheShowcaseService, _ *cacheShowcaseUsageStub, r *GroupCachePublishRequest) {
			r.Rounds[1] = r.Rounds[0]
		},
		"one round": func(_ *GroupCacheShowcaseService, _ *cacheShowcaseUsageStub, r *GroupCachePublishRequest) {
			r.Rounds = r.Rounds[:1]
		},
		"negative tokens": func(_ *GroupCacheShowcaseService, u *cacheShowcaseUsageStub, _ *GroupCachePublishRequest) {
			u.logs[2].InputTokens = -1
		},
		"negative latency": func(_ *GroupCacheShowcaseService, u *cacheShowcaseUsageStub, _ *GroupCachePublishRequest) {
			n := -1
			u.logs[2].FirstTokenMs = &n
		},
		"old result": func(_ *GroupCacheShowcaseService, u *cacheShowcaseUsageStub, _ *GroupCachePublishRequest) {
			u.logs[1].CreatedAt = u.logs[1].CreatedAt.Add(-8 * 24 * time.Hour)
		},
		"future result": func(_ *GroupCacheShowcaseService, u *cacheShowcaseUsageStub, _ *GroupCachePublishRequest) {
			u.logs[3].CreatedAt = u.logs[3].CreatedAt.Add(time.Hour)
		},
		"out of order": func(_ *GroupCacheShowcaseService, _ *cacheShowcaseUsageStub, r *GroupCachePublishRequest) {
			r.Rounds[0], r.Rounds[1] = r.Rounds[1], r.Rounds[0]
		},
		"inactive group": func(s *GroupCacheShowcaseService, _ *cacheShowcaseUsageStub, _ *GroupCachePublishRequest) {
			s.groups.(*cacheShowcaseGroupStub).group.Status = StatusDisabled
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo, usage, req, now := cacheShowcaseFixture()
			change(svc, usage, &req)
			_, err := svc.Publish(context.Background(), 1, req, now)
			require.Error(t, err)
			require.Nil(t, repo.saved)
		})
	}
}
func TestGroupCacheShowcaseSwitchFailsClosedAndHidesRecords(t *testing.T) {
	svc, repo, _, _, now := cacheShowcaseFixture()
	ctx := context.Background()
	updates := 0
	svc.onUpdate = func() { updates++ }
	view, err := svc.View(ctx, now)
	require.NoError(t, err)
	require.False(t, view.Enabled)
	require.Empty(t, view.Items)
	require.Zero(t, repo.summaryCalls)
	require.NoError(t, svc.SetEnabled(ctx, true))
	view, err = svc.View(ctx, now)
	require.NoError(t, err)
	require.Len(t, view.Items, 1)
	require.Equal(t, now.Add(-24*time.Hour), view.WindowStart)
	raw, err := json.Marshal(view)
	require.NoError(t, err)
	for _, field := range []string{"created_by", "run_id", "usage_log_id", "request_id", "account_id", "api_key", "cost"} {
		require.NotContains(t, string(raw), field)
	}
	require.NoError(t, svc.SetEnabled(ctx, false))
	require.Equal(t, 2, updates, "switch changes invalidate injected public settings")
	view, err = svc.View(ctx, now)
	require.NoError(t, err)
	require.Empty(t, view.Items)
	svc.settings.(*showcaseRuntimeSettingRepo).readErr = errors.New("offline")
	view, err = svc.View(ctx, now)
	require.Error(t, err)
	require.Nil(t, view)
}
