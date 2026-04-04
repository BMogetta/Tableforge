package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AuditLog represents a single admin audit log entry.
type AuditLog struct {
	ID         uuid.UUID        `json:"id"`
	ActorID    uuid.UUID        `json:"actor_id"`
	Action     string           `json:"action"`
	TargetType string           `json:"target_type"`
	TargetID   string           `json:"target_id"`
	Details    json.RawMessage  `json:"details,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
}

// AuditFilter holds optional filters for listing audit logs.
type AuditFilter struct {
	ActorID    *uuid.UUID
	Action     *string
	TargetType *string
	From       *time.Time
	To         *time.Time
	Limit      int
	Offset     int
}

func (s *pgStore) LogAction(ctx context.Context, actorID uuid.UUID, action, targetType, targetID string, details map[string]any) error {
	var detailsJSON []byte
	if details != nil {
		var err error
		detailsJSON, err = json.Marshal(details)
		if err != nil {
			return fmt.Errorf("LogAction marshal details: %w", err)
		}
	}

	_, err := s.db.Exec(ctx,
		`INSERT INTO audit_logs (actor_id, action, target_type, target_id, details)
		 VALUES ($1, $2, $3, $4, $5)`,
		actorID, action, targetType, targetID, detailsJSON,
	)
	if err != nil {
		return fmt.Errorf("LogAction: %w", err)
	}
	return nil
}

func (s *pgStore) ListAuditLogs(ctx context.Context, filter AuditFilter) ([]AuditLog, error) {
	var (
		clauses []string
		args    []any
		idx     = 1
	)

	if filter.ActorID != nil {
		clauses = append(clauses, fmt.Sprintf("actor_id = $%d", idx))
		args = append(args, *filter.ActorID)
		idx++
	}
	if filter.Action != nil {
		clauses = append(clauses, fmt.Sprintf("action = $%d", idx))
		args = append(args, *filter.Action)
		idx++
	}
	if filter.TargetType != nil {
		clauses = append(clauses, fmt.Sprintf("target_type = $%d", idx))
		args = append(args, *filter.TargetType)
		idx++
	}
	if filter.From != nil {
		clauses = append(clauses, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, *filter.From)
		idx++
	}
	if filter.To != nil {
		clauses = append(clauses, fmt.Sprintf("created_at <= $%d", idx))
		args = append(args, *filter.To)
		idx++
	}

	query := "SELECT id, actor_id, action, target_type, target_id, details, created_at FROM audit_logs"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY created_at DESC"

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, filter.Offset)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ListAuditLogs: %w", err)
	}
	defer rows.Close()

	logs := []AuditLog{}
	for rows.Next() {
		var l AuditLog
		if err := rows.Scan(&l.ID, &l.ActorID, &l.Action, &l.TargetType, &l.TargetID, &l.Details, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("ListAuditLogs scan: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
