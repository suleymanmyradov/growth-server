package billingservicelogic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/paddle"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// paddlePortalServer stubs the portal-sessions endpoint and captures the body.
type paddlePortalServer struct {
	*httptest.Server
	lastBody map[string]any
	lastPath string
}

func newPaddlePortalServer(t *testing.T) *paddlePortalServer {
	t.Helper()
	s := &paddlePortalServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.lastPath = r.URL.Path
		if r.Method == http.MethodPost && r.URL.Path == "/customers/ctm_01h_test/portal-sessions" {
			if r.Body != nil {
				_ = json.NewDecoder(r.Body).Decode(&s.lastBody)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"id":"ps_01h_test","urls":{"general":{"overview":"https://portal.test/overview"},"subscriptions":[{"subscription_id":"sub_01h_test","cancel_subscription":"https://portal.test/cancel"}]}},"meta":{"request_id":"req_1"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(s.Close)
	return s
}

func portalLogic(t *testing.T, m *paddleMockBilling, apiURL string, paddleEnabled bool) *CreateCustomerPortalSessionLogic {
	t.Helper()
	cfg := config.Config{}
	cfg.Billing.Paddle.Enabled = paddleEnabled
	var pc *paddle.Client
	if apiURL != "" {
		pc = paddle.NewClient("pdl_sdbx_test", true, nil, paddle.WithBaseURL(apiURL))
	}
	return &CreateCustomerPortalSessionLogic{
		ctx: principal.WithPrincipal(context.Background(), principal.Principal{
			UserID:   paddleTestUserID.String(),
			Username: "tester",
		}),
		svcCtx: &svc.ServiceContext{
			Config:       cfg,
			Repo:         &repository.Repository{Billing: m},
			PaddleClient: pc,
		},
		Logger: logx.WithContext(context.Background()),
	}
}

func TestCreateCustomerPortalSession_PaddleHappyPath(t *testing.T) {
	ts := newPaddlePortalServer(t)
	ctm := "ctm_01h_test"
	subID := "sub_01h_test"
	m := &paddleMockBilling{getSub: db.GetUserSubscriptionRow{
		UserID:               paddleTestUserID,
		PaddleCustomerID:     &ctm,
		PaddleSubscriptionID: &subID,
	}}
	l := portalLogic(t, m, ts.URL, true)

	resp, err := l.CreateCustomerPortalSession(&client.CreateCustomerPortalSessionRequest{})
	require.NoError(t, err)
	assert.Equal(t, "https://portal.test/overview", resp.PortalUrl)
	assert.Equal(t, "/customers/ctm_01h_test/portal-sessions", ts.lastPath)
	assert.Equal(t, []any{"sub_01h_test"}, ts.lastBody["subscription_ids"])
}

func TestCreateCustomerPortalSession_PaddleDisabledFallsBack(t *testing.T) {
	// Paddle disabled → empty portal URL.
	ctm := "ctm_01h_test"
	m := &paddleMockBilling{getSub: db.GetUserSubscriptionRow{
		UserID:           paddleTestUserID,
		PaddleCustomerID: &ctm,
	}}
	l := portalLogic(t, m, "http://unused", false)

	resp, err := l.CreateCustomerPortalSession(&client.CreateCustomerPortalSessionRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.PortalUrl)
}

func TestCreateCustomerPortalSession_NoPaddleCustomerFallsBack(t *testing.T) {
	// Paddle enabled but the user has no Paddle customer ID → empty portal URL.
	m := &paddleMockBilling{getSub: db.GetUserSubscriptionRow{UserID: paddleTestUserID}}
	l := portalLogic(t, m, "http://unused", true)

	resp, err := l.CreateCustomerPortalSession(&client.CreateCustomerPortalSessionRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.PortalUrl)
}

func TestCreateCustomerPortalSession_NoSubscription(t *testing.T) {
	m := &paddleMockBilling{getSubErr: pgx.ErrNoRows}
	l := portalLogic(t, m, "http://unused", true)

	_, err := l.CreateCustomerPortalSession(&client.CreateCustomerPortalSessionRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}
