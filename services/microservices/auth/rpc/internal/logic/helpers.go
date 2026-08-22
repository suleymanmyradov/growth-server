package logic

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
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

// publishUserProfileUpdated publishes a user_profile_updated event synchronously.
// It logs loudly on failure but does not return an error — the DB write has
// already committed and failing the RPC would mislead the caller. In dev this
// makes broker issues immediately visible in logs rather than silently diverging.
func publishUserProfileUpdated(ctx context.Context, pub *events.Publisher, u db.User) {
	if pub == nil {
		return
	}

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

	env, err := events.NewEnvelope(events.TypeUserProfileUpdated, events.UserProfileUpdated{
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
	if err != nil {
		logx.WithContext(ctx).Errorf("failed to build user_profile_updated envelope: %v", err)
		return
	}
	if err := pub.Publish(ctx, env); err != nil {
		logx.WithContext(ctx).Errorf("failed to publish user_profile_updated event for user %s: %v", u.ID, err)
	}
}
