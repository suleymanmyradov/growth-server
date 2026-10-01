package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/config"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/mfa"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/middleware"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
)

// --- fakes -----------------------------------------------------------------

type fakeUsers struct {
	byEmail map[string]*db.InternalUser
	byID    map[uuid.UUID]*db.InternalUser
}

func newFakeUsers() *fakeUsers {
	return &fakeUsers{byEmail: map[string]*db.InternalUser{}, byID: map[uuid.UUID]*db.InternalUser{}}
}

func (f *fakeUsers) add(u db.InternalUser) {
	f.byEmail[u.Email] = &u
	f.byID[u.ID] = &u
}

func (f *fakeUsers) Create(_ context.Context, email, passwordHash, fullName, role string) (db.InternalUser, error) {
	u := db.InternalUser{ID: uuid.New(), Email: email, PasswordHash: passwordHash, FullName: fullName, Role: role}
	f.add(u)
	return u, nil
}

func (f *fakeUsers) GetByEmail(_ context.Context, email string) (db.InternalUser, error) {
	if u, ok := f.byEmail[email]; ok {
		return *u, nil
	}
	return db.InternalUser{}, pgx.ErrNoRows
}

func (f *fakeUsers) GetByID(_ context.Context, id uuid.UUID) (db.InternalUser, error) {
	if u, ok := f.byID[id]; ok {
		return *u, nil
	}
	return db.InternalUser{}, pgx.ErrNoRows
}

func (f *fakeUsers) UpdatePassword(_ context.Context, id uuid.UUID, passwordHash string) (db.InternalUser, error) {
	u, ok := f.byID[id]
	if !ok {
		return db.InternalUser{}, pgx.ErrNoRows
	}
	u.PasswordHash = passwordHash
	return *u, nil
}

func (f *fakeUsers) UpdateProfile(_ context.Context, id uuid.UUID, fullName string) (db.InternalUser, error) {
	u, ok := f.byID[id]
	if !ok {
		return db.InternalUser{}, pgx.ErrNoRows
	}
	u.FullName = fullName
	return *u, nil
}

type fakeMfa struct {
	tickets   map[string]*db.AdminMfaTicket // tokenHash -> ticket
	codesUsed map[string]bool               // userID:hash -> used
	pending   map[uuid.UUID]string          // userID -> plaintext code hash
	users     *fakeUsers
}

func newFakeMfa(users *fakeUsers) *fakeMfa {
	return &fakeMfa{tickets: map[string]*db.AdminMfaTicket{}, codesUsed: map[string]bool{}, pending: map[uuid.UUID]string{}, users: users}
}

func (f *fakeMfa) SetTotpSecret(_ context.Context, userID uuid.UUID, enc string) error {
	u, ok := f.users.byID[userID]
	if !ok {
		return pgx.ErrNoRows
	}
	u.TotpSecretEncrypted = &enc
	return nil
}

func (f *fakeMfa) EnableTotp(_ context.Context, userID uuid.UUID) error {
	u, ok := f.users.byID[userID]
	if !ok {
		return pgx.ErrNoRows
	}
	u.TotpEnabledAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	return nil
}

func (f *fakeMfa) DisableTotp(_ context.Context, userID uuid.UUID) error {
	u, ok := f.users.byID[userID]
	if !ok {
		return pgx.ErrNoRows
	}
	u.TotpSecretEncrypted = nil
	u.TotpEnabledAt = pgtype.Timestamptz{}
	return nil
}

func (f *fakeMfa) CreateTicket(_ context.Context, userID uuid.UUID, tokenHash, purpose string, expiresAt time.Time) (db.AdminMfaTicket, error) {
	t := &db.AdminMfaTicket{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: tokenHash,
		Purpose:   purpose,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}
	f.tickets[tokenHash] = t
	return *t, nil
}

func (f *fakeMfa) GetTicketByHash(_ context.Context, tokenHash string) (db.AdminMfaTicket, error) {
	if t, ok := f.tickets[tokenHash]; ok {
		return *t, nil
	}
	return db.AdminMfaTicket{}, pgx.ErrNoRows
}

