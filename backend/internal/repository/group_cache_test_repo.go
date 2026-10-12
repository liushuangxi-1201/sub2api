package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type groupCacheTestRepository struct{ db *sql.DB }

func NewGroupCacheTestRepository(db *sql.DB) service.GroupCacheTestRepository {
	return &groupCacheTestRepository{db: db}
}

const cachePlanSelect = `SELECT p.id,p.group_id,p.created_by,g.name,g.platform,
 CASE WHEN g.deleted_at IS NULL THEN g.status ELSE 'deleted' END,
 p.model_id,p.cron_expression,p.enabled,p.config,p.last_run_at,p.next_run_at,p.running_until,p.updated_at,
 COALESCE(c.today,0),COALESCE(c.total,0),COALESCE(c.today_partial,false),COALESCE(c.total_partial,false),r.result,r.id,r.publication_id
 FROM group_cache_test_plans p JOIN groups g ON g.id=p.group_id
 LEFT JOIN LATERAL (SELECT SUM(cost_usd) FILTER(WHERE cost_date=$1::date) today,SUM(cost_usd) total,
 BOOL_OR(unpriced_count>0) FILTER(WHERE cost_date=$1::date) today_partial,BOOL_OR(unpriced_count>0) total_partial
 FROM group_cache_test_daily_costs WHERE group_id=p.group_id) c ON true
 LEFT JOIN LATERAL (SELECT t.result,t.id,s.id publication_id FROM group_cache_test_results t
 LEFT JOIN group_cache_showcase_runs s ON s.id=t.publication_id WHERE t.plan_id=p.id ORDER BY t.id DESC LIMIT 1) r ON true`

