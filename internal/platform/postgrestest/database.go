// Package postgrestest opens only explicitly configured disposable test databases.
package postgrestest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(t *testing.T, key string) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv(key)
	if dsn == "" {
		t.Skip("set " + key + " to run PostgreSQL integration tests")
	}
	cfg, err := databaseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	return pool
}

func databaseConfig(dsn string) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(cfg.ConnConfig.Database, "_test") {
		return nil, fmt.Errorf("integration tests require a database ending in _test")
	}
	return cfg, nil
}
