// Package recovery provides gRPC server interceptors that recover from panics,
// log the stack trace, and return a sanitized Internal error to the caller.
package recovery

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor returns a gRPC unary server interceptor that recovers
// from panics in the handler chain. It should be the outermost interceptor so
// that recoveries cover the entire request lifecycle.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				logx.WithContext(ctx).Errorw("panic recovered in gRPC handler",
					logx.Field("method", info.FullMethod),
					logx.Field("panic", fmt.Sprintf("%v", r)),
					logx.Field("stack", string(debug.Stack())),
				)
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}
