package repository

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

// B3: Pro access requires a granting status AND a current period that has
// not lapsed — a stale 'active' status alone must not unlock paid features.
func TestGrantsProAccess(t *testing.T) {
	now := time.Now()
	future := pgtype.Timestamptz{Time: now.Add(24 * time.Hour), Valid: true}
	past := pgtype.Timestamptz{Time: now.Add(-24 * time.Hour), Valid: true}

	tests := []struct {
		name      string
		planCode  string
		status    string
		periodEnd pgtype.Timestamptz
		want      bool
	}{
		{"active live period", "pro", "active", future, true},
		{"trialing live period", "pro", "trialing", future, true},
		{"past_due in grace window", "pro", "past_due", future, true},
		{"active but lapsed period", "pro", "active", past, false},
		{"trialing but lapsed period", "pro", "trialing", past, false},
		{"past_due past grace", "pro", "past_due", past, false},
		{"active no period recorded", "pro", "active", pgtype.Timestamptz{}, false},
		{"paused live period — not granting", "pro", "paused", future, false},
		{"canceled", "pro", "canceled", future, false},
		{"expired", "pro", "expired", past, false},
		{"free plan even with live period", "free", "active", future, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GrantsProAccess(tt.planCode, tt.status, tt.periodEnd, now))
		})
	}
}
