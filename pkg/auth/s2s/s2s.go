// Package s2s provides service-to-service authentication interceptors.
// It uses a shared HMAC secret to sign and verify inter-service RPC calls,
// preventing unauthorized internal access even if the network boundary is breached.
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
)

const (
	mdServiceAuth   = "x-service-auth"
	mdServiceAuthTs = "x-service-auth-ts"
)

// Config holds the shared secret used for service-to-service authentication.
type Config struct {
	Secret string
}

// Sign computes an HMAC-SHA256 signature over "method:timestamp" using the shared secret.
func Sign(secret, method string, timestamp int64) string {
	msg := fmt.Sprintf("%s:%d", method, timestamp)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks whether the provided signature is valid for the given method and timestamp.
// It also rejects timestamps older than maxSkew (default 5 minutes) to prevent replay attacks.
func Verify(secret, method, sig string, timestamp int64, maxSkew time.Duration) bool {
	if maxSkew == 0 {
		maxSkew = 5 * time.Minute
	}
	now := time.Now().Unix()
	if now < timestamp-int64(maxSkew.Seconds()) || now > timestamp+int64(maxSkew.Seconds()) {
		return false
	}
	expected := Sign(secret, method, timestamp)
	return hmac.Equal([]byte(sig), []byte(expected))
}

// UnaryClientInterceptor returns a gRPC client interceptor that signs outgoing RPCs
// with a service authentication token derived from the shared secret.
func UnaryClientInterceptor(cfg Config) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if cfg.Secret == "" {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
		ts := time.Now().Unix()
		sig := Sign(cfg.Secret, method, ts)
		ctx = metadata.AppendToOutgoingContext(ctx, mdServiceAuth, sig, mdServiceAuthTs, strconv.FormatInt(ts, 10))
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// UnaryServerInterceptor returns a gRPC server interceptor that verifies the
// service authentication token on incoming RPCs. It skips validation for health
// and reflection methods.
func UnaryServerInterceptor(cfg Config) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if cfg.Secret == "" {
			return handler(ctx, req)
		}
		if shouldSkipValidation(info.FullMethod) {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.PermissionDenied, "missing service auth metadata")
		}

		sigs := md.Get(mdServiceAuth)
		tss := md.Get(mdServiceAuthTs)
		if len(sigs) == 0 || len(tss) == 0 {
			return nil, status.Error(codes.PermissionDenied, "missing service auth token")
		}

		ts, err := strconv.ParseInt(tss[0], 10, 64)
		if err != nil {
			return nil, status.Error(codes.PermissionDenied, "invalid service auth timestamp")
		}

		if !Verify(cfg.Secret, info.FullMethod, sigs[0], ts, 5*time.Minute) {
			return nil, status.Error(codes.PermissionDenied, "invalid service auth token")
		}

		return handler(ctx, req)
	}
}

func shouldSkipValidation(method string) bool {
	return strings.HasPrefix(method, "/grpc.health") ||
		strings.HasPrefix(method, "/grpc.reflection") ||
		strings.Contains(method, "Reflection")
}
