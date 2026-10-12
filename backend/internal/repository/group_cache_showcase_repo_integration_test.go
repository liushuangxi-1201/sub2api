//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupCacheShowcaseRepo_WindowVisibilityAndWithdrawal(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("cache-showcase-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleAdmin, Status: service.StatusActive})
	group := mustCreateGroup(t, client, &service.Group{Name: "Cache showcase fixture"})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM groups WHERE id=$1`, group.ID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM users WHERE id=$1`, user.ID)
	})
	repo := NewGroupCacheShowcaseRepository(integrationDB)
	now := time.Now().UTC()
	latency := 4700
	round := func(id int64, index int, input, read int64, when time.Time, ttft *int) service.GroupCacheSnapshotRound {
		return service.GroupCacheSnapshotRound{UsageLogID: id, Index: index, InputTokens: input, CacheReadTokens: read, MeasuredAt: when, FirstTokenMs: ttft}
	}
	snapshot := &service.GroupCacheSnapshot{RunID: "window", CreatedBy: user.ID, GroupID: group.ID, Model: "demo-model", Protocol: "responses", TestedAt: now.Add(-time.Minute), Rounds: []service.GroupCacheSnapshotRound{
		round(1, 1, 1000000, 0, now.Add(-25*time.Hour), &latency),             // baseline is excluded
		round(2, 2, 1000000, 0, now.Add(-24*time.Hour-time.Second), &latency), // too old
		round(3, 3, 1000, 922, now.Add(-time.Minute), &latency),
		round(4, 4, 3000, 2100, now.Add(-time.Minute), nil),
	}}
	require.NoError(t, repo.Publish(ctx, snapshot))
	require.Positive(t, snapshot.ID)
	items, err := repo.List(ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	summary, err := repo.Summary(ctx, now.Add(-24*time.Hour), now)
	require.NoError(t, err)
	var metric *service.GroupCacheShowcaseItem
	for _, item := range summary {
		if item.GroupID == group.ID {
			metric = item
		}
	}
	require.NotNil(t, metric)
	require.EqualValues(t, 2, metric.Samples)
	require.EqualValues(t, 1, metric.LatencySamples)
	require.EqualValues(t, 4000, metric.InputTokens)
	require.EqualValues(t, 3022, metric.CacheReadTokens)
	require.InDelta(t, 0.7555, *metric.CacheHitRate, 0.00001, "weighted by tokens, never average per-request percentages")
	require.InDelta(t, 4700, *metric.FirstTokenMs, 0.001)

	duplicate := *snapshot
	duplicate.RunID = "duplicate"
	require.Error(t, repo.Publish(ctx, &duplicate), "a usage record cannot be counted twice")
	items, err = repo.List(ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, items, 1, "partial duplicate transaction rolled back")

	_, err = integrationDB.ExecContext(ctx, `UPDATE groups SET status=$1 WHERE id=$2`, service.StatusDisabled, group.ID)
	require.NoError(t, err)
	summary, err = repo.Summary(ctx, now.Add(-24*time.Hour), now)
	require.NoError(t, err)
	for _, item := range summary {
		require.NotEqual(t, group.ID, item.GroupID, "disabled groups hidden")
	}
	_, err = integrationDB.ExecContext(ctx, `UPDATE groups SET status=$1 WHERE id=$2`, service.StatusActive, group.ID)
	require.NoError(t, err)

	summary, err = repo.Summary(ctx, now.Add(time.Minute), now.Add(24*time.Hour))
	require.NoError(t, err)
	for _, item := range summary {
		if item.GroupID == group.ID {
			require.Zero(t, item.Samples)
			require.Nil(t, item.CacheHitRate)
			require.Nil(t, item.FirstTokenMs)
		}
	}
	deleted, err := repo.Delete(ctx, snapshot.ID)
	require.NoError(t, err)
	require.True(t, deleted)
	items, err = repo.List(ctx, group.ID)
	require.NoError(t, err)
	require.Empty(t, items)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM group_cache_showcase_rounds WHERE run_id=$1`, snapshot.ID).Scan(&count))
	require.Zero(t, count)
}

func TestGroupCacheShowcaseRepo_RetentionAndRepublish(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("cache-retention-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleAdmin, Status: service.StatusActive})
	group := mustCreateGroup(t, client, &service.Group{Name: "Cache retention fixture"})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM groups WHERE id=$1`, group.ID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM users WHERE id=$1`, user.ID)
	})
	repo := NewGroupCacheShowcaseRepository(integrationDB)
	now := time.Now().UTC()
	for i := int64(1); i <= 101; i++ {
		snapshot := &service.GroupCacheSnapshot{RunID: fmt.Sprint(i), CreatedBy: user.ID, GroupID: group.ID, Model: "demo", Protocol: "messages", TestedAt: now.Add(time.Duration(i) * time.Second), Rounds: []service.GroupCacheSnapshotRound{{Index: 1, UsageLogID: 10000 + i, InputTokens: 1000, MeasuredAt: now}}}
		require.NoError(t, repo.Publish(ctx, snapshot))
	}
	items, err := repo.List(ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, items, 100)
	require.Equal(t, "101", items[0].RunID)
	require.Equal(t, "2", items[99].RunID)
	deleted, err := repo.Delete(ctx, items[0].ID)
	require.NoError(t, err)
	require.True(t, deleted)
	require.NoError(t, repo.Publish(ctx, &service.GroupCacheSnapshot{RunID: "101", CreatedBy: user.ID, GroupID: group.ID, Model: "demo", Protocol: "messages", TestedAt: now, Rounds: []service.GroupCacheSnapshotRound{{Index: 1, UsageLogID: 10101, InputTokens: 1000, MeasuredAt: now}}}))
}
