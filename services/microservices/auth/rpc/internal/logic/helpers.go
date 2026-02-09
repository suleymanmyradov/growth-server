package logic

import (
	"database/sql"
	"strings"
	"time"

	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
)

func toNullString(s string) sql.NullString {
	if strings.TrimSpace(s) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func formatNullTime(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format(time.RFC3339)
}

func toPbUser(u db.User, p db.Profile) *auth.User {
	return &auth.User{
		Id:        u.ID.String(),
		Username:  u.Username,
		Email:     u.Email,
		FullName:  u.FullName,
		Bio:       p.Bio.String,
		Location:  p.Location.String,
		Website:   p.Website.String,
		Interests: p.Interests,
		AvatarUrl: p.AvatarUrl.String,
		CreatedAt: formatNullTime(u.CreatedAt),
		UpdatedAt: formatNullTime(u.UpdatedAt),
	}
}