func scanCachePlan(row rowScanner) (*service.GroupCacheTestPlan, error) {
	p := &service.GroupCacheTestPlan{}
	var config, raw []byte
	var resultID, publication sql.NullInt64
	err := row.Scan(&p.ID, &p.GroupID, &p.CreatedBy, &p.GroupName, &p.GroupPlatform, &p.GroupStatus, &p.ModelID, &p.CronExpression, &p.Enabled, &config, &p.LastRunAt, &p.NextRunAt, &p.RunningUntil, &p.UpdatedAt, &p.TodayCostUSD, &p.TotalCostUSD, &p.TodayCostIncomplete, &p.TotalCostIncomplete, &raw, &resultID, &publication)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(config, &p.Config); err != nil {
		return nil, err
	}
	if len(raw) > 0 {
		p.LastResult = &service.GroupCacheTestResult{}
		if err = json.Unmarshal(raw, p.LastResult); err != nil {
			return nil, err
		}
		p.LastResult.ID = resultID.Int64
		p.LastResult.PublicationID = nil
		if publication.Valid {
			p.LastResult.PublicationID = &publication.Int64
		}
	}
	return p, nil
}
func (r *groupCacheTestRepository) plans(ctx context.Context, suffix string, args ...any) ([]*service.GroupCacheTestPlan, error) {
	rows, err := r.db.QueryContext(ctx, cachePlanSelect+suffix, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []*service.GroupCacheTestPlan{}
	for rows.Next() {
		p, e := scanCachePlan(rows)
		if e != nil {
			return nil, e
		}
		items = append(items, p)
	}
	return items, rows.Err()
}
func (r *groupCacheTestRepository) ListPlans(ctx context.Context) ([]*service.GroupCacheTestPlan, error) {
	return r.plans(ctx, ` ORDER BY g.sort_order,g.id,p.id`, timezone.Now().Format(time.DateOnly))
}
func (r *groupCacheTestRepository) GetPlan(ctx context.Context, id int64) (*service.GroupCacheTestPlan, error) {
	p, e := scanCachePlan(r.db.QueryRowContext(ctx, cachePlanSelect+` WHERE p.id=$2`, timezone.Now().Format(time.DateOnly), id))
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	return p, e
}
func (r *groupCacheTestRepository) SavePlan(ctx context.Context, p *service.GroupCacheTestPlan) (*service.GroupCacheTestPlan, error) {
	config, err := json.Marshal(p.Config)
	if err != nil {
		return nil, err
	}
	if p.ID == 0 {
		err = r.db.QueryRowContext(ctx, `INSERT INTO group_cache_test_plans(group_id,created_by,model_id,cron_expression,enabled,config,next_run_at) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, p.GroupID, p.CreatedBy, p.ModelID, p.CronExpression, p.Enabled, config, p.NextRunAt).Scan(&p.ID)
	} else {
		var result sql.Result
		result, err = r.db.ExecContext(ctx, `UPDATE group_cache_test_plans SET model_id=$2,cron_expression=$3,enabled=$4,config=$5,next_run_at=$6,updated_at=NOW() WHERE id=$1 AND updated_at=$7`, p.ID, p.ModelID, p.CronExpression, p.Enabled, config, p.NextRunAt, p.UpdatedAt)
		if err == nil {
			n, e := result.RowsAffected()
			if e != nil {
				return nil, e
			}
			if n == 0 {
				return nil, service.ErrGroupCacheTestRunning
			}
		}
	}
	if err != nil {
		return nil, err
	}
	return r.GetPlan(ctx, p.ID)
}
func (r *groupCacheTestRepository) DeletePlan(ctx context.Context, id int64) (bool, error) {
	res, e := r.db.ExecContext(ctx, `DELETE FROM group_cache_test_plans WHERE id=$1`, id)
	if e != nil {
		return false, e
	}
	n, e := res.RowsAffected()
	return n > 0, e
}
func (r *groupCacheTestRepository) ListDue(ctx context.Context, now time.Time) ([]*service.GroupCacheTestPlan, error) {
	return r.plans(ctx, ` WHERE p.enabled AND p.next_run_at<=$2 AND (p.running_until IS NULL OR p.running_until<$2) AND g.deleted_at IS NULL AND g.status='active' ORDER BY p.next_run_at,p.id`, now.In(timezone.Location()).Format(time.DateOnly), now)
}
func (r *groupCacheTestRepository) Claim(ctx context.Context, p *service.GroupCacheTestPlan, now, until time.Time, next *time.Time) (bool, error) {
	var res sql.Result
	var e error
	if next == nil {
		res, e = r.db.ExecContext(ctx, `UPDATE group_cache_test_plans SET running_until=$3 WHERE id=$1 AND (running_until IS NULL OR running_until<$2) AND updated_at=$4 AND EXISTS(SELECT 1 FROM groups WHERE id=group_id AND deleted_at IS NULL AND status='active')`, p.ID, now, until, p.UpdatedAt)
	} else {
		res, e = r.db.ExecContext(ctx, `UPDATE group_cache_test_plans SET running_until=$3,next_run_at=$4 WHERE id=$1 AND enabled AND next_run_at<=$2 AND (running_until IS NULL OR running_until<$2) AND updated_at=$5 AND EXISTS(SELECT 1 FROM groups WHERE id=group_id AND deleted_at IS NULL AND status='active')`, p.ID, now, until, *next, p.UpdatedAt)
	}
	if e != nil {
		return false, e
	}
	n, e := res.RowsAffected()
	return n == 1, e
}
func (r *groupCacheTestRepository) Finish(ctx context.Context, id int64, until, finished time.Time) error {
	_, e := r.db.ExecContext(ctx, `UPDATE group_cache_test_plans SET running_until=NULL,last_run_at=$3 WHERE id=$1 AND running_until=$2`, id, until, finished)
	return e
}
func (r *groupCacheTestRepository) SaveResult(ctx context.Context, v *service.GroupCacheTestResult) error {
	raw, e := json.Marshal(v)
	if e != nil {
		return e
	}
	_, e = r.db.ExecContext(ctx, `WITH saved AS (INSERT INTO group_cache_test_results(plan_id,result,publication_id,started_at,finished_at) VALUES($1,$2,$3,$4,$5) RETURNING plan_id)
 INSERT INTO group_cache_test_daily_costs(group_id,cost_date,cost_usd,unpriced_count)
 SELECT p.group_id,$6::date,$7,$8 FROM saved JOIN group_cache_test_plans p ON p.id=saved.plan_id
 ON CONFLICT(group_id,cost_date) DO UPDATE SET cost_usd=group_cache_test_daily_costs.cost_usd+EXCLUDED.cost_usd,unpriced_count=group_cache_test_daily_costs.unpriced_count+EXCLUDED.unpriced_count`, v.PlanID, raw, v.PublicationID, v.StartedAt, v.FinishedAt, v.FinishedAt.In(timezone.Location()).Format(time.DateOnly), cacheKnownCost(v.CostUSD), cacheIncomplete(v.CostUSD, v.CostIncomplete))
	return e
}
func cacheKnownCost(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
func cacheIncomplete(v *float64, incomplete bool) int {
	if v == nil || incomplete {
		return 1
	}
	return 0
}

const cacheResultSelect = `SELECT t.id,t.result,g.name,s.id FROM group_cache_test_results t JOIN group_cache_test_plans p ON p.id=t.plan_id JOIN groups g ON g.id=p.group_id LEFT JOIN group_cache_showcase_runs s ON s.id=t.publication_id`

func scanCacheResult(row rowScanner) (*service.GroupCacheTestResult, error) {
	v := &service.GroupCacheTestResult{}
	var raw []byte
	var id int64
	var name string
	var publication sql.NullInt64
	e := row.Scan(&id, &raw, &name, &publication)
	if e != nil {
		return nil, e
	}
	if e = json.Unmarshal(raw, v); e != nil {
		return nil, e
	}
	v.ID = id
	v.GroupName = name
	v.PublicationID = nil
	if publication.Valid {
		v.PublicationID = &publication.Int64
	}
	return v, nil
}
func (r *groupCacheTestRepository) ListResults(ctx context.Context, planID int64, offset, limit int) ([]*service.GroupCacheTestResult, int64, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM group_cache_test_results WHERE ($1::bigint=0 OR plan_id=$1)`, planID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, cacheResultSelect+` WHERE($1::bigint=0 OR t.plan_id=$1) ORDER BY t.id DESC OFFSET $2 LIMIT $3`, planID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := []*service.GroupCacheTestResult{}
	for rows.Next() {
		v, e := scanCacheResult(rows)
		if e != nil {
			return nil, 0, e
		}
		items = append(items, v)
	}
	return items, total, rows.Err()
}
func (r *groupCacheTestRepository) GetResult(ctx context.Context, id int64) (*service.GroupCacheTestResult, error) {
	v, e := scanCacheResult(r.db.QueryRowContext(ctx, cacheResultSelect+` WHERE t.id=$1`, id))
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	return v, e
}
func (r *groupCacheTestRepository) Prune(ctx context.Context, before time.Time) error {
	_, e := r.db.ExecContext(ctx, `DELETE FROM group_cache_test_results WHERE finished_at<$1 OR id IN(SELECT id FROM(SELECT id,ROW_NUMBER() OVER(PARTITION BY plan_id ORDER BY id DESC) rn FROM group_cache_test_results)x WHERE rn>100)`, before)
	return e
}
