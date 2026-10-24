package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BeginOperation serializes writes for a caller-assigned operation ID, including
// IDs that do not yet have a resource row. The lock is local to this database.
func BeginOperation(ctx context.Context, pool *pgxpool.Pool, id string) (pgx.Tx, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", id); err != nil {
		Rollback(tx)
		return nil, err
	}
	return tx, nil
}

func Rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
