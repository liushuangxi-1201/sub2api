package service

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const SettingKeyGroupCacheShowcaseConfig = "group_cache_showcase_config"
const SettingKeyGroupCacheShowcaseAPIEnabled = "group_cache_showcase_api_enabled"

type GroupCacheShowcaseSettings struct {
	Enabled    bool `json:"enabled"`
	APIEnabled bool `json:"api_enabled"`
	PelicanShowcaseConfig
}

func (s *GroupCacheShowcaseService) Settings(ctx context.Context) (GroupCacheShowcaseSettings, error) {
	values, err := s.settings.GetMultiple(ctx, []string{SettingKeyGroupCacheShowcaseEnabled, SettingKeyGroupCacheShowcaseConfig, SettingKeyGroupCacheShowcaseAPIEnabled})
	if err != nil {
		return GroupCacheShowcaseSettings{}, err
	}
	cfg := DefaultPelicanShowcaseConfig()
	if raw := values[SettingKeyGroupCacheShowcaseConfig]; raw != "" {
		if err = json.Unmarshal([]byte(raw), &cfg); err != nil {
			return GroupCacheShowcaseSettings{}, err
		}
		cfg, err = NormalizePelicanShowcaseConfig(cfg)
		if err != nil {
			return GroupCacheShowcaseSettings{}, err
		}
	}
	return GroupCacheShowcaseSettings{Enabled: values[SettingKeyGroupCacheShowcaseEnabled] == "true", APIEnabled: values[SettingKeyGroupCacheShowcaseAPIEnabled] == "true", PelicanShowcaseConfig: cfg}, nil
}
func (s *GroupCacheShowcaseService) UpdateSettings(ctx context.Context, input GroupCacheShowcaseSettings) (GroupCacheShowcaseSettings, error) {
	cfg, err := NormalizePelicanShowcaseConfig(input.PelicanShowcaseConfig)
	if err != nil {
		return input, infraerrors.BadRequest("INVALID_CACHE_SHOWCASE_SETTINGS", err.Error())
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return input, err
	}
	err = s.settings.SetMultiple(ctx, map[string]string{SettingKeyGroupCacheShowcaseEnabled: strconv.FormatBool(input.Enabled), SettingKeyGroupCacheShowcaseAPIEnabled: strconv.FormatBool(input.APIEnabled), SettingKeyGroupCacheShowcaseConfig: string(raw)})
	if err != nil {
		return input, err
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
	input.PelicanShowcaseConfig = cfg
	return input, nil
}
func (s *GroupCacheShowcaseService) Cleanup(ctx context.Context, now time.Time) error {
	settings, err := s.Settings(ctx)
	if err != nil {
		return err
	}
	return s.repo.Prune(ctx, settings.MaxItems, settings.retentionCutoff(now))
}
