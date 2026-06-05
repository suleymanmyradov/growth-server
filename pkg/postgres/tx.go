// Package postgres provides database helpers for RLS and transaction management.
package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TxRunner executes a block of work inside a PostgreSQL transaction that has
// Row-Level Security (RLS) user context set via `app.current_user_id`.
type TxRunner struct{ db *sql.DB }

// NewTxRunner creates a runner backed by a database/sql DB.
func NewTxRunner(db *sql.DB) *TxRunner { return &TxRunner{db: db} }

// Run begins a transaction, sets the RLS user context, executes fn, and commits.
// If fn returns an error the transaction is rolled back.
func (r *TxRunner) Run(ctx context.Context, userID string, fn func(*sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := setRLSUserSQL(ctx, tx, userID); err != nil {
		return fmt.Errorf("set rls user: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// RunSerializable is like Run but uses SERIALIZABLE isolation.
func (r *TxRunner) RunSerializable(ctx context.Context, userID string, fn func(*sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("begin serializable tx: %w", err)
	}
	defer tx.Rollback()

	if err := setRLSUserSQL(ctx, tx, userID); err != nil {
		return fmt.Errorf("set rls user: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func setRLSUserSQL(ctx context.Context, tx *sql.Tx, userID string) error {
	if userID == "" {
		return nil
	}
	_, err := tx.ExecContext(ctx, "SELECT set_config('app.current_user_id', $1, true)", userID)
	return err
}

// PgxTxRunner is the pgx/v5 variant of TxRunner.
type PgxTxRunner struct{ db *pgxpool.Pool }

// NewPgxTxRunner creates a runner backed by a pgx pool.
func NewPgxTxRunner(db *pgxpool.Pool) *PgxTxRunner { return &PgxTxRunner{db: db} }

// Run begins a transaction, sets the RLS user context, executes fn, and commits.
// If fn returns an error the transaction is rolled back.
func (r *PgxTxRunner) Run(ctx context.Context, userID string, fn func(pgx.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := setRLSUserPgx(ctx, tx, userID); err != nil {
		return fmt.Errorf("set rls user: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// RunSerializable is like Run but uses SERIALIZABLE isolation.
func (r *PgxTxRunner) RunSerializable(ctx context.Context, userID string, fn func(pgx.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin serializable tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := setRLSUserPgx(ctx, tx, userID); err != nil {
		return fmt.Errorf("set rls user: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func setRLSUserPgx(ctx context.Context, tx pgx.Tx, userID string) error {
	if userID == "" {
		return nil
	}
	_, err := tx.Exec(ctx, "SELECT set_config('app.current_user_id', $1, true)", userID)
	return err
}