func (f *fakeMfa) IncrementTicketAttempts(_ context.Context, ticketID uuid.UUID) error {
	for _, t := range f.tickets {
		if t.ID == ticketID {
			t.Attempts++
			return nil
		}
	}
	return pgx.ErrNoRows
}

func (f *fakeMfa) DeleteTicket(_ context.Context, ticketID uuid.UUID) error {
	for h, t := range f.tickets {
		if t.ID == ticketID {
			delete(f.tickets, h)
			return nil
		}
	}
	return pgx.ErrNoRows
}

func (f *fakeMfa) DeleteTicketsForUser(_ context.Context, userID uuid.UUID) error {
	for h, t := range f.tickets {
		if t.UserID == userID {
			delete(f.tickets, h)
		}
	}
	return nil
}

func (f *fakeMfa) InsertBackupCode(_ context.Context, userID uuid.UUID, codeHash string) error {
	f.pending[userID] = codeHash
	f.codesUsed[userID.String()+":"+codeHash] = false
	return nil
}

func (f *fakeMfa) ConsumeBackupCode(_ context.Context, userID uuid.UUID, codeHash string) (bool, error) {
	k := userID.String() + ":" + codeHash
	used, ok := f.codesUsed[k]
	if !ok || used {
		return false, nil
	}
	f.codesUsed[k] = true
	return true, nil
}

func (f *fakeMfa) DeleteBackupCodesForUser(_ context.Context, userID uuid.UUID) error {
	for k := range f.codesUsed {
		if len(k) > 36 && k[:36] == userID.String() {
			delete(f.codesUsed, k)
		}
	}
	delete(f.pending, userID)
	return nil
}

// --- harness ---------------------------------------------------------------

const testMfaKeyB64 = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=" // 32 bytes, tests only

type testEnv struct {
	svcCtx *svc.ServiceContext
	users  *fakeUsers
	mfa    *fakeMfa
}

func newTestEnv(t *testing.T, required bool) *testEnv {
	t.Helper()

	priv, pub, err := jwt.GenerateKeyPair()
	require.NoError(t, err)
	maker, err := jwt.NewTokenMaker(jwt.Config{
		PrivateKey:            priv,
		PublicKey:             pub,
		Issuer:                "growth-admin",
		Audience:              "growth-admin",
		AccessExpiryDuration:  15 * time.Minute,
		RefreshExpiryDuration: 24 * time.Hour,
	}, nil)
	require.NoError(t, err)

	users := newFakeUsers()
	m := newFakeMfa(users)

	c := config.Config{}
	c.Mfa.Required = required
	c.Mfa.Issuer = "Growth Admin"
	c.Mfa.TicketTTL = 5 * time.Minute
	c.Mfa.EncryptionKey = testMfaKeyB64

	mfaKey, err := mfa.ParseKey(testMfaKeyB64)
	require.NoError(t, err)

	return &testEnv{
		svcCtx: &svc.ServiceContext{
			Config:     c,
			TokenMaker: maker,
			MfaKey:     mfaKey,
			Repo: &repository.Repository{
				InternalUsers: users,
				Mfa:           m,
			},
		},
		users: users,
		mfa:   m,
	}
}

// newAdmin inserts a user with a bcrypt-hashed password.
func (e *testEnv) newAdmin(t *testing.T, password string) db.InternalUser {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	u := db.InternalUser{
		ID:           uuid.New(),
		Email:        "admin@example.com",
		PasswordHash: string(hash),
		FullName:     "Admin",
		Role:         "admin",
	}
	e.users.add(u)
	return u
}

// enroll puts a TOTP secret on the stored user in the enabled state,
// returning the plaintext base32 secret.
func (e *testEnv) enroll(t *testing.T, u db.InternalUser) string {
	t.Helper()
	key, err := mfa.NewTOTPKey("Growth Admin", u.Email)
	require.NoError(t, err)
	enc, err := mfa.EncryptSecret(e.svcCtx.MfaKey, key.Secret())
	require.NoError(t, err)
	stored := e.users.byID[u.ID]
	stored.TotpSecretEncrypted = &enc
	stored.TotpEnabledAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	return key.Secret()
}

// --- tests -----------------------------------------------------------------

