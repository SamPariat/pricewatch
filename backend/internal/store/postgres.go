package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sampariat/prices-reminder/internal/domain"
)

// Postgres implements domain.Repository. It is the only place in the
// codebase that imports pgx — the domain layer depends on the Repository
// port, never on this package.
type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

// --- Watches ---

func (p *Postgres) CreateWatch(ctx context.Context, w domain.Watch) (domain.Watch, error) {
	row := p.pool.QueryRow(ctx, `
		INSERT INTO watches (name, kind, enabled, cron_expr, timezone, params, threshold_pct)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`,
		w.Name, string(w.Kind), w.Enabled, w.CronExpr, w.Timezone, w.Params, w.ThresholdPct,
	)
	var id string
	if err := row.Scan(&id, &w.CreatedAt); err != nil {
		return domain.Watch{}, fmt.Errorf("store: create watch: %w", err)
	}
	w.ID = domain.WatchID(id)
	return w, nil
}

func (p *Postgres) GetWatch(ctx context.Context, id domain.WatchID) (domain.Watch, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT id, name, kind, enabled, cron_expr, timezone, params, threshold_pct, created_at
		FROM watches WHERE id = $1`, string(id))
	w, err := scanWatch(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Watch{}, fmt.Errorf("store: watch %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return domain.Watch{}, fmt.Errorf("store: get watch: %w", err)
	}
	return w, nil
}

func (p *Postgres) ListWatches(ctx context.Context) ([]domain.Watch, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, name, kind, enabled, cron_expr, timezone, params, threshold_pct, created_at
		FROM watches ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("store: list watches: %w", err)
	}
	defer rows.Close()

	var out []domain.Watch
	for rows.Next() {
		w, err := scanWatch(rows)
		if err != nil {
			return nil, fmt.Errorf("store: scan watch: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (p *Postgres) UpdateWatch(ctx context.Context, w domain.Watch) (domain.Watch, error) {
	cmd, err := p.pool.Exec(ctx, `
		UPDATE watches SET name=$2, kind=$3, enabled=$4, cron_expr=$5, timezone=$6, params=$7, threshold_pct=$8
		WHERE id=$1`,
		string(w.ID), w.Name, string(w.Kind), w.Enabled, w.CronExpr, w.Timezone, w.Params, w.ThresholdPct,
	)
	if err != nil {
		return domain.Watch{}, fmt.Errorf("store: update watch: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.Watch{}, fmt.Errorf("store: watch %s: %w", w.ID, ErrNotFound)
	}
	return p.GetWatch(ctx, w.ID)
}

func (p *Postgres) DeleteWatch(ctx context.Context, id domain.WatchID) error {
	cmd, err := p.pool.Exec(ctx, `DELETE FROM watches WHERE id = $1`, string(id))
	if err != nil {
		return fmt.Errorf("store: delete watch: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("store: watch %s: %w", id, ErrNotFound)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanWatch(row rowScanner) (domain.Watch, error) {
	var w domain.Watch
	var id, kind string
	err := row.Scan(&id, &w.Name, &kind, &w.Enabled, &w.CronExpr, &w.Timezone, &w.Params, &w.ThresholdPct, &w.CreatedAt)
	w.ID = domain.WatchID(id)
	w.Kind = domain.AssetKind(kind)
	return w, err
}

// --- Quotes / samples ---

func (p *Postgres) InsertQuotes(ctx context.Context, qs []domain.Quote) error {
	if len(qs) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, q := range qs {
		fetchedDate := time.Date(q.FetchedAt.Year(), q.FetchedAt.Month(), q.FetchedAt.Day(), 0, 0, 0, 0, time.UTC)
		batch.Queue(`
			INSERT INTO quotes (watch_id, fetched_at, fetched_date, provider, price_minor, currency, depart_date, return_date, stops, carrier, deep_link, fingerprint, raw)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			ON CONFLICT (watch_id, provider, depart_date, return_date, fingerprint, fetched_date) DO NOTHING`,
			string(q.WatchID), q.FetchedAt, fetchedDate, q.Provider, q.PriceMinor, q.Currency,
			q.DepartDate, q.ReturnDate, q.Stops, q.Carrier, q.DeepLink, q.Fingerprint, q.Raw,
		)
	}
	br := p.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range qs {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("store: insert quotes: %w", err)
		}
	}
	return nil
}

func (p *Postgres) UpsertPriceSample(ctx context.Context, s domain.PriceSample) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO price_samples (watch_id, sample_date, min_minor, median_minor, max_minor, n_quotes, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (watch_id, sample_date) DO UPDATE SET
			min_minor = EXCLUDED.min_minor,
			median_minor = EXCLUDED.median_minor,
			max_minor = EXCLUDED.max_minor,
			n_quotes = EXCLUDED.n_quotes,
			updated_at = EXCLUDED.updated_at`,
		string(s.WatchID), s.SampleDate, s.MinMinor, s.MedianMinor, s.MaxMinor, s.NQuotes, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("store: upsert price sample: %w", err)
	}
	return nil
}

