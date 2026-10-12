package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type cachePlanRepoFake struct {
	GroupCacheTestRepository
	plan     *GroupCacheTestPlan
	claim    bool
	due      []*GroupCacheTestPlan
	claims   int
	next     *time.Time
	finished int
	results  []*GroupCacheTestResult
}

func (r *cachePlanRepoFake) GetPlan(context.Context, int64) (*GroupCacheTestPlan, error) {
	return r.plan, nil
}
func (r *cachePlanRepoFake) SavePlan(_ context.Context, p *GroupCacheTestPlan) (*GroupCacheTestPlan, error) {
	r.plan = p
	return p, nil
}
func (r *cachePlanRepoFake) ListDue(context.Context, time.Time) ([]*GroupCacheTestPlan, error) {
	return r.due, nil
}
func (r *cachePlanRepoFake) Claim(_ context.Context, _ *GroupCacheTestPlan, _, _ time.Time, next *time.Time) (bool, error) {
	r.claims++
	r.next = next
	return r.claim, nil
}
func (r *cachePlanRepoFake) Finish(context.Context, int64, time.Time, time.Time) error {
	r.finished++
	return nil
}
func (r *cachePlanRepoFake) SaveResult(_ context.Context, v *GroupCacheTestResult) error {
	r.results = append(r.results, v)
	return nil
}
func (r *cachePlanRepoFake) Prune(context.Context, time.Time) error { return nil }

type cachePlanShowcaseFake struct {
	GroupCacheShowcaseRepository
	items  []*GroupCacheSnapshot
	keep   int
	cutoff time.Time
}

