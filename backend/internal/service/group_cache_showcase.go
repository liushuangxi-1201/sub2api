package service

import (
	"context"
	"regexp"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const SettingKeyGroupCacheShowcaseEnabled = "group_cache_showcase_enabled"

type GroupCachePublishRound struct {
	UsageLogID int64  `json:"usage_log_id"`
	RequestID  string `json:"request_id"`
}
type GroupCachePublishRequest struct {
	RunID    string                   `json:"run_id"`
	GroupID  int64                    `json:"group_id"`
	APIKeyID int64                    `json:"api_key_id"`
	Model    string                   `json:"model"`
	Protocol string                   `json:"protocol"`
	Rounds   []GroupCachePublishRound `json:"rounds"`
}

// Only verified server measurements enter a snapshot. Never store prompts, keys,
// account identities, costs or browser-reported cache rates in this feature.
type GroupCacheSnapshotRound struct {
	UsageLogID      int64
	Index           int
	InputTokens     int64
	CacheReadTokens int64
	FirstTokenMs    *int
	MeasuredAt      time.Time
}
type GroupCacheSnapshot struct {
	ID          int64                     `json:"id"`
	PlanID      int64                     `json:"-"`
	Source      string                    `json:"source"`
	RunID       string                    `json:"run_id"`
	CreatedBy   int64                     `json:"created_by"`
	GroupID     int64                     `json:"group_id"`
	GroupName   string                    `json:"group_name"`
	Model       string                    `json:"model"`
	Protocol    string                    `json:"protocol"`
	TestedAt    time.Time                 `json:"tested_at"`
	PublishedAt time.Time                 `json:"published_at"`
	Rounds      []GroupCacheSnapshotRound `json:"-"`
}

// Separate public DTO: internal snapshot/user/request IDs never reach users.
type GroupCacheShowcaseItem struct {
	Source          string    `json:"source"`
	GroupID         int64     `json:"group_id"`
	GroupName       string    `json:"group_name"`
	Platform        string    `json:"platform"`
	Model           string    `json:"model"`
	Protocol        string    `json:"protocol"`
	LastTestedAt    time.Time `json:"last_tested_at"`
	Samples         int64     `json:"samples"`
	LatencySamples  int64     `json:"latency_samples"`
	InputTokens     int64     `json:"input_tokens"`
	CacheReadTokens int64     `json:"cache_read_tokens"`
	CacheHitRate    *float64  `json:"cache_hit_rate"`
	FirstTokenMs    *float64  `json:"first_token_ms"`
}
type GroupCacheShowcaseView struct {
	Enabled     bool                      `json:"enabled"`
	WindowStart time.Time                 `json:"window_start"`
	WindowEnd   time.Time                 `json:"window_end"`
	Items       []*GroupCacheShowcaseItem `json:"items"`
}

type GroupCacheShowcaseRepository interface {
	Publish(context.Context, *GroupCacheSnapshot) error
	List(context.Context, int64) ([]*GroupCacheSnapshot, error)
	Delete(context.Context, int64) (bool, error)
	Summary(context.Context, time.Time, time.Time, int, time.Time) ([]*GroupCacheShowcaseItem, error)
	Prune(context.Context, int, time.Time) error
}
type GroupCacheShowcaseService struct {
	repo     GroupCacheShowcaseRepository
	settings SettingRepository
	usage    UsageLogRepository
	groups   GroupRepository
	onUpdate func()
}

func NewGroupCacheShowcaseService(repo GroupCacheShowcaseRepository, settings *SettingService, usage UsageLogRepository, groups GroupRepository) *GroupCacheShowcaseService {
	return &GroupCacheShowcaseService{repo: repo, settings: settings.settingRepo, usage: usage, groups: groups, onUpdate: func() {
		if settings.onUpdate != nil {
			settings.onUpdate()
		}
	}}
}

func (s *GroupCacheShowcaseService) Enabled(ctx context.Context) (bool, error) {
	values, err := s.settings.GetMultiple(ctx, []string{SettingKeyGroupCacheShowcaseEnabled})
	return values[SettingKeyGroupCacheShowcaseEnabled] == "true", err
}
func (s *GroupCacheShowcaseService) SetEnabled(ctx context.Context, enabled bool) error {
	value := "false"
	if enabled {
		value = "true"
	}
	if err := s.settings.SetMultiple(ctx, map[string]string{SettingKeyGroupCacheShowcaseEnabled: value}); err != nil {
		return err
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
	return nil
}
func (s *GroupCacheShowcaseService) View(ctx context.Context, now time.Time) (*GroupCacheShowcaseView, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return nil, err
	}
	view := &GroupCacheShowcaseView{Enabled: settings.Enabled, WindowStart: now.Add(-24 * time.Hour), WindowEnd: now, Items: []*GroupCacheShowcaseItem{}}
	if !settings.Enabled {
		return view, nil
	}
	view.Items, err = s.repo.Summary(ctx, view.WindowStart, now, settings.MaxItems, settings.retentionCutoff(now))
	return view, err
}
func (s *GroupCacheShowcaseService) List(ctx context.Context, groupID int64) ([]*GroupCacheSnapshot, error) {
	if groupID <= 0 {
		return nil, cacheShowcaseInvalid()
	}
	return s.repo.List(ctx, groupID)
}
func (s *GroupCacheShowcaseService) Remove(ctx context.Context, id int64) error {
	deleted, err := s.repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	if !deleted {
		return infraerrors.NotFound("CACHE_SHOWCASE_NOT_FOUND", "published test not found")
	}
	return nil
}

var cacheShowcaseRunID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)

