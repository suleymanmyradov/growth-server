package mdpropagate

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
)

func TestOutgoing(t *testing.T) {
	tests := []struct {
		name      string
		principal principal.Principal
		wantKeys  []string
	}{
		{
			name: "full principal",
			principal: principal.Principal{
				UserID:    "user123",
				Username:  "testuser",
				Roles:     []string{"admin", "user"},
				SessionID: "session456",
			},
			wantKeys: []string{MDUserID, MDUsername, MDRoles, MDSessionID},
		},
		{
			name: "partial principal",
			principal: principal.Principal{
				UserID: "user123",
				Roles:  []string{"admin"},
			},
			wantKeys: []string{MDUserID, MDRoles},
		},
		{
			name:      "empty principal",
			principal: principal.Principal{},
			wantKeys:  nil, // No metadata should be added
		},
		{
			name: "principal with empty roles",
			principal: principal.Principal{
				UserID: "user123",
				Roles:  []string{"", "admin", ""},
			},
			wantKeys: []string{MDUserID, MDRoles},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = principal.WithPrincipal(ctx, tt.principal)

			ctx = Outgoing(ctx)

			md, ok := metadata.FromOutgoingContext(ctx)
			if tt.wantKeys == nil {
				// No metadata expected
				if ok && len(md) > 0 {
					t.Errorf("expected no metadata, got %v", md)
				}
				return
			}

			if !ok {
				t.Fatal("no metadata in context")
			}

			for _, key := range tt.wantKeys {
				if _, exists := md[key]; !exists {
					t.Errorf("metadata key %q not found", key)
				}
			}

			// Ensure no extra keys were added
			for key := range md {
				found := false
				for _, wantKey := range tt.wantKeys {
					if key == wantKey {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("unexpected metadata key %q", key)
				}
			}
		})
	}
}

func TestPrincipalFromMetadata(t *testing.T) {
	tests := []struct {
		name       string
		metadata   metadata.MD
		wantErr    bool
		wantUserID string
	}{
		{
			name: "full metadata",
			metadata: metadata.Pairs(
				MDUserID, "user123",
				MDUsername, "testuser",
				MDRoles, "admin,user",
				MDSessionID, "session456",
			),
			wantErr:    false,
			wantUserID: "user123",
		},
		{
			name: "partial metadata",
			metadata: metadata.Pairs(
				MDUserID, "user123",
				MDUsername, "testuser",
			),
			wantErr:    false,
			wantUserID: "user123",
		},
		{
			name:       "missing metadata",
			metadata:   metadata.Pairs(),
			wantErr:    true,
			wantUserID: "",
		},
		{
			name:       "missing user id",
			metadata:   metadata.Pairs(MDUsername, "testuser"),
			wantErr:    true,
			wantUserID: "",
		},
		{
			name: "roles with empty strings",
			metadata: metadata.Pairs(
				MDUserID, "user123",
				MDRoles, ",admin,,user,",
			),
			wantErr:    false,
			wantUserID: "user123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := metadata.NewIncomingContext(context.Background(), tt.metadata)

			ctx, err := PrincipalFromMetadata(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("PrincipalFromMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				p, ok := principal.PrincipalFrom(ctx)
				if !ok {
					t.Fatal("principal not found in context")
				}
				if p.UserID != tt.wantUserID {
					t.Errorf("UserID = %q, want %q", p.UserID, tt.wantUserID)
				}
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	original := principal.Principal{
		UserID:    "user123",
		Username:  "testuser",
		Roles:     []string{"admin", "user"},
		SessionID: "session456",
	}

	// Set principal in context
	ctx := context.Background()
	ctx = principal.WithPrincipal(ctx, original)

	// Convert to outgoing metadata
	ctx = Outgoing(ctx)

	// Extract from incoming metadata (simulating gRPC call)
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("no metadata in context")
	}

	ctx = metadata.NewIncomingContext(context.Background(), md)

	// Extract principal from metadata
	ctx, err := PrincipalFromMetadata(ctx)
	if err != nil {
		t.Fatalf("PrincipalFromMetadata() error = %v", err)
	}

	recovered, ok := principal.PrincipalFrom(ctx)
	if !ok {
		t.Fatal("principal not found in context")
	}

	if recovered.UserID != original.UserID {
		t.Errorf("UserID = %q, want %q", recovered.UserID, original.UserID)
	}
	if recovered.Username != original.Username {
		t.Errorf("Username = %q, want %q", recovered.Username, original.Username)
	}
	if len(recovered.Roles) != len(original.Roles) {
		t.Errorf("Roles length = %d, want %d", len(recovered.Roles), len(original.Roles))
	}
	for i, role := range recovered.Roles {
		if role != original.Roles[i] {
			t.Errorf("Roles[%d] = %q, want %q", i, role, original.Roles[i])
		}
	}
	if recovered.SessionID != original.SessionID {
		t.Errorf("SessionID = %q, want %q", recovered.SessionID, original.SessionID)
	}
}

func TestUnaryClientInterceptor(t *testing.T) {
	tests := []struct {
		name      string
		principal principal.Principal
		wantMD    bool
	}{
		{
			name:      "with principal",
			principal: principal.Principal{UserID: "user123", Username: "testuser"},
			wantMD:    true,
		},
		{
			name:      "without principal",
			principal: principal.Principal{},
			wantMD:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.principal.UserID != "" {
				ctx = principal.WithPrincipal(ctx, tt.principal)
			}

			// Create a mock invoker that captures the context
			var capturedCtx context.Context
			invoker := func(ctx context.Context, _ string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				capturedCtx = ctx
				return nil
			}

			interceptor := UnaryClientInterceptor()
			err := interceptor(ctx, "/test/method", nil, nil, nil, invoker)
			if err != nil {
				t.Fatalf("UnaryClientInterceptor() error = %v", err)
			}

			md, ok := metadata.FromOutgoingContext(capturedCtx)
			if tt.wantMD && !ok {
				t.Error("expected metadata in context, got none")
			}
			if !tt.wantMD && ok && len(md) > 0 {
				t.Errorf("expected no metadata, got %v", md)
			}
		})
	}
}

func TestUnaryServerInterceptorOptional(t *testing.T) {
	tests := []struct {
		name      string
		metadata  metadata.MD
		wantError bool
	}{
		{
			name:      "with principal metadata",
			metadata:  metadata.Pairs(MDUserID, "user123", MDUsername, "testuser"),
			wantError: false,
		},
		{
			name:      "without principal metadata",
			metadata:  metadata.Pairs(),
			wantError: false,
		},
		{
			name:      "with partial metadata",
			metadata:  metadata.Pairs(MDUsername, "testuser"),
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := metadata.NewIncomingContext(context.Background(), tt.metadata)

			interceptor := UnaryServerInterceptorOptional()
			handlerCalled := false

			handler := func(_ context.Context, req interface{}) (interface{}, error) {
				handlerCalled = true
				return nil, nil
			}

			_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
			if (err != nil) != tt.wantError {
				t.Errorf("UnaryServerInterceptorOptional() error = %v, wantError %v", err, tt.wantError)
			}
			if !handlerCalled {
				t.Error("handler was not called")
			}
		})
	}
}
