package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/semaian/platform/services/api/internal/platform/tx"
)

type txCtxKey struct{}

// WithTx attaches a pgx.Tx to context so PGXRecorder.Record participates in the caller's transaction.
func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txCtxKey{}, tx)
}

// Event represents an append-only security/business audit record.
type Event struct {
	ID             string
	ActorID        *string
	OrganizationID *string
	Action         string
	TargetType     string
	TargetID       string
	CorrelationID  string
	Metadata       map[string]any
	CreatedAt      time.Time
}

// Recorder records audit events.
type Recorder interface {
	Record(ctx context.Context, ev Event) error
}

// PGXRecorder writes audit events to audit.events (INSERT only).
type PGXRecorder struct {
	pool *pgxpool.Pool
}

// NewPGXRecorder creates a PostgreSQL audit recorder.
func NewPGXRecorder(pool *pgxpool.Pool) *PGXRecorder {
	return &PGXRecorder{pool: pool}
}

// Record inserts an immutable audit event using the context's pgx.Tx if present, otherwise the pool.
func (r *PGXRecorder) Record(ctx context.Context, ev Event) error {
	if ev.ID == "" {
		ev.ID = uuid.NewString()
	}
	metaBytes, err := json.Marshal(ev.Metadata)
	if err != nil {
		metaBytes = []byte("{}")
	}
	const q = `INSERT INTO audit.events (id, actor_id, organization_id, action, target_type, target_id, correlation_id, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	if t, ok := ctx.Value(txCtxKey{}).(pgx.Tx); ok && t != nil {
		_, err = t.Exec(ctx, q, ev.ID, ev.ActorID, ev.OrganizationID, ev.Action, ev.TargetType, ev.TargetID, ev.CorrelationID, metaBytes)
		return err
	}
	_, err = tx.Executor(ctx, r.pool).Exec(ctx, q, ev.ID, ev.ActorID, ev.OrganizationID, ev.Action, ev.TargetType, ev.TargetID, ev.CorrelationID, metaBytes)
	return err
}

// NoopRecorder is a no-op Recorder for unit tests.
type NoopRecorder struct{}

func (NoopRecorder) Record(_ context.Context, _ Event) error { return nil }
