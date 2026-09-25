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

// paddleCheckoutServer stubs the Paddle API and captures the transaction body.
type paddleCheckoutServer struct {
	*httptest.Server
	lastTxnBody map[string]any
}

func newPaddleCheckoutServer(t *testing.T) *paddleCheckoutServer {
	t.Helper()
	s := &paddleCheckoutServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/transactions" {
			_ = json.NewDecoder(r.Body).Decode(&s.lastTxnBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"id":"txn_01h_test","status":"draft","checkout":{"url":"https://checkout.test/pay?_ptxn=txn_01h_test"}},"meta":{"request_id":"req_1"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(s.Close)
	return s
}

func paddleCheckoutLogic(t *testing.T, m *paddleMockBilling, apiURL string) *CreatePaddleCheckoutLogic {
	t.Helper()
	cfg := config.Config{}
	cfg.Billing.Paddle.Enabled = true
	var pc *paddle.Client
	if apiURL != "" {
		pc = paddle.NewClient("pdl_sdbx_test", true, nil, paddle.WithBaseURL(apiURL))
	}
	return &CreatePaddleCheckoutLogic{
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

func TestCreatePaddleCheckout_HappyPath(t *testing.T) {
	ts := newPaddleCheckoutServer(t)
	m := &paddleMockBilling{getSubErr: pgx.ErrNoRows}
	l := paddleCheckoutLogic(t, m, ts.URL)

	resp, err := l.CreatePaddleCheckout(&client.CreatePaddleCheckoutRequest{
		PriceId:     "pri_01m340f2f38cbr9ndj77bmpbzc",
		SuccessUrl:  "https://app.example.com/ok",
		CancelUrl:   "https://app.example.com/cancel",
		CheckoutUrl: "https://app.example.com/checkout",
	})
	require.NoError(t, err)
	assert.Equal(t, "txn_01h_test", resp.TransactionId)
	assert.Equal(t, "https://checkout.test/pay?_ptxn=txn_01h_test", resp.CheckoutUrl)

	// custom_data.user_id stamped for webhook user mapping.
	cd, ok := ts.lastTxnBody["custom_data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, paddleTestUserID.String(), cd["user_id"])

	items, ok := ts.lastTxnBody["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	assert.Equal(t, "pri_01m340f2f38cbr9ndj77bmpbzc", items[0].(map[string]any)["price_id"])

	// Redirect params ride on the checkout URL sent to Paddle.
	co, ok := ts.lastTxnBody["checkout"].(map[string]any)
	require.True(t, ok)
	checkoutURL := co["url"].(string)
	assert.Contains(t, checkoutURL, "success_url=")
	assert.Contains(t, checkoutURL, "cancel_url=")

	// No stored Paddle customer — none sent.
	assert.NotContains(t, ts.lastTxnBody, "customer_id")
}

func TestCreatePaddleCheckout_ReusesCustomer(t *testing.T) {
	ts := newPaddleCheckoutServer(t)
	ctm := "ctm_01h8441jn5pcwrfhwh78jqt8hk"
	m := &paddleMockBilling{getSub: db.GetUserSubscriptionRow{
		UserID:           paddleTestUserID,
		PaddleCustomerID: &ctm,
	}}
	l := paddleCheckoutLogic(t, m, ts.URL)

	_, err := l.CreatePaddleCheckout(&client.CreatePaddleCheckoutRequest{
		PriceId: "pri_01m340f2f38cbr9ndj77bmpbzc",
	})
	require.NoError(t, err)
	assert.Equal(t, ctm, ts.lastTxnBody["customer_id"])
}

func TestCreatePaddleCheckout_NoPrincipal(t *testing.T) {
	l := paddleCheckoutLogic(t, &paddleMockBilling{}, "http://unused")
	l.ctx = context.Background()

	_, err := l.CreatePaddleCheckout(&client.CreatePaddleCheckoutRequest{PriceId: "pri_x"})
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestCreatePaddleCheckout_InvalidPrice(t *testing.T) {
	l := paddleCheckoutLogic(t, &paddleMockBilling{}, "http://unused")

	_, err := l.CreatePaddleCheckout(&client.CreatePaddleCheckoutRequest{PriceId: "pro-monthly"})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCreatePaddleCheckout_Disabled(t *testing.T) {
	l := paddleCheckoutLogic(t, &paddleMockBilling{}, "")
	l.svcCtx.Config.Billing.Paddle.Enabled = false

	_, err := l.CreatePaddleCheckout(&client.CreatePaddleCheckoutRequest{PriceId: "pri_x"})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}