func (r *cachePlanShowcaseFake) Publish(_ context.Context, v *GroupCacheSnapshot) error {
	v.ID = 90
	r.items = append(r.items, v)
	return nil
}
func (r *cachePlanShowcaseFake) Prune(_ context.Context, keep int, before time.Time) error {
	r.keep = keep
	r.cutoff = before
	return nil
}
func cachePlanFixture() (*GroupCacheTestService, *cachePlanRepoFake, *cachePlanShowcaseFake, *groupTestRouterFake) {
	p := &GroupCacheTestPlan{ID: 7, GroupID: 4, CreatedBy: 1, GroupName: "Cache group", GroupStatus: StatusActive, ModelID: "demo", CronExpression: "*/30 * * * *", Enabled: true, Config: GroupCacheTestConfig{Prompt: GroupCacheDefaultPrompt, ReasoningEffort: "minimal", ParallelCount: 1, Rounds: 3, PrefixBlocks: 8, MaxOutputTokens: 256}}
	repo := &cachePlanRepoFake{plan: p, claim: true}
	snapshot := &cachePlanShowcaseFake{}
	router := &groupTestRouterFake{steps: []routeStep{{account: account(11, "sample-a")}, {account: account(12, "sample-b")}}}
	s := &GroupCacheTestService{repo: repo, groups: groupTestGroupsFake{4: {ID: 4, Status: StatusActive, Platform: PlatformOpenAI}}, router: router, showcase: &GroupCacheShowcaseService{repo: snapshot, settings: &showcaseRuntimeSettingRepo{values: map[string]string{}}}, now: func() time.Time { return groupTestNow }, slots: make(chan struct{}, 4)}
	s.runAccount = func(ctx context.Context, _ int64, _, _, _ string, _ int) (*ScheduledTestResult, *int, error) {
		usage := startPelicanTestUsage(ctx, "openai")
		usage.read(`{"type":"response.completed","response":{"model":"demo","usage":{"input_tokens":1000,"output_tokens":2,"input_tokens_details":{"cached_tokens":900}}}}`)
		ms := 4700
		return &ScheduledTestResult{Status: "success", ResponseText: "OK"}, &ms, nil
	}
	return s, repo, snapshot, router
}
func TestGroupCachePlansScheduleAndAutoCollectionWhileHidden(t *testing.T) {
	s, repo, published, router := cachePlanFixture()
	repo.due = []*GroupCacheTestPlan{repo.plan}
	original := s.runAccount
	prompts := []string{}
	s.runAccount = func(ctx context.Context, id int64, model, prompt, effort string, budget int) (*ScheduledTestResult, *int, error) {
		prompts = append(prompts, prompt)
		return original(ctx, id, model, prompt, effort, budget)
	}
	s.RunDue(context.Background(), groupTestNow)
	require.Equal(t, 1, repo.claims)
	require.NotNil(t, repo.next)
	require.True(t, repo.next.After(groupTestNow))
	require.Equal(t, 1, repo.finished)
	require.Len(t, repo.results, 1)
	require.Equal(t, "success", repo.results[0].Status)
	require.Len(t, published.items, 1, "samples are collected even when the user showcase is closed")
	require.Equal(t, "upstream", published.items[0].Source)
	require.Equal(t, int64(7), published.items[0].PlanID)
	require.Len(t, router.released, 1, "one routed account and one held slot per sequence")
	require.Len(t, prompts, 3)
	prefix := strings.Split(prompts[0], "\n[cache-round]")[0]
	for _, prompt := range prompts {
		require.Equal(t, prefix, strings.Split(prompt, "\n[cache-round]")[0])
	}
	require.NotEqual(t, prompts[0], prompts[1])
	raw, err := json.Marshal(published.items[0])
	require.NoError(t, err)
	require.NotContains(t, string(raw), "sample-a")
	require.Equal(t, 20, published.keep)
}
func TestGroupCachePlansDoNotPublishMissingMeteringOrReplayWarmFailure(t *testing.T) {
	for _, name := range []string{"warm failure", "missing cache usage", "missing output usage", "negative cache usage", "oversized input", "cache exceeds input", "fractional tokens"} {
		t.Run(name, func(t *testing.T) {
			s, repo, published, router := cachePlanFixture()
			calls := 0
			s.runAccount = func(ctx context.Context, _ int64, _, _, _ string, _ int) (*ScheduledTestResult, *int, error) {
				calls++
				u := startPelicanTestUsage(ctx, "openai")
				switch name {
				case "missing cache usage":
					u.read(`{"type":"response.completed","response":{"usage":{"input_tokens":1000,"output_tokens":2}}}`)
				case "missing output usage":
					u.read(`{"type":"response.completed","response":{"usage":{"input_tokens":1000,"input_tokens_details":{"cached_tokens":0}}}}`)
				case "negative cache usage":
					u.read(`{"type":"response.completed","response":{"usage":{"input_tokens":1000,"output_tokens":2,"input_tokens_details":{"cached_tokens":-1}}}}`)
				case "oversized input":
					u.read(`{"type":"response.completed","response":{"usage":{"input_tokens":1e30,"output_tokens":2,"input_tokens_details":{"cached_tokens":0}}}}`)
				case "cache exceeds input":
					u.read(`{"type":"response.completed","response":{"usage":{"input_tokens":1000,"output_tokens":2,"input_tokens_details":{"cached_tokens":1001}}}}`)
				case "fractional tokens":
					u.read(`{"type":"response.completed","response":{"usage":{"input_tokens":1000.5,"output_tokens":2,"input_tokens_details":{"cached_tokens":0}}}}`)
				default:
					u.read(`{"type":"response.completed","response":{"usage":{"input_tokens":1000,"output_tokens":2,"input_tokens_details":{"cached_tokens":0}}}}`)
				}
				status := "success"
				if calls == 2 && name == "warm failure" {
					status = "failed"
				}
				return &ScheduledTestResult{Status: status, ResponseText: "partial output"}, nil, nil
			}
			s.execute(context.Background(), repo.plan, groupTestNow.Add(cacheTestLease))
			require.Empty(t, published.items)
			require.Equal(t, "failed", repo.results[0].Status)
			require.Len(t, router.released, 1)
			if name == "warm failure" {
				require.Equal(t, 2, calls)
			} else {
				require.Equal(t, 1, calls)
				require.Equal(t, "cache_usage_unavailable", repo.results[0].ErrorMessage)
				if name == "missing cache usage" {
					require.Nil(t, repo.results[0].Rounds[0].CacheReadTokens)
				}
			}
		})
	}
}
func TestGroupCachePlansFailoverOnlyBaselineAndRespectLease(t *testing.T) {
	s, repo, published, router := cachePlanFixture()
	calls := 0
	original := s.runAccount
	s.runAccount = func(ctx context.Context, id int64, model, prompt, effort string, budget int) (*ScheduledTestResult, *int, error) {
		calls++
		if calls == 1 {
			return &ScheduledTestResult{Status: "failed", ErrorMessage: "HTTP rejected"}, nil, nil
		}
		return original(ctx, id, model, prompt, effort, budget)
	}
	require.NoError(t, s.RunNow(context.Background(), 7))
	s.runs.Wait()
	require.Len(t, published.items, 1)
	require.Len(t, router.released, 2)
	require.Len(t, repo.results[0].Attempts, 1)
	require.Equal(t, int64(12), repo.results[0].AccountID)
	repo.claim = false
	require.ErrorIs(t, s.RunNow(context.Background(), 7), ErrGroupCacheTestRunning)
	require.Empty(t, s.slots, "a rejected lease releases its worker slot")
}