func TestAuthLogin_NoMfa_ReturnsTokens(t *testing.T) {
	e := newTestEnv(t, false)
	e.newAdmin(t, "pw")

	resp, err := NewAuthLoginLogic(context.Background(), e.svcCtx).
		AuthLogin(&types.LoginRequest{Email: "admin@example.com", Password: "pw"})
	require.NoError(t, err)
	require.False(t, resp.MfaRequired)
	require.NotNil(t, resp.Auth)
	require.NotEmpty(t, resp.Auth.AccessToken)
}

func TestAuthLogin_Enrolled_ReturnsVerifyTicket(t *testing.T) {
	e := newTestEnv(t, false)
	u := e.newAdmin(t, "pw")
	e.enroll(t, u)

	resp, err := NewAuthLoginLogic(context.Background(), e.svcCtx).
		AuthLogin(&types.LoginRequest{Email: "admin@example.com", Password: "pw"})
	require.NoError(t, err)
	require.True(t, resp.MfaRequired)
	require.False(t, resp.MfaSetupRequired)
	require.NotEmpty(t, resp.MfaTicket)
	require.Nil(t, resp.Auth)
}

func TestAuthLogin_RequiredAndUnenrolled_ReturnsEnrollTicket(t *testing.T) {
	e := newTestEnv(t, true)
	e.newAdmin(t, "pw")

	resp, err := NewAuthLoginLogic(context.Background(), e.svcCtx).
		AuthLogin(&types.LoginRequest{Email: "admin@example.com", Password: "pw"})
	require.NoError(t, err)
	require.True(t, resp.MfaRequired)
	require.True(t, resp.MfaSetupRequired)
	require.NotEmpty(t, resp.MfaTicket)
	require.Nil(t, resp.Auth)

	ticket, err := e.mfa.GetTicketByHash(context.Background(), mfa.HashTicket(resp.MfaTicket))
	require.NoError(t, err)
	assert.Equal(t, mfa.TicketPurposeEnroll, ticket.Purpose)
}

func TestAuthLogin_BadPassword_StillInvalidCredentials(t *testing.T) {
	e := newTestEnv(t, true)
	u := e.newAdmin(t, "pw")
	e.enroll(t, u)

	_, err := NewAuthLoginLogic(context.Background(), e.svcCtx).
		AuthLogin(&types.LoginRequest{Email: "admin@example.com", Password: "wrong"})
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestAuthMfaVerify_GoodCode_ReturnsTokens(t *testing.T) {
	e := newTestEnv(t, false)
	u := e.newAdmin(t, "pw")
	secret := e.enroll(t, u)

	login, err := NewAuthLoginLogic(context.Background(), e.svcCtx).
		AuthLogin(&types.LoginRequest{Email: u.Email, Password: "pw"})
	require.NoError(t, err)

	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)

	resp, err := NewAuthMfaVerifyLogic(context.Background(), e.svcCtx).
		AuthMfaVerify(&types.MfaVerifyRequest{Ticket: login.MfaTicket, Code: code})
	require.NoError(t, err)
	require.NotEmpty(t, resp.AccessToken)
	require.NotEmpty(t, resp.RefreshToken)

	// Ticket is single-use: replaying it must fail.
	_, err = NewAuthMfaVerifyLogic(context.Background(), e.svcCtx).
		AuthMfaVerify(&types.MfaVerifyRequest{Ticket: login.MfaTicket, Code: code})
	require.ErrorIs(t, err, ErrInvalidMfaTicket)
}

func TestAuthMfaVerify_WrongCode_CountsAttempts(t *testing.T) {
	e := newTestEnv(t, false)
	u := e.newAdmin(t, "pw")
	e.enroll(t, u)

	login, err := NewAuthLoginLogic(context.Background(), e.svcCtx).
		AuthLogin(&types.LoginRequest{Email: u.Email, Password: "pw"})
	require.NoError(t, err)

	_, err = NewAuthMfaVerifyLogic(context.Background(), e.svcCtx).
		AuthMfaVerify(&types.MfaVerifyRequest{Ticket: login.MfaTicket, Code: "000000"})
	require.ErrorIs(t, err, ErrInvalidMfaCode)

	ticket, err := e.mfa.GetTicketByHash(context.Background(), mfa.HashTicket(login.MfaTicket))
	require.NoError(t, err)
	assert.EqualValues(t, 1, ticket.Attempts)
}

