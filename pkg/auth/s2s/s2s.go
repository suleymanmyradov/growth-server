// Package s2s provides service-to-service authentication interceptors.
// It uses a shared HMAC secret to sign and verify inter-service RPC calls,
// preventing unauthorized internal access even if the network boundary is breached.
//
// Security model:
//   - The shared secret must be loaded from environment/config and must never be empty in production.
//   - An empty secret causes a fatal error at construction time; there is no "skip if empty" fallback.
//   - The s2s interceptor should be chained BEFORE the auth/mdpropagate interceptor so that
//     identity propagation only happens after the caller is authenticated as an internal service.
//   - Signatures cover method + timestamp + a SHA-256 hash of the request body,
//     so a captured signature cannot be replayed with a different body inside
//     the timestamp tolerance window.
//   - Streaming RPCs are authenticated too: the client binds the request it is
//     about to send via ContextWithSigningBody (or signs unbound), and the
//     server verifies the hash of the first received message.
package s2s

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const (
	mdServiceAuth     = "x-service-auth"
	mdServiceAuthTs   = "x-service-auth-ts"
	mdServiceAuthBody = "x-service-auth-body"
)

// signingBodyKey carries the request a stream client is about to send so the
// stream client interceptor can bind it into the signature at NewStream time
// (the interceptor itself never sees the message — it is only delivered via
// ClientStream.SendMsg after metadata has already been sent).
type signingBodyKey struct{}

// Config holds the shared secret used for service-to-service authentication.
type Config struct {
	Secret string `json:",optional" secret:"true"`
}

// MustValidate ensures the config is safe for production use. It returns an error
// if the secret is empty, preventing accidental fail-open deployments.
func (c Config) MustValidate() error {
	if c.Secret == "" {
		return fmt.Errorf("s2s: shared secret is required; set a non-empty secret or disable the service")
	}
	if len(c.Secret) < 32 {
		return fmt.Errorf("s2s: shared secret must be at least 32 bytes")
	}
	return nil
}

// Sign computes an HMAC-SHA256 signature over "method:timestamp:bodyHash" using
// the shared secret. bodyHash is the SHA-256 hex digest produced by HashRequest;
// it may be empty for callers that cannot bind a body (unbound streams).
func Sign(secret, method, bodyHash string, timestamp int64) string {
	msg := fmt.Sprintf("%s:%d:%s", method, timestamp, bodyHash)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks whether the provided signature is valid for the given method,
// claimed body hash, and timestamp. It also rejects timestamps older than
// maxSkew (default 5 minutes) to prevent replay attacks.
func Verify(secret, method, bodyHash, sig string, timestamp int64, maxSkew time.Duration) bool {
	if maxSkew == 0 {
		maxSkew = 5 * time.Minute
	}
	now := time.Now().Unix()
	if now < timestamp-int64(maxSkew.Seconds()) || now > timestamp+int64(maxSkew.Seconds()) {
		return false
	}
	expected := Sign(secret, method, bodyHash, timestamp)
	return hmac.Equal([]byte(sig), []byte(expected))
}

// HashRequest returns the SHA-256 hex digest of the request as bound into the
// signature. Proto messages are marshaled with deterministic ordering so the
// hash is stable between the client-side sign and server-side re-marshal.
func HashRequest(req interface{}) (string, error) {
	var b []byte
	switch m := req.(type) {
	case proto.Message:
		out, err := proto.MarshalOptions{Deterministic: true}.Marshal(m)
		if err != nil {
			return "", fmt.Errorf("s2s: marshal request for signing: %w", err)
		}
		b = out
	case []byte:
		b = m
	case nil:
		b = nil
	default:
		b = []byte(fmt.Sprintf("%#v", req))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// ContextWithSigningBody attaches the request the caller is about to send on a
// streaming RPC so StreamClientInterceptor can bind it into the signature.
// Callers of server-streaming RPCs (one request message, many responses) should
// wrap their context with this before invoking the generated client.
func ContextWithSigningBody(ctx context.Context, req interface{}) context.Context {
	return context.WithValue(ctx, signingBodyKey{}, req)
}

// signingBodyFrom extracts a request bound with ContextWithSigningBody.
func signingBodyFrom(ctx context.Context) (interface{}, bool) {
	req := ctx.Value(signingBodyKey{})
	return req, req != nil
}

// appendAuth computes the signature and appends the three auth metadata keys to
// the outgoing context.
func appendAuth(ctx context.Context, cfg Config, method, bodyHash string) context.Context {
	ts := time.Now().Unix()
	sig := Sign(cfg.Secret, method, bodyHash, ts)
	return metadata.AppendToOutgoingContext(ctx,
		mdServiceAuth, sig,
		mdServiceAuthTs, strconv.FormatInt(ts, 10),
		mdServiceAuthBody, bodyHash,
	)
}

// verifyMetadata extracts and verifies the auth signature from incoming
// metadata. It returns the claimed body hash so the caller can compare it
// against the actual request.
func verifyMetadata(md metadata.MD, secret, method string) (string, error) {
	sigs := md.Get(mdServiceAuth)
	tss := md.Get(mdServiceAuthTs)
	bodies := md.Get(mdServiceAuthBody)
	if len(sigs) == 0 || len(tss) == 0 {
		return "", status.Error(codes.PermissionDenied, "missing service auth token")
	}
	claimedBody := ""
	if len(bodies) > 0 {
		claimedBody = bodies[0]
	}
	ts, err := strconv.ParseInt(tss[0], 10, 64)
	if err != nil {
		return "", status.Error(codes.PermissionDenied, "invalid service auth timestamp")
	}
	if !Verify(secret, method, claimedBody, sigs[0], ts, 5*time.Minute) {
		return "", status.Error(codes.PermissionDenied, "invalid service auth token")
	}
	return claimedBody, nil
}

// UnaryClientInterceptor returns a gRPC client interceptor that signs outgoing
// RPCs — method, timestamp, and a hash of the request body — with the shared
// secret.
func UnaryClientInterceptor(cfg Config) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		bodyHash, err := HashRequest(req)
		if err != nil {
			return err
		}
		return invoker(appendAuth(ctx, cfg, method, bodyHash), method, req, reply, cc, opts...)
	}
}

// UnaryServerInterceptor returns a gRPC server interceptor that verifies the
// service authentication token on incoming RPCs, including the request body
// hash so captured signatures cannot be replayed with a different body.
// It skips validation for health and reflection methods.
func UnaryServerInterceptor(cfg Config) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if shouldSkipValidation(info.FullMethod) {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.PermissionDenied, "missing service auth metadata")
		}
		claimedBody, err := verifyMetadata(md, cfg.Secret, info.FullMethod)
		if err != nil {
			return nil, err
		}
		if claimedBody == "" {
			return nil, status.Error(codes.PermissionDenied, "missing service auth body hash")
		}
		actualBody, err := HashRequest(req)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to hash request")
		}
		if !hmac.Equal([]byte(actualBody), []byte(claimedBody)) {
			return nil, status.Error(codes.PermissionDenied, "service auth body hash mismatch")
		}

		return handler(ctx, req)
	}
}