func cacheShowcaseInvalid() error {
	return infraerrors.BadRequest("INVALID_CACHE_SHOWCASE", "publish 2–6 distinct, billed rounds from your selected group, key, model and protocol within the last 7 days")
}

// Publish accepts identities only, then verifies every round against stored
// usage and the current operator. A submitted rate/timestamp can never override
// server measurements. Publishing does not enable the user showcase.
func (s *GroupCacheShowcaseService) Publish(ctx context.Context, userID int64, req GroupCachePublishRequest, now time.Time) (*GroupCacheSnapshot, error) {
	endpoints := map[string]string{"responses": "/v1/responses", "chat": "/v1/chat/completions", "messages": "/v1/messages"}
	endpoint := endpoints[req.Protocol]
	if userID <= 0 || req.GroupID <= 0 || req.APIKeyID <= 0 || endpoint == "" || !cacheShowcaseRunID.MatchString(req.RunID) || len(req.Model) == 0 || len(req.Model) > 200 || len(req.Rounds) < 2 || len(req.Rounds) > 6 {
		return nil, cacheShowcaseInvalid()
	}
	group, err := s.groups.GetByID(ctx, req.GroupID)
	if err != nil {
		return nil, err
	}
	if group == nil || group.Status != StatusActive {
		return nil, cacheShowcaseInvalid()
	}
	snapshot := &GroupCacheSnapshot{RunID: req.RunID, CreatedBy: userID, GroupID: req.GroupID, GroupName: group.Name, Model: req.Model, Protocol: req.Protocol, Source: "billing"}
	seen := map[int64]bool{}
	seenRequests := map[string]bool{}
	var first, previous time.Time
	for i, source := range req.Rounds {
		if source.UsageLogID <= 0 || seen[source.UsageLogID] || seenRequests[source.RequestID] || len(source.RequestID) < 8 || len(source.RequestID) > 128 || source.RequestID[:7] != "client:" {
			return nil, cacheShowcaseInvalid()
		}
		seen[source.UsageLogID], seenRequests[source.RequestID] = true, true
		log, err := s.usage.GetByID(ctx, source.UsageLogID)
		if err != nil {
			return nil, err
		}
		if log == nil {
			return nil, cacheShowcaseInvalid()
		}
		model := log.RequestedModel
		if model == "" {
			model = log.Model
		}
		if log.UserID != userID || log.APIKeyID != req.APIKeyID || log.GroupID == nil || *log.GroupID != req.GroupID || log.RequestID != source.RequestID || model != req.Model || log.InboundEndpoint == nil || *log.InboundEndpoint != endpoint || !log.Stream || log.OutputTokens <= 0 {
			return nil, cacheShowcaseInvalid()
		}
		for _, tokens := range []int{log.InputTokens, log.CacheReadTokens, log.CacheCreationTokens} {
			if tokens < 0 || tokens > 1_000_000_000 {
				return nil, cacheShowcaseInvalid()
			}
		}
		input := int64(log.InputTokens) + int64(log.CacheReadTokens) + int64(log.CacheCreationTokens)
		if input == 0 || log.CreatedAt.Before(now.Add(-7*24*time.Hour)) || log.CreatedAt.After(now) || (!previous.IsZero() && log.CreatedAt.Before(previous)) || (log.FirstTokenMs != nil && *log.FirstTokenMs < 0) {
			return nil, cacheShowcaseInvalid()
		}
		if i == 0 {
			first = log.CreatedAt
		}
		if log.CreatedAt.Sub(first) > time.Hour {
			return nil, cacheShowcaseInvalid()
		}
		previous = log.CreatedAt
		snapshot.Rounds = append(snapshot.Rounds, GroupCacheSnapshotRound{UsageLogID: log.ID, Index: i + 1, InputTokens: input, CacheReadTokens: int64(log.CacheReadTokens), FirstTokenMs: log.FirstTokenMs, MeasuredAt: log.CreatedAt})
	}
	snapshot.TestedAt = previous
	if err := s.repo.Publish(ctx, snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}
