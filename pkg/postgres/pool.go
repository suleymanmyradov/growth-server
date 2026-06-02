package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OpenPool creates a new pgxpool.Pool with the given configuration.
// It validates connectivity with a Ping before returning.
func OpenPool(datasource string, maxOpen, maxIdle int, maxLifetime time.Duration) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(datasource)
	if err != nil {
		return nil, fmt.Errorf("parse pgx config: %w", err)
	}
	cfg.MaxConns = int32(maxOpen)
	cfg.MinConns = int32(maxIdle)
	cfg.MaxConnLifetime = maxLifetime

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("pgx pool: %w", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pgx ping: %w", err)
	}
	return pool, nil
}

// MustOpenPool is a convenience wrapper around OpenPool that panics on error.
func MustOpenPool(datasource string, maxOpen, maxIdle int, maxLifetime time.Duration) *pgxpool.Pool {
	pool, err := OpenPool(datasource, maxOpen, maxIdle, maxLifetime)
	if err != nil {
		panic(err)
	}
	return pool
}