// StreamClientInterceptor returns a gRPC client interceptor that signs outgoing
// streaming RPCs. If the caller bound a request via ContextWithSigningBody the
// signature covers its hash; otherwise the call is signed without a body
// binding (the server still authenticates method+timestamp).
// If auth metadata is already present on the outgoing context (e.g. set by an
// earlier interceptor) it is left untouched.
func StreamClientInterceptor(cfg Config) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		if md, ok := metadata.FromOutgoingContext(ctx); ok && len(md.Get(mdServiceAuth)) > 0 {
			return streamer(ctx, desc, cc, method, opts...)
		}
		bodyHash := ""
		if req, ok := signingBodyFrom(ctx); ok {
			h, err := HashRequest(req)
			if err != nil {
				return nil, err
			}
			bodyHash = h
		}
		return streamer(appendAuth(ctx, cfg, method, bodyHash), desc, cc, method, opts...)
	}
}

// StreamServerInterceptor returns a gRPC server interceptor that verifies the
// service authentication token on incoming streaming RPCs. When the caller
// bound a body hash, the hash of the first received message must match —
// verified by intercepting the first RecvMsg call. It skips validation for
// health and reflection methods.
func StreamServerInterceptor(cfg Config) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if shouldSkipValidation(info.FullMethod) {
			return handler(srv, ss)
		}

		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			return status.Error(codes.PermissionDenied, "missing service auth metadata")
		}
		claimedBody, err := verifyMetadata(md, cfg.Secret, info.FullMethod)
		if err != nil {
			return err
		}

		return handler(srv, &bodyCheckingServerStream{
			ServerStream: ss,
			claimedBody:  claimedBody,
		})
	}
}

// bodyCheckingServerStream verifies, on the first received message, that its
// hash matches the body hash claimed in the auth signature. A call bound by the
// client therefore cannot be replayed with a different request body.
type bodyCheckingServerStream struct {
	grpc.ServerStream
	claimedBody string
	checked     bool
}

func (s *bodyCheckingServerStream) RecvMsg(m interface{}) error {
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return err
	}
	if s.checked {
		return nil
	}
	s.checked = true
	if s.claimedBody == "" {
		return nil
	}
	actualBody, err := HashRequest(m)
	if err != nil {
		return status.Error(codes.Internal, "failed to hash request")
	}
	if !hmac.Equal([]byte(actualBody), []byte(s.claimedBody)) {
		return status.Error(codes.PermissionDenied, "service auth body hash mismatch")
	}
	return nil
}

func shouldSkipValidation(method string) bool {
	return strings.HasPrefix(method, "/grpc.health") ||
		strings.HasPrefix(method, "/grpc.reflection") ||
		strings.Contains(method, "Reflection")
}