func TestAuthMfaVerify_ExhaustedAttempts_KillsTicket(t *testing.T) {
	e := newTestEnv(t, false)
	u := e.newAdmin(t, "pw")
	secret := e.enroll(t, u)

	login, err := NewAuthLoginLogic(context.Background(), e.svcCtx).
		AuthLogin(&types.LoginRequest{Email: u.Email, Password: "pw"})
	require.NoError(t, err)

	for i := 0; i < maxMfaAttempts; i++ {
		_, err = NewAuthMfaVerifyLogic(context.Background(), e.svcCtx).
			AuthMfaVerify(&types.MfaVerifyRequest{Ticket: login.MfaTicket, Code: "000000"})
		require.Error(t, err)
	}

	// Even a correct code must now fail — the ticket is dead.
	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)
	_, err = NewAuthMfaVerifyLogic(context.Background(), e.svcCtx).
		AuthMfaVerify(&types.MfaVerifyRequest{Ticket: login.MfaTicket, Code: code})
	require.ErrorIs(t, err, ErrInvalidMfaTicket)
}

func TestAuthMfaVerify_BackupCode_WorksOnce(t *testing.T) {
	e := newTestEnv(t, false)
	u := e.newAdmin(t, "pw")
	e.enroll(t, u)

	codes, hashes, err := mfa.NewBackupCodes()
	require.NoError(t, err)
	for _, h := range hashes {
		require.NoError(t, e.mfa.InsertBackupCode(context.Background(), u.ID, h))
	}

	login, err := NewAuthLoginLogic(context.Background(), e.svcCtx).
		AuthLogin(&types.LoginRequest{Email: u.Email, Password: "pw"})
	require.NoError(t, err)

	resp, err := NewAuthMfaVerifyLogic(context.Background(), e.svcCtx).
		AuthMfaVerify(&types.MfaVerifyRequest{Ticket: login.MfaTicket, Code: codes[0]})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Same backup code cannot be reused on a fresh login.
	login2, err := NewAuthLoginLogic(context.Background(), e.svcCtx).
		AuthLogin(&types.LoginRequest{Email: u.Email, Password: "pw"})
	require.NoError(t, err)
	_, err = NewAuthMfaVerifyLogic(context.Background(), e.svcCtx).
		AuthMfaVerify(&types.MfaVerifyRequest{Ticket: login2.MfaTicket, Code: codes[0]})
	require.ErrorIs(t, err, ErrInvalidMfaCode)
}

func TestAuthMfaVerify_EnrollTicketRejected(t *testing.T) {
	e := newTestEnv(t, true)
	e.newAdmin(t, "pw")

	login, err := NewAuthLoginLogic(context.Background(), e.svcCtx).
		AuthLogin(&types.LoginRequest{Email: "admin@example.com", Password: "pw"})
	require.NoError(t, err)
	require.True(t, login.MfaSetupRequired)

	_, err = NewAuthMfaVerifyLogic(context.Background(), e.svcCtx).
		AuthMfaVerify(&types.MfaVerifyRequest{Ticket: login.MfaTicket, Code: "123456"})
	require.ErrorIs(t, err, ErrInvalidMfaTicket)
}

