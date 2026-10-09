package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
)

// TestBillingWrites_RejectNilUserID: every billing write must refuse uuid.Nil
// before reaching the database. The repo has no db wired, so a write that
// slipped past the guard would panic instead of returning ErrNilUserID.
func TestBillingWrites_RejectNilUserID(t *testing.T) {
	r := &billingRepo{}
	ctx := context.Background()

	tests := []struct {
		name  string
		write func() error
	}{
		{"GetOrCreateUserSubscription", func() error {
			_, err := r.GetOrCreateUserSubscription(ctx, uuid.Nil)
			return err
		}},
		{"CreateDefaultFreeSubscription", func() error {
			_, err := r.CreateDefaultFreeSubscription(ctx, uuid.Nil)
			return err
		}},
		{"ApplyMergedSubscription", func() error {
			_, err := r.ApplyMergedSubscription(ctx, db.ApplyMergedSubscriptionParams{UserID: uuid.Nil})
			return err
		}},
		{"UpsertSubscriptionProviderState", func() error {
			_, err := r.UpsertSubscriptionProviderState(ctx, db.UpsertSubscriptionProviderStateParams{UserID: uuid.Nil})
			return err
		}},
		{"LinkPaddleProviderIDs", func() error {
			return r.LinkPaddleProviderIDs(ctx, uuid.Nil, nil, nil, pgtype.Timestamptz{})
		}},
		{"RecordPaddleCheckout", func() error {
			return r.RecordPaddleCheckout(ctx, "txn_01", uuid.Nil)
		}},
		{"CreateUpgradeEvent", func() error {
			_, err := r.CreateUpgradeEvent(ctx, db.CreateUpgradeEventParams{UserID: uuid.Nil})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ErrorIs(t, tt.write(), ErrNilUserID)
		})
	}
}
