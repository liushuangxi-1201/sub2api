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

func TestGroupCachePlansRepo_LeasesCostsAndRetention(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("cache-plans-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleAdmin, Status: service.StatusActive})
	group := mustCreateGroup(t, client, &service.Group{Name: "Cache plan fixture", Platform: service.PlatformOpenAI, Status: service.StatusActive})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id=$1`, group.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, user.ID)
	})
	plans := NewGroupCacheTestRepository(integrationDB)
	showcase := NewGroupCacheShowcaseRepository(integrationDB)
	now := time.Now().UTC().Truncate(time.Microsecond)
	due := now.Add(-time.Minute)
	cfg := service.GroupCacheTestConfig{Prompt: service.GroupCacheDefaultPrompt, ReasoningEffort: "minimal", ParallelCount: 1, Rounds: 3, PrefixBlocks: 32, MaxOutputTokens: 256}
	plan, err := plans.SavePlan(ctx, &service.GroupCacheTestPlan{GroupID: group.ID, CreatedBy: user.ID, ModelID: "fixture-model", CronExpression: "*/30 * * * *", Enabled: true, Config: cfg, NextRunAt: &due})
	require.NoError(t, err)
	require.Equal(t, group.Name, plan.GroupName)
	require.Equal(t, user.ID, plan.CreatedBy)
	duePlans, err := plans.ListDue(ctx, now)
	require.NoError(t, err)
	found := false
	for _, p := range duePlans {
		found = found || p.ID == plan.ID
	}
	require.True(t, found)
	until := now.Add(15 * time.Minute)
	next := now.Add(30 * time.Minute)
	claimed, err := plans.Claim(ctx, plan, now, until, &next)
	require.NoError(t, err)
	require.True(t, claimed)
	claimed, err = plans.Claim(ctx, plan, now, until, nil)
	require.NoError(t, err)
	require.False(t, claimed, "manual and scheduled requests share the lease")
	require.NoError(t, plans.Finish(ctx, plan.ID, until.Add(time.Second), now))
	current, err := plans.GetPlan(ctx, plan.ID)
	require.NoError(t, err)
	require.NotNil(t, current.RunningUntil, "stale finish cannot release current run")
	require.NoError(t, plans.Finish(ctx, plan.ID, until, now))
	current, err = plans.GetPlan(ctx, plan.ID)
	require.NoError(t, err)
	require.Nil(t, current.RunningUntil)
	latency := 4700
	for i := 0; i < 3; i++ {
		item := &service.GroupCacheSnapshot{RunID: fmt.Sprintf("run-%d", i), CreatedBy: user.ID, PlanID: plan.ID, GroupID: group.ID, Model: plan.ModelID, Protocol: "responses", Source: "upstream", TestedAt: now.Add(time.Duration(i) * time.Second), Rounds: []service.GroupCacheSnapshotRound{{Index: 1, InputTokens: 1000, MeasuredAt: now}, {Index: 2, InputTokens: 1000, CacheReadTokens: 922, FirstTokenMs: &latency, MeasuredAt: now}}}
		require.NoError(t, showcase.Publish(ctx, item), "scheduled samples do not need usage-log identities")
		cost := 0.5
		require.NoError(t, plans.SaveResult(ctx, &service.GroupCacheTestResult{PlanID: plan.ID, GroupID: group.ID, GroupName: group.Name, ModelID: plan.ModelID, Config: cfg, Status: "success", CostUSD: &cost, StartedAt: now, FinishedAt: now, PublicationID: &item.ID, Rounds: []service.GroupCacheTestRound{{Index: 1, Status: "success"}}}))
	}
	current, err = plans.GetPlan(ctx, plan.ID)
	require.NoError(t, err)
	require.InDelta(t, 1.5, current.TotalCostUSD, .00001)
	require.NotNil(t, current.LastResult)
	require.NotNil(t, current.LastResult.PublicationID)
	results, total, err := plans.ListResults(ctx, plan.ID, 0, 2)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, results, 2)
	require.Len(t, results[0].Rounds, 1)
	saved, err := plans.GetResult(ctx, results[0].ID)
	require.NoError(t, err)
	require.Equal(t, results[0].ID, saved.ID)
	require.NoError(t, showcase.Prune(ctx, 2, time.Time{}))
	items, err := showcase.List(ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, items, 2)
	summary, err := showcase.Summary(ctx, now.Add(-24*time.Hour), now.Add(time.Minute), 100, time.Time{})
	require.NoError(t, err)
	var metrics *service.GroupCacheShowcaseItem
	for _, m := range summary {
		if m.GroupID == group.ID {
			metrics = m
		}
	}
	require.NotNil(t, metrics)
	require.Equal(t, "upstream", metrics.Source)
	require.InDelta(t, .922, *metrics.CacheHitRate, .00001)
	require.NoError(t, plans.Prune(ctx, now.Add(time.Minute)))
	_, total, err = plans.ListResults(ctx, plan.ID, 0, 20)
	require.NoError(t, err)
	require.Zero(t, total)
	current, err = plans.GetPlan(ctx, plan.ID)
	require.NoError(t, err)
	require.InDelta(t, 1.5, current.TotalCostUSD, .00001, "cost aggregate survives cleanup")
	items, err = showcase.List(ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, items, 2, "showcase snapshots survive admin history pruning")
	deleted, err := plans.DeletePlan(ctx, plan.ID)
	require.NoError(t, err)
	require.True(t, deleted)
	items, err = showcase.List(ctx, group.ID)
	require.NoError(t, err)
	require.Empty(t, items, "deleting a plan withdraws its collected samples")
	var totalCost float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT sum(cost_usd) FROM group_cache_test_daily_costs WHERE group_id=$1`, group.ID).Scan(&totalCost))
	require.InDelta(t, 1.5, totalCost, .00001)
}
