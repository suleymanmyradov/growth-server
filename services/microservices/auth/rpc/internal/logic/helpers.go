package logic

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"github.com/zeromicro/go-zero/core/logx"
)

func toNullString(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func formatTime(t pgtype.Timestamptz) string {
	return t.Time.Format("2006-01-02T15:04:05Z07:00")
}

func toPbUser(u db.User) *auth.User {
	bio := ""
	if u.Bio != nil {
		bio = *u.Bio
	}
	location := ""
	if u.Location != nil {
		location = *u.Location
	}
	website := ""
	if u.Website != nil {
		website = *u.Website
	}
	avatarUrl := ""
	if u.AvatarUrl != nil {
		avatarUrl = *u.AvatarUrl
	}

	return &auth.User{
		Id:       u.ID.String(),
		Username: u.Username,
		Email:    u.Email,
		FullName: u.FullName,
		Bio:      bio,
		Location: location,
		Website:  website,
		// Normalize nil → [] so JSON serialization produces [] instead of null
		// (go-zero's `optional` tag is not omitempty).
		Interests:     nonNilStrings(u.Interests),
		AvatarUrl:     avatarUrl,
		CreatedAt:     formatTime(u.CreatedAt),
		UpdatedAt:     formatTime(u.UpdatedAt),
		EmailVerified: u.EmailVerified,
	}
}

// nonNilStrings returns the slice if non-nil, otherwise an empty slice so JSON
// serialization produces [] instead of null.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// runInTx runs fn inside a transaction with the repository bound to that
// transaction. When override is set (tests only), it replaces the runner and
// the repository resolves to svcCtx.Repo so calls land on mocks — mirroring
// the client service's testTxRunner/getTxRepo seam.
func runInTx(svcCtx *svc.ServiceContext, override svc.TxRunnerInterface, ctx context.Context, userID string, fn func(*repository.Repository) error) error {
	r := svcCtx.TxRunner
	if override != nil {
		r = override
	}
	return r.Run(ctx, userID, func(tx pgx.Tx) error {
		repo := svcCtx.WithTx(tx)
		if override != nil {
			repo = svcCtx.Repo
		}
		return fn(repo)
	})
}

// userProfileUpdatedEnvelope builds the user_profile_updated event for the
// given user row.
func userProfileUpdatedEnvelope(u db.User) (events.Envelope, error) {
	bio := ""
	if u.Bio != nil {
		bio = *u.Bio
	}
	location := ""
	if u.Location != nil {
		location = *u.Location
	}
	website := ""
	if u.Website != nil {
		website = *u.Website
	}
	avatar := ""
	if u.AvatarUrl != nil {
		avatar = *u.AvatarUrl
	}
	return events.NewEnvelope(events.TypeUserProfileUpdated, events.UserProfileUpdated{
		UserID:        u.ID.String(),
		Username:      u.Username,
		Email:         u.Email,
		EmailVerified: u.EmailVerified,
		Name:          u.FullName,
		Bio:           bio,
		Location:      location,
		Website:       website,
		Interests:     u.Interests,
		Avatar:        avatar,
	})
}

// enqueueUserProfileUpdated writes a user_profile_updated row into
// auth_event_outbox via the outbox repo — which may be transaction-scoped so
// the event commits atomically with the user mutation (P1). The svc-layer
// relay drains the outbox to the broker under a stable event ID.
func enqueueUserProfileUpdated(ctx context.Context, outboxRepo repository.IEventOutbox, u db.User) error {
	env, err := userProfileUpdatedEnvelope(u)
	if err != nil {
		return err
	}
	return outboxRepo.Enqueue(ctx, env)
}

// publishUserProfileUpdated enqueues a user_profile_updated event into
// auth_event_outbox for mutations that do not run inside a transaction
// (e.g. login, which only reads the user). The outbox write is a single local
// INSERT — far narrower than the old publish-after-commit network call — and
// the relay republishes under a stable event ID until it succeeds (P1).
// It logs loudly on failure but does not return an error: failing the RPC
// after the write committed would mislead the caller.
func publishUserProfileUpdated(ctx context.Context, pool *pgxpool.Pool, u db.User) {
	if pool == nil {
		return
	}
	if err := enqueueUserProfileUpdated(ctx, repository.NewEventOutboxRepo(db.New(pool)), u); err != nil {
		logx.WithContext(ctx).Errorf("failed to enqueue user_profile_updated event for user %s: %v", u.ID, err)
	}
}
