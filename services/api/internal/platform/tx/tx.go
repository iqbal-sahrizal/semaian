package tx

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Runner executes fn inside a transaction boundary.
type Runner interface {
	WithinTx(ctx context.Context, fn func(txCtx context.Context) error) error
}

// NoopRunner executes fn without starting a database transaction (for unit tests).
type NoopRunner struct{}

func (NoopRunner) WithinTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

// DBTX is satisfied by both *pgxpool.Pool and pgx.Tx.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

// PGXRunner manages PostgreSQL transactions via pgxpool.Pool.
type PGXRunner struct {
	pool *pgxpool.Pool
}

func NewPGXRunner(pool *pgxpool.Pool) *PGXRunner {
	return &PGXRunner{pool: pool}
}

// WithinTx starts a pgx.Tx (or reuses an existing one in ctx) and rolls back on error or panic.
func (r *PGXRunner) WithinTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	if _, ok := TxFromContext(ctx); ok {
		return fn(ctx)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		_ = tx.Rollback(ctx)
	}()

	txCtx := context.WithValue(ctx, txKey{}, tx)
	if err := fn(txCtx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// TxFromContext returns the active pgx.Tx from ctx if present.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	t, ok := ctx.Value(txKey{}).(pgx.Tx)
	return t, ok && t != nil
}

// Executor returns the active pgx.Tx if present in ctx, otherwise pool.
func Executor(ctx context.Context, pool *pgxpool.Pool) DBTX {
	if t, ok := TxFromContext(ctx); ok {
		return t
	}
	return pool
}
