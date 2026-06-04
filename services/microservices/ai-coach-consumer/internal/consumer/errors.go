package consumer

import (
	"context"
	"errors"
	"net"

	"github.com/jackc/pgx/v5/pgconn"
)

// IsTransientError returns true for errors that may resolve on retry.
// Permanent errors (bad JSON, invalid UUID, safety block, business logic
// violations) should be sent to the DLQ rather than retried.
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}
	// Context cancellation/deadline from the consumer side is treated as
	// transient so the message can be re-processed by another consumer or
	// after restart.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// Database connection or transaction errors are usually transient.
	// pgx surfaces connection issues as generic errors or context timeouts.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// Class 08 — connection exception (transient).
		if len(pgErr.Code) >= 2 && pgErr.Code[:2] == "08" {
			return true
		}
	}
	// Network-level errors are transient.
	if _, ok := err.(net.Error); ok {
		return true
	}
	// Default to transient for unknown errors. It is safer to retry a
	// message a few extra times than to drop it permanently.
	return true
}
