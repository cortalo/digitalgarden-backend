package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store wraps the pool so we can hang query methods off it; consumers
// depend on small interfaces they define themselves, not on this
// concrete type.
type Store struct {
	pool *pgxpool.Pool
}

// New connects through Supabase's transaction-mode pooler, which is what
// serverless deployments (many short-lived, concurrent instances) should
// use instead of a direct connection.
func New(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	// Transaction-mode pgbouncer doesn't guarantee a session sticks to one
	// backend connection, so server-side prepared statements break.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	// Every serverless instance gets its own pool; keep each one small so
	// concurrent instances don't exhaust the shared pooler.
	cfg.MaxConns = 4

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}
