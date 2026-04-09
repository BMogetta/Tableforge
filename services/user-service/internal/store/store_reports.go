package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *pgStore) CreateReport(ctx context.Context, params CreateReportParams) (PlayerReport, error) {
	row := s.db.QueryRow(ctx, `
		INSERT INTO users.player_reports (reporter_id, reported_id, reason, context)
		VALUES ($1, $2, $3, $4)
		RETURNING id, reporter_id, reported_id, reason, context,
		          status, reviewed_by, reviewed_at, resolution, ban_id, created_at
	`, params.ReporterID, params.ReportedID, params.Reason, params.Context)

	return scanReport(row)
}

func (s *pgStore) ReviewReport(ctx context.Context, params ReviewReportParams) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE users.player_reports
		SET status      = 'reviewed',
		    reviewed_by = $2,
		    reviewed_at = NOW(),
		    resolution  = $3,
		    ban_id      = $4
		WHERE id = $1 AND status = 'pending'
	`, params.ReportID, params.ReviewedBy, params.Resolution, params.BanID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) ListPendingReports(ctx context.Context, limit, offset int) ([]PlayerReport, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, reporter_id, reported_id, reason, context,
		       status, reviewed_by, reviewed_at, resolution, ban_id, created_at
		FROM users.player_reports
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReports(rows)
}

func (s *pgStore) ListReportsByPlayer(ctx context.Context, reportedID uuid.UUID) ([]PlayerReport, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, reporter_id, reported_id, reason, context,
		       status, reviewed_by, reviewed_at, resolution, ban_id, created_at
		FROM users.player_reports
		WHERE reported_id = $1
		ORDER BY created_at DESC
	`, reportedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReports(rows)
}

// --- helpers -----------------------------------------------------------------

func scanReport(row pgx.Row) (PlayerReport, error) {
	var r PlayerReport
	err := row.Scan(
		&r.ID, &r.ReporterID, &r.ReportedID, &r.Reason, &r.Context,
		&r.Status, &r.ReviewedBy, &r.ReviewedAt, &r.Resolution, &r.BanID, &r.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlayerReport{}, ErrNotFound
	}
	return r, err
}

func scanReports(rows pgx.Rows) ([]PlayerReport, error) {
	var out []PlayerReport
	for rows.Next() {
		var r PlayerReport
		if err := rows.Scan(
			&r.ID, &r.ReporterID, &r.ReportedID, &r.Reason, &r.Context,
			&r.Status, &r.ReviewedBy, &r.ReviewedAt, &r.Resolution, &r.BanID, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
