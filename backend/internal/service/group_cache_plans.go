package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/google/uuid"
)

const GroupCacheDefaultPrompt = "Read the synthetic reference below. Reply with OK only."
const cacheTestLease = 15 * time.Minute

type GroupCacheTestConfig struct {
	Prompt          string `json:"prompt"`
	ReasoningEffort string `json:"reasoning_effort"`
	ParallelCount   int    `json:"parallel_count"`
	Rounds          int    `json:"rounds"`
	PrefixBlocks    int    `json:"prefix_blocks"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	IntervalMs      int    `json:"interval_ms"`
}
type GroupCacheTestPlanInput struct {
	GroupID        int64  `json:"group_id"`
	ModelID        string `json:"model_id"`
	CronExpression string `json:"cron_expression"`
	Enabled        bool   `json:"enabled"`
	GroupCacheTestConfig
}
type GroupCacheTestPlan struct {
	ID                  int64                 `json:"id"`
	GroupID             int64                 `json:"group_id"`
	CreatedBy           int64                 `json:"-"`
	GroupName           string                `json:"group_name"`
	GroupPlatform       string                `json:"group_platform"`
	GroupStatus         string                `json:"group_status"`
	ModelID             string                `json:"model_id"`
	CronExpression      string                `json:"cron_expression"`
	Enabled             bool                  `json:"enabled"`
	Config              GroupCacheTestConfig  `json:"config"`
	LastRunAt           *time.Time            `json:"last_run_at"`
	NextRunAt           *time.Time            `json:"next_run_at"`
	RunningUntil        *time.Time            `json:"running_until"`
	UpdatedAt           time.Time             `json:"updated_at"`
	TodayCostUSD        float64               `json:"today_cost_usd"`
	TotalCostUSD        float64               `json:"total_cost_usd"`
	TodayCostIncomplete bool                  `json:"today_cost_incomplete"`
	TotalCostIncomplete bool                  `json:"total_cost_incomplete"`
	LastResult          *GroupCacheTestResult `json:"last_result,omitempty"`
}
type GroupCacheTestRound struct {
	Index            int       `json:"index"`
	Status           string    `json:"status"`
	Protocol         string    `json:"protocol"`
	InputTokens      *int64    `json:"input_tokens"`
	CacheReadTokens  *int64    `json:"cache_read_tokens"`
	CacheWriteTokens *int64    `json:"cache_write_tokens"`
	OutputTokens     *int64    `json:"output_tokens"`
	FirstTokenMs     *int      `json:"first_token_ms"`
	DurationMs       int64     `json:"duration_ms"`
	MeasuredAt       time.Time `json:"measured_at"`
}
type GroupCacheTestResult struct {
	ID             int64                     `json:"id"`
	PlanID         int64                     `json:"plan_id"`
	GroupID        int64                     `json:"group_id"`
	GroupName      string                    `json:"group_name"`
	ModelID        string                    `json:"model_id"`
	RunID          string                    `json:"-"`
	AccountID      int64                     `json:"account_id"`
	AccountName    string                    `json:"account_name"`
	Attempts       []PelicanGroupTestAttempt `json:"attempts"`
	Status         string                    `json:"status"`
	ErrorMessage   string                    `json:"error_message"`
	CostUSD        *float64                  `json:"cost_usd"`
	CostIncomplete bool                      `json:"cost_incomplete"`
	Config         GroupCacheTestConfig      `json:"config"`
	Rounds         []GroupCacheTestRound     `json:"rounds"`
	PublicationID  *int64                    `json:"publication_id"`
	StartedAt      time.Time                 `json:"started_at"`
	FinishedAt     time.Time                 `json:"finished_at"`
	LatencyMs      int64                     `json:"latency_ms"`
}
type GroupCacheTestRepository interface {
	ListPlans(context.Context) ([]*GroupCacheTestPlan, error)
	GetPlan(context.Context, int64) (*GroupCacheTestPlan, error)
	SavePlan(context.Context, *GroupCacheTestPlan) (*GroupCacheTestPlan, error)
	DeletePlan(context.Context, int64) (bool, error)
	ListDue(context.Context, time.Time) ([]*GroupCacheTestPlan, error)
	Claim(context.Context, *GroupCacheTestPlan, time.Time, time.Time, *time.Time) (bool, error)
	Finish(context.Context, int64, time.Time, time.Time) error
	SaveResult(context.Context, *GroupCacheTestResult) error
	ListResults(context.Context, int64, int, int) ([]*GroupCacheTestResult, int64, error)
	GetResult(context.Context, int64) (*GroupCacheTestResult, error)
	Prune(context.Context, time.Time) error
}

var ErrGroupCacheTestNotFound = infraerrors.NotFound("CACHE_TEST_NOT_FOUND", "cache test not found")
var ErrGroupCacheTestRunning = infraerrors.Conflict("CACHE_TEST_RUNNING", "cache test is already running")

type GroupCacheTestService struct {
	repo       GroupCacheTestRepository
	groups     pelicanGroupTestGroups
	router     pelicanGroupRouter
	billing    *BillingService
	showcase   *GroupCacheShowcaseService
	runAccount func(context.Context, int64, string, string, string, int) (*ScheduledTestResult, *int, error)
	now        func() time.Time
	slots      chan struct{}
	runs       sync.WaitGroup
}

func NewGroupCacheTestService(repo GroupCacheTestRepository, groups *PelicanGroupTestService, account *AccountTestService, showcase *GroupCacheShowcaseService) *GroupCacheTestService {
	return &GroupCacheTestService{repo: repo, groups: groups.groups, router: groups.router, billing: groups.billing, showcase: showcase, runAccount: account.RunCacheBackground, now: time.Now, slots: make(chan struct{}, 4)}
}
func (s *GroupCacheTestService) ListPlans(ctx context.Context) ([]*GroupCacheTestPlan, error) {
	return s.repo.ListPlans(ctx)
}
func (s *GroupCacheTestService) SavePlan(ctx context.Context, id, userID int64, input GroupCacheTestPlanInput) (*GroupCacheTestPlan, error) {
	plan := &GroupCacheTestPlan{ID: id, GroupID: input.GroupID, CreatedBy: userID}
	if id > 0 {
		current, err := s.repo.GetPlan(ctx, id)
		if err != nil {
			return nil, err
		}
		if current == nil {
			return nil, ErrGroupCacheTestNotFound
		}
		plan.GroupID, plan.CreatedBy = current.GroupID, current.CreatedBy
		plan.UpdatedAt = current.UpdatedAt
	}
	invalid := func() (*GroupCacheTestPlan, error) {
		return nil, infraerrors.BadRequest("INVALID_CACHE_TEST", "choose an active supported group and valid model, prompt, schedule and sampling limits")
	}
	group, err := s.groups.GetByID(ctx, plan.GroupID)
	if err != nil && !errors.Is(err, ErrGroupNotFound) {
		return nil, err
	}
	if group == nil {
		return invalid()
	}
	if id == 0 && group.Status != StatusActive {
		return invalid()
	}
	if !isOpenAIResponsesGatewayPlatform(group.Platform) && group.Platform != PlatformAnthropic && group.Platform != PlatformComposite {
		return invalid()
	}
	plan.ModelID = strings.TrimSpace(input.ModelID)
	cfg := input.GroupCacheTestConfig
	cfg.Prompt = strings.TrimSpace(cfg.Prompt)
	cfg.ReasoningEffort = normalizePelicanReasoningEffort(cfg.ReasoningEffort)
	if cfg.ParallelCount == 0 {
		cfg.ParallelCount = 1
	}
	if cfg.Rounds == 0 {
		cfg.Rounds = 3
	}
	if cfg.PrefixBlocks == 0 {
		cfg.PrefixBlocks = 32
	}
	if cfg.MaxOutputTokens == 0 {
		cfg.MaxOutputTokens = 256
	}
	if plan.CreatedBy <= 0 || len(plan.ModelID) == 0 || len(plan.ModelID) > 100 || len(cfg.Prompt) == 0 || len(cfg.Prompt) > 32000 || cfg.ReasoningEffort == "" || cfg.ParallelCount < 1 || cfg.ParallelCount > 8 || cfg.Rounds < 2 || cfg.Rounds > 6 || cfg.PrefixBlocks < 8 || cfg.PrefixBlocks > 128 || cfg.MaxOutputTokens < 16 || cfg.MaxOutputTokens > 8192 || cfg.IntervalMs < 0 || cfg.IntervalMs > 30000 {
		return invalid()
	}
	cron := strings.TrimSpace(input.CronExpression)
	if len(cron) == 0 || len(cron) > 100 {
		return invalid()
	}
	next, err := computeNextRun(cron, s.now())
	if err != nil {
		return invalid()
	}
	plan.CronExpression, plan.Enabled, plan.Config = cron, input.Enabled, cfg
	if input.Enabled {
		plan.NextRunAt = &next
	}
	return s.repo.SavePlan(ctx, plan)
}
func (s *GroupCacheTestService) DeletePlan(ctx context.Context, id int64) error {
	deleted, err := s.repo.DeletePlan(ctx, id)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrGroupCacheTestNotFound
	}
	return nil
}
func (s *GroupCacheTestService) RunNow(ctx context.Context, id int64) error {
	plan, err := s.repo.GetPlan(ctx, id)
	if err != nil {
		return err
	}
	if plan == nil {
		return ErrGroupCacheTestNotFound
	}
	if plan.GroupStatus != StatusActive {
		return infraerrors.BadRequest("CACHE_GROUP_UNAVAILABLE", "group is disabled or deleted")
	}
	// Bound manual runs as well as scheduled runs before taking the durable lease.
	select {
	case s.slots <- struct{}{}:
	default:
		return infraerrors.Conflict("CACHE_TEST_BUSY", "cache test workers are busy")
	}
	now := s.now()
	until := now.Add(cacheTestLease).Truncate(time.Microsecond)
	claimed, err := s.repo.Claim(ctx, plan, now, until, nil)
	if err != nil || !claimed {
		<-s.slots
		if err != nil {
			return err
		}
		return ErrGroupCacheTestRunning
	}
	s.runs.Add(1)
	go func() {
		defer s.runs.Done()
		defer func() { <-s.slots }()
		s.execute(context.Background(), plan, until)
	}()
	return nil
}
func (s *GroupCacheTestService) RunDue(ctx context.Context, now time.Time) {
	if s == nil {
		return
	}
	plans, err := s.repo.ListDue(ctx, now)
	if err != nil {
		slog.Error("cache test list due failed", "error", err)
		return
	}
	var wg sync.WaitGroup
	for _, p := range plans {
		select {
		case s.slots <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return
		default:
			continue
		}
		next, err := computeNextRun(p.CronExpression, now)
		if err != nil {
			<-s.slots
			continue
		}
		until := now.Add(cacheTestLease).Truncate(time.Microsecond)
		claimed, err := s.repo.Claim(ctx, p, now, until, &next)
		if err != nil || !claimed {
			<-s.slots
			continue
		}
		wg.Add(1)
		go func(plan *GroupCacheTestPlan) {
			defer wg.Done()
			defer func() { <-s.slots }()
			s.execute(ctx, plan, until)
		}(p)
	}
	wg.Wait()
}
func (s *GroupCacheTestService) execute(parent context.Context, plan *GroupCacheTestPlan, until time.Time) {
	defer func() {
		if recover() != nil {
			slog.Error("cache test panicked", "plan_id", plan.ID)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.repo.Finish(ctx, plan.ID, until, s.now()); err != nil {
			slog.Error("cache test finish failed", "error", err)
		}
	}()
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	defer cancel()
	group, err := s.groups.GetByID(ctx, plan.GroupID)
	count := plan.Config.ParallelCount
	results := make([]*GroupCacheTestResult, count)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			if err != nil || group == nil || group.Status != StatusActive {
				results[index] = s.newResult(plan, "group_unavailable")
			} else {
				results[index] = s.sample(ctx, plan, group)
			}
		}(i)
	}
	wg.Wait()
	saveCtx, stop := context.WithTimeout(context.Background(), 30*time.Second)
	defer stop()
	for _, result := range results {
		if result.Status == "success" {
			snapshot := &GroupCacheSnapshot{RunID: result.RunID, CreatedBy: plan.CreatedBy, GroupID: plan.GroupID, GroupName: plan.GroupName, Model: plan.ModelID, PlanID: plan.ID, Source: "upstream", TestedAt: result.FinishedAt}
			for _, round := range result.Rounds {
				snapshot.Protocol = round.Protocol
				snapshot.Rounds = append(snapshot.Rounds, GroupCacheSnapshotRound{Index: round.Index, InputTokens: *round.InputTokens, CacheReadTokens: *round.CacheReadTokens, FirstTokenMs: round.FirstTokenMs, MeasuredAt: round.MeasuredAt})
			}
			if err := s.showcase.repo.Publish(saveCtx, snapshot); err == nil {
				result.PublicationID = &snapshot.ID
			} else {
				slog.Error("cache test publish failed", "plan_id", plan.ID, "error", err)
			}
		}
		if err := s.repo.SaveResult(saveCtx, result); err != nil {
			slog.Error("cache test save result failed", "plan_id", plan.ID, "error", err)
		}
	}
	s.Cleanup(saveCtx, s.now())
}
func (s *GroupCacheTestService) newResult(plan *GroupCacheTestPlan, message string) *GroupCacheTestResult {
	now := s.now()
	zero := 0.0
	return &GroupCacheTestResult{PlanID: plan.ID, GroupID: plan.GroupID, GroupName: plan.GroupName, ModelID: plan.ModelID, RunID: uuid.NewString(), Config: plan.Config, Status: "failed", ErrorMessage: message, CostUSD: &zero, Attempts: []PelicanGroupTestAttempt{}, Rounds: []GroupCacheTestRound{}, StartedAt: now, FinishedAt: now}
}
func cacheSyntheticPrefix(runID string, blocks int) string {
	var out strings.Builder
	fmt.Fprintf(&out, "Synthetic cache reference %s\n", runID)
	for i := 0; i < blocks; i++ {
		fmt.Fprintf(&out, "Reference %03d: alpha cedar delta ember fern granite harbor indigo jasmine kestrel linen meadow nectar opal pebble quartz river silver timber umber violet willow xenon yellow zephyr. This is synthetic diagnostic content; preserve this reference and answer only the final instruction.\n", i)
	}
	return out.String()
}
func (s *GroupCacheTestService) sample(ctx context.Context, plan *GroupCacheTestPlan, group *Group) (result *GroupCacheTestResult) {
	result = s.newResult(plan, "")
	priced, attempted, incomplete := false, false, false
	total := 0.0
	defer func() {
		if recover() != nil {
			result.Status = "failed"
			result.ErrorMessage = "cache_sample_failed"
		}
		result.FinishedAt = s.now()
		result.LatencyMs = result.FinishedAt.Sub(result.StartedAt).Milliseconds()
		if priced || !attempted {
			result.CostUSD = &total
		} else {
			result.CostUSD = nil
		}
		result.CostIncomplete = incomplete || (attempted && !priced)
	}()
	excluded := map[int64]struct{}{}
	// Salt before the administrator text so a long shared prompt cannot prime
	// the synthetic prefix across samples. Provider system content may still hit.
	prefix := cacheSyntheticPrefix(result.RunID, plan.Config.PrefixBlocks) + "\n" + plan.Config.Prompt + "\nReply with OK only."
	for attempt := 0; attempt < PelicanGroupTestMaxAttempts; attempt++ {
		if ctx.Err() != nil {
			result.ErrorMessage = "cache_test_timeout"
			return result
		}
		route, err := s.router.route(ctx, group, plan.ModelID, excluded)
		if err != nil || route == nil {
			result.ErrorMessage = "no_available_account"
			return result
		}
		result.AccountID, result.AccountName = route.account.ID, route.account.Name
		retry := false
		func() {
			defer route.release()
			for index := 1; index <= plan.Config.Rounds; index++ {
				if ctx.Err() != nil {
					result.ErrorMessage = "cache_test_timeout"
					return
				}
				collector := &pelicanTestUsageCollector{model: route.account.GetMappedModel(route.model)}
				roundCtx, stop := context.WithTimeout(context.WithValue(ctx, pelicanTestUsageKey{}, collector), 2*time.Minute)
				attempted = true
				prompt := prefix + fmt.Sprintf("\n[cache-round]%d\nReply with OK only.", index)
				sample, ttft, err := s.runAccount(roundCtx, route.account.ID, route.model, prompt, plan.Config.ReasoningEffort, plan.Config.MaxOutputTokens)
				stop()
				cost, partial := collector.cost(s.billing, route.account)
				incomplete = incomplete || partial
				for _, usage := range collector.requests {
					incomplete = incomplete || !usage.cacheReadSeen || (usage.protocol == "anthropic" && !usage.cacheWriteSeen)
				}
				if cost != nil {
					priced = true
					total += *cost
				}
				if err != nil || sample == nil {
					sample = &ScheduledTestResult{Status: "failed", ErrorMessage: "cache_request_failed"}
				}
				round := GroupCacheTestRound{Index: index, Status: sample.Status, FirstTokenMs: ttft, DurationMs: sample.LatencyMs, MeasuredAt: s.now()}
				if len(collector.requests) > 0 {
					usage := collector.requests[len(collector.requests)-1]
					round.Protocol = map[string]string{"openai": "responses", "chat": "chat", "anthropic": "messages"}[usage.protocol]
					if usage.inputSeen && !usage.invalid && (usage.protocol != "anthropic" || usage.cacheWriteSeen) {
						totalInput := int64(usage.tokens.InputTokens) + int64(usage.tokens.CacheReadTokens) + int64(usage.tokens.CacheCreationTokens)
						round.InputTokens = &totalInput
					}
					if usage.cacheReadSeen && !usage.invalid {
						read := int64(usage.tokens.CacheReadTokens)
						round.CacheReadTokens = &read
					}
					if usage.cacheWriteSeen && !usage.invalid {
						write := int64(usage.tokens.CacheCreationTokens)
						round.CacheWriteTokens = &write
					}
					if usage.outputSeen && !usage.invalid {
						output := int64(usage.tokens.OutputTokens)
						round.OutputTokens = &output
					}
				}
				if sample.Status != "success" {
					result.ErrorMessage = "cache_request_failed"
					if index == 1 && pelicanFailedBeforeOutput(sample) && attempt+1 < PelicanGroupTestMaxAttempts {
						retry = true
						result.Attempts = append(result.Attempts, PelicanGroupTestAttempt{AccountID: route.account.ID, AccountName: route.account.Name, Error: "request_failed_before_output"})
						excluded[route.account.ID] = struct{}{}
						return
					}
					result.Rounds = append(result.Rounds, round)
					return
				}
				if round.Protocol == "" || round.InputTokens == nil || *round.InputTokens <= 0 || round.CacheReadTokens == nil || *round.CacheReadTokens > *round.InputTokens || round.OutputTokens == nil || *round.OutputTokens <= 0 {
					round.Status = "failed"
					result.ErrorMessage = "cache_usage_unavailable"
				}
				if len(result.Rounds) > 0 && round.Protocol != result.Rounds[0].Protocol {
					round.Status = "failed"
					result.ErrorMessage = "cache_protocol_changed"
				}
				result.Rounds = append(result.Rounds, round)
				if round.Status != "success" {
					return
				}
				if index < plan.Config.Rounds && plan.Config.IntervalMs > 0 {
					select {
					case <-ctx.Done():
						result.ErrorMessage = "cache_test_timeout"
						return
					case <-time.After(time.Duration(plan.Config.IntervalMs) * time.Millisecond):
					}
				}
			}
			result.Status = "success"
			result.ErrorMessage = ""
		}()
		if !retry {
			return result
		}
	}
	return result
}
func (s *GroupCacheTestService) ListResults(ctx context.Context, planID int64, page, pageSize int) ([]*GroupCacheTestResult, int64, error) {
	p := pagination.PaginationParams{Page: page, PageSize: pageSize}
	return s.repo.ListResults(ctx, planID, p.Offset(), p.Limit())
}
func (s *GroupCacheTestService) GetResult(ctx context.Context, id int64) (*GroupCacheTestResult, error) {
	r, e := s.repo.GetResult(ctx, id)
	if e != nil {
		return nil, e
	}
	if r == nil {
		return nil, ErrGroupCacheTestNotFound
	}
	return r, nil
}
func (s *GroupCacheTestService) Cleanup(ctx context.Context, now time.Time) {
	if s == nil {
		return
	}
	if err := s.repo.Prune(ctx, now.Add(-7*24*time.Hour)); err != nil {
		slog.Error("cache test history cleanup failed", "error", err)
	}
	if err := s.showcase.Cleanup(ctx, now); err != nil {
		slog.Error("cache test showcase cleanup failed", "error", err)
	}
}