func (p *Postgres) ListPriceSamples(ctx context.Context, watchID domain.WatchID, since time.Time) ([]domain.PriceSample, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT watch_id, sample_date, min_minor, median_minor, max_minor, n_quotes, updated_at
		FROM price_samples WHERE watch_id = $1 AND sample_date >= $2 ORDER BY sample_date`,
		string(watchID), since,
	)
	if err != nil {
		return nil, fmt.Errorf("store: list price samples: %w", err)
	}
	defer rows.Close()

	var out []domain.PriceSample
	for rows.Next() {
		var s domain.PriceSample
		var wid string
		if err := rows.Scan(&wid, &s.SampleDate, &s.MinMinor, &s.MedianMinor, &s.MaxMinor, &s.NQuotes, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("store: scan price sample: %w", err)
		}
		s.WatchID = domain.WatchID(wid)
		out = append(out, s)
	}
	return out, rows.Err()
}

// --- Runs ---

func (p *Postgres) CreateDigestRun(ctx context.Context, r domain.DigestRun) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO digest_runs (run_id, watch_id, started_at, status, message_body, error)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		string(r.RunID), string(r.WatchID), r.StartedAt, string(r.Status), r.MessageBody, r.Error,
	)
	if err != nil {
		return fmt.Errorf("store: create digest run: %w", err)
	}
	return nil
}

func (p *Postgres) FinishDigestRun(ctx context.Context, r domain.DigestRun) error {
	_, err := p.pool.Exec(ctx, `
		UPDATE digest_runs SET finished_at=$2, status=$3, message_body=$4, error=$5
		WHERE run_id=$1`,
		string(r.RunID), r.FinishedAt, string(r.Status), r.MessageBody, r.Error,
	)
	if err != nil {
		return fmt.Errorf("store: finish digest run: %w", err)
	}
	return nil
}

func (p *Postgres) ListRecentDigestRuns(ctx context.Context, limit int) ([]domain.DigestRun, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT run_id, watch_id, started_at, finished_at, status, message_body, error
		FROM digest_runs ORDER BY started_at DESC LIMIT $1`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("store: list recent digest runs: %w", err)
	}
	defer rows.Close()

	var out []domain.DigestRun
	for rows.Next() {
		var r domain.DigestRun
		var runID, watchID, status string
		if err := rows.Scan(&runID, &watchID, &r.StartedAt, &r.FinishedAt, &status, &r.MessageBody, &r.Error); err != nil {
			return nil, fmt.Errorf("store: scan digest run: %w", err)
		}
		r.RunID = domain.RunID(runID)
		r.WatchID = domain.WatchID(watchID)
		r.Status = domain.RunStatus(status)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (p *Postgres) InsertRunEvent(ctx context.Context, e domain.RunEvent) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO run_events (run_id, at, stage, level, msg, fields)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		string(e.RunID), e.At, e.Stage, string(e.Level), e.Msg, e.Fields,
	)
	if err != nil {
		return fmt.Errorf("store: insert run event: %w", err)
	}
	return nil
}

