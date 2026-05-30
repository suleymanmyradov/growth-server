// Package postgres provides database helpers for RLS and transaction management.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

// TxRunner executes a block of work inside a PostgreSQL transaction that has
// Row-Level Security (RLS) user context set via `app.current_user_id`.
type TxRunner struct{ db *sql.DB }

// NewTxRunner creates a runner backed by db.
func NewTxRunner(db *sql.DB) *TxRunner { return &TxRunner{db: db} }

// Run begins a transaction, sets the RLS user context, executes fn, and commits.
// If fn returns an error the transaction is rolled back.
func (r *TxRunner) Run(ctx context.Context, userID string, fn func(*sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := setRLSUser(ctx, tx, userID); err != nil {
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
// Use this when the fn reads data then writes derived values (e.g. streak calculations).
func (r *TxRunner) RunSerializable(ctx context.Context, userID string, fn func(*sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("begin serializable tx: %w", err)
	}
	defer tx.Rollback()

	if err := setRLSUser(ctx, tx, userID); err != nil {
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

func setRLSUser(ctx context.Context, tx *sql.Tx, userID string) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty for RLS")
	}
	_, err := tx.ExecContext(ctx, "SET LOCAL app.current_user_id = $1", userID)
	return err
}
