package audit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"annet-oil/internal/config"
	"annet-oil/internal/logging"
)

// Recorder records and queries audit events. Record is best-effort and never
// returns an error, so callers can invoke it unconditionally without affecting
// the operation being audited.
type Recorder interface {
	Record(ctx context.Context, e Event)
	List(ctx context.Context, f Filter) ([]Event, int, error)
	Close() error
}

// NopRecorder is used when auditing is disabled. It drops every event and needs
// no database, so all recording hooks can call Record unconditionally.
type NopRecorder struct{}

func (NopRecorder) Record(context.Context, Event) {}
func (NopRecorder) List(context.Context, Filter) ([]Event, int, error) {
	return nil, 0, fmt.Errorf("audit is disabled")
}
func (NopRecorder) Close() error { return nil }

// NewRecorder is the single factory: it returns a PostgresRecorder when
// auditing is enabled, otherwise a NopRecorder. It never returns nil.
func NewRecorder(cfg config.AuditConfig) (Recorder, error) {
	if !cfg.Enabled {
		return NopRecorder{}, nil
	}
	return NewPostgresRecorder(cfg)
}

const (
	defaultBufferSize = 1024
	flushInterval     = 2 * time.Second
	batchMax          = 100
)

// PostgresRecorder writes events asynchronously: Record enqueues onto a buffered
// channel and a background worker flushes batches to PostgreSQL. When the queue
// is full or the database is unavailable, events are logged and dropped — a
// network operation is never blocked or failed by auditing.
type PostgresRecorder struct {
	pool   *pgxpool.Pool
	events chan Event

	mu     sync.RWMutex
	closed bool
	wg     sync.WaitGroup
}

// NewPostgresRecorder connects, applies the schema idempotently, and starts the
// background writer.
func NewPostgresRecorder(cfg config.AuditConfig) (*PostgresRecorder, error) {
	dsn := dsnFromConfig(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("audit: connect to postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("audit: ping postgres: %w", err)
	}
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("audit: apply schema: %w", err)
	}

	bufSize := cfg.BufferSize
	if bufSize <= 0 {
		bufSize = defaultBufferSize
	}

	r := &PostgresRecorder{
		pool:   pool,
		events: make(chan Event, bufSize),
	}
	r.wg.Add(1)
	go r.worker()

	logging.Info("audit: postgres recorder started", "buffer_size", bufSize)
	return r, nil
}

// Record enqueues an event without blocking. A full queue drops the event.
func (r *PostgresRecorder) Record(ctx context.Context, e Event) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		return
	}

	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	// Enrich from context when the caller didn't set identity explicitly.
	if e.Actor == "" || e.Source == "" {
		a := ActorFrom(ctx)
		if e.Actor == "" {
			e.Actor = a.Name
		}
		if e.ActorRole == "" {
			e.ActorRole = a.Role
		}
		if e.Source == "" {
			e.Source = a.Source
		}
	}

	select {
	case r.events <- e:
	default:
		logging.Warn("audit: event queue full, dropping event", "action", e.Action, "actor", e.Actor)
	}
}

// worker drains the queue and flushes batches on a timer or when batchMax is
// reached.
func (r *PostgresRecorder) worker() {
	defer r.wg.Done()

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batch := make([]Event, 0, batchMax)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		r.flush(batch)
		batch = batch[:0]
	}

	for {
		select {
		case e, ok := <-r.events:
			if !ok {
				flush()
				return
			}
			batch = append(batch, e)
			if len(batch) >= batchMax {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// Close stops accepting events, flushes the remainder, and closes the pool.
func (r *PostgresRecorder) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	close(r.events)
	r.mu.Unlock()

	r.wg.Wait()
	r.pool.Close()
	return nil
}

func dsnFromConfig(cfg config.AuditConfig) string {
	if cfg.DSN != "" {
		return cfg.DSN
	}
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}
	port := cfg.Port
	if port == 0 {
		port = 5432
	}
	ssl := cfg.SSLMode
	if ssl == "" {
		ssl = "disable"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User, cfg.Password, host, port, cfg.Database, ssl)
}
