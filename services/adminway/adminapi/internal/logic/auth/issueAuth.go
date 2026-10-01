package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/mfa"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
)

// issueAuthTokens creates a fresh session and returns the token pair that the
// login, mfa-verify, and enroll-confirm flows all end with.
func issueAuthTokens(ctx context.Context, svcCtx *svc.ServiceContext, user db.InternalUser) (*types.AuthResponse, error) {
	sessionID := uuid.New()

	accessToken, err := svcCtx.TokenMaker.CreateAccessToken(ctx, user.ID, user.Email, []string{user.Role}, sessionID)
	if err != nil {
		return nil, fmt.Errorf("create access token: %w", err)
	}
	refreshToken, err := svcCtx.TokenMaker.CreateRefreshToken(ctx, user.ID, user.Email, []string{user.Role}, sessionID)
	if err != nil {
		return nil, fmt.Errorf("create refresh token: %w", err)
	}

	return &types.AuthResponse{
		AccessToken:  accessToken.Token,
		RefreshToken: refreshToken.Token,
		User: types.UserInfo{
			Id:       user.ID.String(),
			Email:    user.Email,
			FullName: user.FullName,
			Role:     user.Role,
		},
	}, nil
}

// issueMfaTicket clears the user's stale tickets and mints a fresh one,
// returning the client-facing token (the DB row keeps only its hash).
func issueMfaTicket(ctx context.Context, svcCtx *svc.ServiceContext, userID uuid.UUID, purpose string) (string, error) {
	// Best-effort cleanup: prior half-finished logins leave dead tickets.
	_ = svcCtx.Repo.Mfa.DeleteTicketsForUser(ctx, userID)

	token, hash, err := mfa.NewTicket()
	if err != nil {
		return "", fmt.Errorf("generate mfa ticket: %w", err)
	}
	expiresAt := time.Now().Add(svcCtx.Config.Mfa.TicketTTL)
	if _, err := svcCtx.Repo.Mfa.CreateTicket(ctx, userID, hash, purpose, expiresAt); err != nil {
		return "", fmt.Errorf("store mfa ticket: %w", err)
	}
	return token, nil
}

// callerUser resolves the admin from the principal set by the MfaAuth
// middleware — a JWT principal or an enroll-ticket principal both land here.
func callerUser(ctx context.Context, svcCtx *svc.ServiceContext) (db.InternalUser, error) {
	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return db.InternalUser{}, ErrInvalidMfaTicket
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return db.InternalUser{}, ErrInvalidMfaTicket
	}
	user, err := svcCtx.Repo.InternalUsers.GetByID(ctx, userID)
	if err != nil {
		return db.InternalUser{}, ErrAdminNotFound
	}
	return user, nil
}
