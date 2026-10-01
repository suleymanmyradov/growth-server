package logic

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/unsubtoken"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/pb/notifications"
)

// Invalid and missing tokens are rejected before any repository access, so
// these run without a database.
func TestUnsubscribeEmail_RejectsBadToken(t *testing.T) {
	svcCtx := &svc.ServiceContext{EmailUnsubscribeSecret: "test-secret"}
	l := NewUnsubscribeEmailLogic(context.Background(), svcCtx)

	for _, token := range []string{"", "garbage", unsubtoken.Sign("other-secret", uuid.New())} {
		_, err := l.UnsubscribeEmail(&notifications.UnsubscribeEmailRequest{Token: token})
		require.Error(t, err, "token %q", token)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	}
}

func TestUnsubscribeEmail_UnconfiguredSecret(t *testing.T) {
	l := NewUnsubscribeEmailLogic(context.Background(), &svc.ServiceContext{})
	_, err := l.UnsubscribeEmail(&notifications.UnsubscribeEmailRequest{Token: "x.y"})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}