func TestGroupCachePlansParallelSamplesAndWorkerLimit(t *testing.T) {
	s, repo, published, router := cachePlanFixture()
	repo.plan.Config.ParallelCount = 2
	s.execute(context.Background(), repo.plan, groupTestNow.Add(cacheTestLease))
	require.Len(t, repo.results, 2)
	require.Len(t, published.items, 2)
	require.NotEqual(t, published.items[0].RunID, published.items[1].RunID)
	require.ElementsMatch(t, []int64{11, 12}, router.released)
	for _, result := range repo.results {
		require.Equal(t, "success", result.Status)
		require.Len(t, result.Rounds, 3)
	}
	for i := 0; i < cap(s.slots); i++ {
		s.slots <- struct{}{}
	}
	require.Error(t, s.RunNow(context.Background(), repo.plan.ID))
	require.Zero(t, repo.claims, "a full worker pool must not take a durable lease")
	require.Len(t, s.slots, 4)
}
func TestGroupCachePlansValidateAndPreserveGroupOwner(t *testing.T) {
	s, repo, _, _ := cachePlanFixture()
	input := GroupCacheTestPlanInput{GroupID: 999, ModelID: "demo", CronExpression: "*/30 * * * *", Enabled: false, GroupCacheTestConfig: repo.plan.Config}
	p, e := s.SavePlan(context.Background(), 7, 99, input)
	require.NoError(t, e)
	require.Equal(t, int64(4), p.GroupID)
	require.Equal(t, int64(1), p.CreatedBy)
	require.Nil(t, p.NextRunAt)
	for _, edit := range []func(*GroupCacheTestPlanInput){func(i *GroupCacheTestPlanInput) { i.Rounds = 7 }, func(i *GroupCacheTestPlanInput) { i.ParallelCount = 9 }, func(i *GroupCacheTestPlanInput) { i.CronExpression = "bad" }, func(i *GroupCacheTestPlanInput) { i.PrefixBlocks = 7 }} {
		bad := input
		edit(&bad)
		_, e = s.SavePlan(context.Background(), 7, 1, bad)
		require.Error(t, e)
	}
}

func TestGroupCacheProbePayloadLeavesTheRoundSuffixUncached(t *testing.T) {
	ctx := withPelicanTestOptions(context.Background(), pelicanTestOptions{cacheProbe: true, maxOutput: 256, prompt: "stable reference\n[cache-round]2\nReply with OK only."})
	payload, err := createPelicanClaudePayload("claude-sonnet-4-5", "unused", "minimal")
	require.NoError(t, err)
	applyCacheProbePayload(ctx, payload, "messages", false)
	require.Equal(t, 256, payload["max_tokens"])
	messages, ok := payload["messages"].([]map[string]any)
	require.True(t, ok)
	content, ok := messages[0]["content"].([]map[string]any)
	require.True(t, ok)
	require.Equal(t, "stable reference", content[0]["text"])
	require.Contains(t, content[0], "cache_control")
	require.NotContains(t, content[1], "cache_control")
	responses := map[string]any{}
	applyCacheProbePayload(ctx, responses, "responses", false)
	require.Equal(t, 256, responses["max_output_tokens"])
	oauth := map[string]any{}
	applyCacheProbePayload(ctx, oauth, "responses", true)
	require.NotContains(t, oauth, "max_output_tokens")
	unchanged := map[string]any{"max_tokens": 32000}
	applyCacheProbePayload(context.Background(), unchanged, "messages", false)
	require.Equal(t, 32000, unchanged["max_tokens"])
}
func TestGroupCacheSettingsFailClosedForCorruptConfiguration(t *testing.T) {
	s, _, repo, _ := cachePlanFixture()
	ctx := context.Background()
	cfg, e := s.showcase.Settings(ctx)
	require.NoError(t, e)
	require.False(t, cfg.Enabled)
	require.False(t, cfg.APIEnabled)
	require.Equal(t, 20, cfg.MaxItems)
	cfg.Enabled = true
	cfg.APIEnabled = true
	cfg.MaxItems = 3
	cfg.RetentionDays = 2
	_, e = s.showcase.UpdateSettings(ctx, cfg)
	require.NoError(t, e)
	require.NoError(t, s.showcase.Cleanup(ctx, groupTestNow))
	require.Equal(t, 3, repo.keep)
	require.Equal(t, groupTestNow.Add(-48*time.Hour), repo.cutoff)
	settings, ok := s.showcase.settings.(*showcaseRuntimeSettingRepo)
	require.True(t, ok)
	settings.values[SettingKeyGroupCacheShowcaseConfig] = "invalid-json"
	require.Error(t, s.showcase.Cleanup(ctx, groupTestNow))
	require.Equal(t, 3, repo.keep, "invalid settings cannot trigger a destructive default cleanup")
	view, e := s.showcase.View(ctx, groupTestNow)
	require.Error(t, e)
	require.Nil(t, view)
}