func TestAuthMfaConfirm_EnrollTicket_CompletesLogin(t *testing.T) {
	e := newTestEnv(t, true)
	u := e.newAdmin(t, "pw")

	login, err := NewAuthLoginLogic(context.Background(), e.svcCtx).
		AuthLogin(&types.LoginRequest{Email: u.Email, Password: "pw"})
	require.NoError(t, err)
	require.True(t, login.MfaSetupRequired)

	// Simulate the MfaAuth middleware: enroll ticket -> principal + ctx ticket.
	ticket, err := e.mfa.GetTicketByHash(context.Background(), mfa.HashTicket(login.MfaTicket))
	require.NoError(t, err)
	ctx := middleware.WithMfaTicket(
		principal.WithPrincipal(context.Background(), principal.Principal{UserID: u.ID.String()}),
		ticket,
	)

	setup, err := NewAuthMfaSetupLogic(ctx, e.svcCtx).AuthMfaSetup()
	require.NoError(t, err)
	require.NotEmpty(t, setup.Secret)
	require.True(t, strings.HasPrefix(setup.QrCodeDataUrl, "data:image/png;base64,"))

	code, err := totp.GenerateCode(setup.Secret, time.Now())
	require.NoError(t, err)
	conf, err := NewAuthMfaConfirmLogic(ctx, e.svcCtx).
		AuthMfaConfirm(&types.MfaConfirmRequest{Code: code})
	require.NoError(t, err)
	require.Len(t, conf.BackupCodes, mfa.BackupCodeCount)
	require.NotNil(t, conf.Auth, "enroll-ticket confirm should complete the login")
	require.NotEmpty(t, conf.Auth.AccessToken)

	// The enroll ticket is consumed.
	_, err = e.mfa.GetTicketByHash(context.Background(), mfa.HashTicket(login.MfaTicket))
	require.Error(t, err)

	// And the user now requires MFA at login.
	login2, err := NewAuthLoginLogic(context.Background(), e.svcCtx).
		AuthLogin(&types.LoginRequest{Email: u.Email, Password: "pw"})
	require.NoError(t, err)
	require.True(t, login2.MfaRequired)
	require.False(t, login2.MfaSetupRequired)
}

func TestAuthMfaConfirm_BadCode_DoesNotEnable(t *testing.T) {
	e := newTestEnv(t, true)
	u := e.newAdmin(t, "pw")

	login, err := NewAuthLoginLogic(context.Background(), e.svcCtx).
		AuthLogin(&types.LoginRequest{Email: u.Email, Password: "pw"})
	require.NoError(t, err)

	ticket, err := e.mfa.GetTicketByHash(context.Background(), mfa.HashTicket(login.MfaTicket))
	require.NoError(t, err)
	ctx := middleware.WithMfaTicket(
		principal.WithPrincipal(context.Background(), principal.Principal{UserID: u.ID.String()}),
		ticket,
	)

	_, err = NewAuthMfaSetupLogic(ctx, e.svcCtx).AuthMfaSetup()
	require.NoError(t, err)

	_, err = NewAuthMfaConfirmLogic(ctx, e.svcCtx).
		AuthMfaConfirm(&types.MfaConfirmRequest{Code: "000000"})
	require.ErrorIs(t, err, ErrInvalidMfaCode)

	u2, _ := e.users.GetByID(context.Background(), u.ID)
	assert.False(t, u2.TotpEnabledAt.Valid, "bad confirm must not enable MFA")
}

func TestAuthMfaDisable_RequiresPasswordAndCode(t *testing.T) {
	e := newTestEnv(t, false)
	u := e.newAdmin(t, "pw")
	secret := e.enroll(t, u)

	ctx := principal.WithPrincipal(context.Background(), principal.Principal{UserID: u.ID.String()})
	logic := NewAuthMfaDisableLogic(ctx, e.svcCtx)

	_, err := logic.AuthMfaDisable(&types.MfaDisableRequest{Password: "wrong", Code: "123456"})
	require.ErrorIs(t, err, ErrInvalidCredentials)

	_, err = logic.AuthMfaDisable(&types.MfaDisableRequest{Password: "pw", Code: "000000"})
	require.ErrorIs(t, err, ErrInvalidMfaCode)

	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)
	_, err = logic.AuthMfaDisable(&types.MfaDisableRequest{Password: "pw", Code: code})
	require.NoError(t, err)

	u2, _ := e.users.GetByID(context.Background(), u.ID)
	assert.False(t, u2.TotpEnabledAt.Valid)
	assert.Nil(t, u2.TotpSecretEncrypted)

	// Login falls back to plain password auth while Required stays off.
	login, err := NewAuthLoginLogic(context.Background(), e.svcCtx).
		AuthLogin(&types.LoginRequest{Email: u.Email, Password: "pw"})
	require.NoError(t, err)
	require.False(t, login.MfaRequired)
}
