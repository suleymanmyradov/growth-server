package mdpropagate

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
)

// userIDGetter matches the accessor generated on proto request messages that
// carry a user_id field. Requests without one pass the guard unchanged.
type userIDGetter interface {
	GetUserId() string
}

// UnaryUserIDGuardInterceptor rejects requests whose user_id field does not
// match the verified principal's user ID, so a service can't be tricked into
// acting on a different user's data by a caller that passes s2s auth but
// supplies an arbitrary user ID. It must be chained AFTER
// UnaryServerInterceptor (or any interceptor that injects the principal).
// Requests with no principal in context, no user_id accessor, or an empty
// user_id are passed through — the logic layer validates required fields.
func UnaryUserIDGuardInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if skipMethod(info.FullMethod) {
			return handler(ctx, req)
		}
		if err := checkUserIDBinding(ctx, req); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamUserIDGuardInterceptor is the streaming counterpart of
// UnaryUserIDGuardInterceptor: it inspects the first received message and
// rejects the stream when its user_id does not match the verified principal.
func StreamUserIDGuardInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if skipMethod(info.FullMethod) {
			return handler(srv, ss)
		}
		return handler(srv, &userGuardServerStream{ServerStream: ss})
	}
}

func checkUserIDBinding(ctx context.Context, req interface{}) error {
	p, ok := principal.PrincipalFrom(ctx)
	if !ok || p.UserID == "" {
		return nil
	}
	ur, ok := req.(userIDGetter)
	if !ok {
		return nil
	}
	if uid := strings.TrimSpace(ur.GetUserId()); uid != "" && uid != p.UserID {
		return status.Error(codes.PermissionDenied, "user_id does not match authenticated principal")
	}
	return nil
}

// userGuardServerStream runs the user-ID binding check on the first message
// received by the server (the request, for server-streaming calls).
type userGuardServerStream struct {
	grpc.ServerStream
	checked bool
}

func (s *userGuardServerStream) RecvMsg(m interface{}) error {
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return err
	}
	if s.checked {
		return nil
	}
	s.checked = true
	return checkUserIDBinding(s.Context(), m)
}