func (p *Postgres) ListRunEvents(ctx context.Context, runID domain.RunID) ([]domain.RunEvent, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT run_id, at, stage, level, msg, fields FROM run_events WHERE run_id = $1 ORDER BY at`,
		string(runID),
	)
	if err != nil {
		return nil, fmt.Errorf("store: list run events: %w", err)
	}
	defer rows.Close()

	var out []domain.RunEvent
	for rows.Next() {
		var e domain.RunEvent
		var rid, level string
		if err := rows.Scan(&rid, &e.At, &e.Stage, &level, &e.Msg, &e.Fields); err != nil {
			return nil, fmt.Errorf("store: scan run event: %w", err)
		}
		e.RunID = domain.RunID(rid)
		e.Level = domain.LogLevel(level)
		out = append(out, e)
	}
	return out, rows.Err()
}

// --- Watch state ---

func (p *Postgres) GetWatchState(ctx context.Context, watchID domain.WatchID) (domain.WatchState, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT watch_id, last_success_at, last_attempt_at, last_error, consecutive_failures
		FROM watch_state WHERE watch_id = $1`, string(watchID))

	var s domain.WatchState
	var wid string
	err := row.Scan(&wid, &s.LastSuccessAt, &s.LastAttemptAt, &s.LastError, &s.ConsecutiveFailures)
	if errors.Is(err, pgx.ErrNoRows) {
		// No state yet is not an error — a watch that has never run has a
		// zero-value state, which is exactly what the staleness badge
		// logic in PLAN.md expects to see for a brand-new watch.
		return domain.WatchState{WatchID: watchID}, nil
	}
	if err != nil {
		return domain.WatchState{}, fmt.Errorf("store: get watch state: %w", err)
	}
	s.WatchID = domain.WatchID(wid)
	return s, nil
}

func (p *Postgres) UpsertWatchState(ctx context.Context, s domain.WatchState) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO watch_state (watch_id, last_success_at, last_attempt_at, last_error, consecutive_failures)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (watch_id) DO UPDATE SET
			last_success_at = EXCLUDED.last_success_at,
			last_attempt_at = EXCLUDED.last_attempt_at,
			last_error = EXCLUDED.last_error,
			consecutive_failures = EXCLUDED.consecutive_failures`,
		string(s.WatchID), s.LastSuccessAt, s.LastAttemptAt, s.LastError, s.ConsecutiveFailures,
	)
	if err != nil {
		return fmt.Errorf("store: upsert watch state: %w", err)
	}
	return nil
}

// --- Settings (singleton row) ---

func (p *Postgres) GetSettings(ctx context.Context) (domain.Settings, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT telegram_chat_id, quiet_hours_start, quiet_hours_end, currency, dry_run
		FROM settings WHERE id = true`)
	var s domain.Settings
	if err := row.Scan(&s.TelegramChatID, &s.QuietHoursStart, &s.QuietHoursEnd, &s.Currency, &s.DryRun); err != nil {
		return domain.Settings{}, fmt.Errorf("store: get settings: %w", err)
	}
	return s, nil
}

func (p *Postgres) UpdateSettings(ctx context.Context, s domain.Settings) error {
	_, err := p.pool.Exec(ctx, `
		UPDATE settings SET telegram_chat_id=$1, quiet_hours_start=$2, quiet_hours_end=$3, currency=$4, dry_run=$5
		WHERE id = true`,
		s.TelegramChatID, s.QuietHoursStart, s.QuietHoursEnd, s.Currency, s.DryRun,
	)
	if err != nil {
		return fmt.Errorf("store: update settings: %w", err)
	}
	return nil
}

var ErrNotFound = errors.New("not found")
