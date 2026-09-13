package audit

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"annet-oil/internal/logging"
)

//go:embed schema.sql
var schemaSQL string

// Filter selects and paginates audit events for List. Zero-value fields are
// ignored. Results are always ordered by timestamp DESC.
type Filter struct {
	Actor   string
	Device  string
	Action  string
	Source  string
	From    time.Time
	To      time.Time
	Success *bool
	Limit   int
	Offset  int
}

const insertSQL = `INSERT INTO audit_events
    (timestamp, actor, actor_role, source, action, devices, command, params, success, duration_ms, error_type, error_message, request_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,$11,$12,$13)`

// flush writes a batch of events in a single round-trip. Failures are logged and
// the batch is dropped — auditing must never block or fail the audited action.
func (r *PostgresRecorder) flush(events []Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	batch := &pgx.Batch{}
	for _, e := range events {
		var params any
		if len(e.Params) > 0 {
			if b, err := json.Marshal(e.Params); err == nil {
				params = string(b)
			}
		}
		var errType, errMsg any
		if e.Error != nil {
			errType = e.Error.Type
			errMsg = e.Error.Message
		}
		var devices any
		if len(e.Devices) > 0 {
			devices = e.Devices
		}
		batch.Queue(insertSQL,
			e.Timestamp, e.Actor, nullIfEmpty(e.ActorRole), e.Source, e.Action,
			devices, nullIfEmpty(e.Command), params, e.Success,
			nullIfZero(e.DurationMs), errType, errMsg, nullIfEmpty(e.RequestID))
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range events {
		if _, err := br.Exec(); err != nil {
			logging.Error("audit: batch insert failed", "error", err, "batch_size", len(events))
			return
		}
	}
}

// List returns matching events (timestamp DESC) plus the total count ignoring
// pagination.
func (r *PostgresRecorder) List(ctx context.Context, f Filter) ([]Event, int, error) {
	where, args := buildWhere(f)

	total, err := r.count(ctx, where, args)
	if err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := `SELECT id, timestamp, actor, actor_role, source, action, devices, command, params,
        success, duration_ms, error_type, error_message, request_id
        FROM audit_events` + where + ` ORDER BY timestamp DESC`
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, limit, f.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("audit: query events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, 0, err
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return events, total, nil
}

func (r *PostgresRecorder) count(ctx context.Context, where string, args []any) (int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events`+where, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("audit: count events: %w", err)
	}
	return total, nil
}

// buildWhere assembles a parameterized WHERE clause (SQL-injection safe).
func buildWhere(f Filter) (string, []any) {
	var conds []string
	var args []any
	add := func(cond string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}

	if f.Actor != "" {
		add("actor = $%d", f.Actor)
	}
	if f.Action != "" {
		add("action = $%d", f.Action)
	}
	if f.Source != "" {
		add("source = $%d", f.Source)
	}
	if f.Device != "" {
		add("$%d = ANY(devices)", f.Device)
	}
	if !f.From.IsZero() {
		add("timestamp >= $%d", f.From)
	}
	if !f.To.IsZero() {
		add("timestamp <= $%d", f.To)
	}
	if f.Success != nil {
		add("success = $%d", *f.Success)
	}

	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func scanEvent(rows pgx.Rows) (Event, error) {
	var (
		e         Event
		role      *string
		command   *string
		params    []byte
		duration  *int64
		errType   *string
		errMsg    *string
		requestID *string
	)
	if err := rows.Scan(&e.ID, &e.Timestamp, &e.Actor, &role, &e.Source, &e.Action,
		&e.Devices, &command, &params, &e.Success, &duration, &errType, &errMsg, &requestID); err != nil {
		return Event{}, fmt.Errorf("audit: scan event: %w", err)
	}
	if role != nil {
		e.ActorRole = *role
	}
	if command != nil {
		e.Command = *command
	}
	if duration != nil {
		e.DurationMs = *duration
	}
	if requestID != nil {
		e.RequestID = *requestID
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &e.Params)
	}
	if errType != nil || errMsg != nil {
		e.Error = &Error{}
		if errType != nil {
			e.Error.Type = *errType
		}
		if errMsg != nil {
			e.Error.Message = *errMsg
		}
	}
	return e, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullIfZero(n int64) any {
	if n == 0 {
		return nil
	}
	return n
}
