package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type groupCacheShowcaseRepository struct{ db *sql.DB }

func NewGroupCacheShowcaseRepository(db *sql.DB) service.GroupCacheShowcaseRepository {
	return &groupCacheShowcaseRepository{db: db}
}
func cachePublicationError(err error) error {
	var pg *pq.Error
	if errors.As(err, &pg) && pg.Code == "23505" {
		return infraerrors.Conflict("CACHE_SHOWCASE_DUPLICATE", "this test or one of its requests is already published")
	}
	return err
}
func (r *groupCacheShowcaseRepository) Publish(ctx context.Context, item *service.GroupCacheSnapshot) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Serialize publications per group, including the retention trim.
	var id int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM groups WHERE id=$1 AND deleted_at IS NULL AND status=$2 FOR UPDATE`, item.GroupID, service.StatusActive).Scan(&id); err != nil {
		return err
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO group_cache_showcase_runs (run_id,created_by,group_id,model,protocol,tested_at)
 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id,published_at`, item.RunID, item.CreatedBy, item.GroupID, item.Model, item.Protocol, item.TestedAt).Scan(&item.ID, &item.PublishedAt)
	if err != nil {
		return cachePublicationError(err)
	}
	for _, round := range item.Rounds {
		_, err = tx.ExecContext(ctx, `INSERT INTO group_cache_showcase_rounds (run_id,round_index,usage_log_id,input_tokens,cache_read_tokens,first_token_ms,measured_at)
 VALUES ($1,$2,$3,$4,$5,$6,$7)`, item.ID, round.Index, round.UsageLogID, round.InputTokens, round.CacheReadTokens, round.FirstTokenMs, round.MeasuredAt)
		if err != nil {
			return cachePublicationError(err)
		}
	}
	// Bounded history; this limit is also shown in the UI and documentation.
	_, err = tx.ExecContext(ctx, `DELETE FROM group_cache_showcase_runs WHERE id IN
 (SELECT id FROM group_cache_showcase_runs WHERE group_id=$1 ORDER BY tested_at DESC,id DESC OFFSET 100)`, item.GroupID)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *groupCacheShowcaseRepository) List(ctx context.Context, groupID int64) ([]*service.GroupCacheSnapshot, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT r.id,r.run_id,r.created_by,r.group_id,g.name,r.model,r.protocol,r.tested_at,r.published_at
 FROM group_cache_showcase_runs r JOIN groups g ON g.id=r.group_id
 WHERE r.group_id=$1 ORDER BY r.tested_at DESC,r.id DESC LIMIT 100`, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []*service.GroupCacheSnapshot{}
	for rows.Next() {
		item := &service.GroupCacheSnapshot{}
		if err := rows.Scan(&item.ID, &item.RunID, &item.CreatedBy, &item.GroupID, &item.GroupName, &item.Model, &item.Protocol, &item.TestedAt, &item.PublishedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (r *groupCacheShowcaseRepository) Delete(ctx context.Context, id int64) (bool, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM group_cache_showcase_runs WHERE id=$1`, id)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
func (r *groupCacheShowcaseRepository) Summary(ctx context.Context, since, now time.Time) ([]*service.GroupCacheShowcaseItem, error) {
	// Filter individual measurement times, not publication/run times. A run may
	// straddle the rolling window. First rounds never contribute to warm metrics.
	rows, err := r.db.QueryContext(ctx, `SELECT g.id,g.name,g.platform,r.model,r.protocol,MAX(r.tested_at),
 COUNT(p.run_id),COUNT(p.first_token_ms),COALESCE(SUM(p.input_tokens),0),COALESCE(SUM(p.cache_read_tokens),0),AVG(p.first_token_ms)
 FROM group_cache_showcase_runs r JOIN groups g ON g.id=r.group_id
 LEFT JOIN group_cache_showcase_rounds p ON p.run_id=r.id AND p.round_index>1 AND p.measured_at >= $1 AND p.measured_at <= $2
 WHERE g.deleted_at IS NULL AND g.status=$3
 GROUP BY g.id,g.name,g.platform,g.sort_order,r.model,r.protocol
 ORDER BY g.sort_order,g.id,r.model,r.protocol`, since, now, service.StatusActive)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []*service.GroupCacheShowcaseItem{}
	for rows.Next() {
		item := &service.GroupCacheShowcaseItem{}
		if err := rows.Scan(&item.GroupID, &item.GroupName, &item.Platform, &item.Model, &item.Protocol, &item.LastTestedAt, &item.Samples, &item.LatencySamples, &item.InputTokens, &item.CacheReadTokens, &item.FirstTokenMs); err != nil {
			return nil, err
		}
		if item.InputTokens > 0 {
			rate := float64(item.CacheReadTokens) / float64(item.InputTokens)
			item.CacheHitRate = &rate
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
