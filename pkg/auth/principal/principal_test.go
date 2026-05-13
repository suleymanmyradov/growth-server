package principal

import (
	"context"
	"testing"
)

func TestWithPrincipal(t *testing.T) {
	ctx := context.Background()
	p := Principal{
		UserID:    "user123",
		Username:  "testuser",
		Roles:     []string{"admin", "user"},
		SessionID: "session456",
	}

	ctx = WithPrincipal(ctx, p)

	got, ok := PrincipalFrom(ctx)
	if !ok {
		t.Fatal("PrincipalFrom returned false")
	}

	if got.UserID != p.UserID {
		t.Errorf("UserID = %q, want %q", got.UserID, p.UserID)
	}
	if got.Username != p.Username {
		t.Errorf("Username = %q, want %q", got.Username, p.Username)
	}
	if len(got.Roles) != len(p.Roles) {
		t.Errorf("Roles length = %d, want %d", len(got.Roles), len(p.Roles))
	}
	for i, role := range got.Roles {
		if role != p.Roles[i] {
			t.Errorf("Roles[%d] = %q, want %q", i, role, p.Roles[i])
		}
	}
	if got.SessionID != p.SessionID {
		t.Errorf("SessionID = %q, want %q", got.SessionID, p.SessionID)
	}
}

func TestPrincipalFrom(t *testing.T) {
	ctx := context.Background()

	_, ok := PrincipalFrom(ctx)
	if ok {
		t.Fatal("PrincipalFrom returned true for empty context")
	}

	p := Principal{UserID: "user123"}
	ctx = WithPrincipal(ctx, p)

	got, ok := PrincipalFrom(ctx)
	if !ok {
		t.Fatal("PrincipalFrom returned false")
	}
	if got.UserID != p.UserID {
		t.Errorf("UserID = %q, want %q", got.UserID, p.UserID)
	}
}

func TestWithToken(t *testing.T) {
	ctx := context.Background()
	token := "jwt-token-123"

	ctx = WithToken(ctx, token)

	got, ok := TokenFrom(ctx)
	if !ok {
		t.Fatal("TokenFrom returned false")
	}
	if got != token {
		t.Errorf("Token = %q, want %q", got, token)
	}
}

func TestTokenFrom(t *testing.T) {
	ctx := context.Background()

	_, ok := TokenFrom(ctx)
	if ok {
		t.Fatal("TokenFrom returned true for empty context")
	}

	token := "jwt-token-456"
	ctx = WithToken(ctx, token)

	got, ok := TokenFrom(ctx)
	if !ok {
		t.Fatal("TokenFrom returned false")
	}
	if got != token {
		t.Errorf("Token = %q, want %q", got, token)
	}
}
