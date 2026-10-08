package billingservicelogic

import (
	"context"
	"encoding/json"
	"errors"
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
	lastTxnBody      map[string]any
	lastCustomerBody map[string]any
}

func newPaddleCheckoutServer(t *testing.T) *paddleCheckoutServer {
	t.Helper()
	s := &paddleCheckoutServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/customers" {
			_ = json.NewDecoder(r.Body).Decode(&s.lastCustomerBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"id":"ctm_01h_new","email":"jane@example.com","status":"active"},"meta":{"request_id":"req_c"}}`))
			return
		}
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

func paddleCheckoutLogic(t *testing.T, m *mockBilling, apiURL string) *CreatePaddleCheckoutLogic {
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
	m := newMockBilling()
	m.getSubErr = pgx.ErrNoRows
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

	// The transaction → user binding is recorded server-side so the webhook
	// never has to trust client-side custom_data (B4).
	require.Len(t, m.checkoutRecords, 1)
	assert.Equal(t, "txn_01h_test", m.checkoutRecords[0].transactionID)
	assert.Equal(t, paddleTestUserID, m.checkoutRecords[0].userID)
}

func TestCreatePaddleCheckout_RecordFailureFailsCheckout(t *testing.T) {
	// If the txn→user record can't be persisted the payment could never be
	// mapped — fail the checkout rather than let the user pay into the void.
	ts := newPaddleCheckoutServer(t)
	m := newMockBilling()
	m.getSubErr = pgx.ErrNoRows
	m.checkoutErr = errors.New("db down")
	l := paddleCheckoutLogic(t, m, ts.URL)

	_, err := l.CreatePaddleCheckout(&client.CreatePaddleCheckoutRequest{
		PriceId: "pri_01m340f2f38cbr9ndj77bmpbzc",
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestCreatePaddleCheckout_ReusesCustomer(t *testing.T) {
	ts := newPaddleCheckoutServer(t)
	ctm := "ctm_01h8441jn5pcwrfhwh78jqt8hk"
	m := newMockBilling()
	m.getSub = db.GetUserSubscriptionRow{
		UserID:           paddleTestUserID,
		PaddleCustomerID: &ctm,
	}
	l := paddleCheckoutLogic(t, m, ts.URL)

	_, err := l.CreatePaddleCheckout(&client.CreatePaddleCheckoutRequest{
		PriceId: "pri_01m340f2f38cbr9ndj77bmpbzc",
	})
	require.NoError(t, err)
	assert.Equal(t, ctm, ts.lastTxnBody["customer_id"])
}

func TestCreatePaddleCheckout_FirstTimeBuyerEmailPrefilled(t *testing.T) {
	// No stored ctm_ id + gateway-supplied email → customer resolved up front
	// so Paddle checkout doesn't ask the user to type their email.
	ts := newPaddleCheckoutServer(t)
	m := newMockBilling()
	m.getSubErr = pgx.ErrNoRows
	l := paddleCheckoutLogic(t, m, ts.URL)

	_, err := l.CreatePaddleCheckout(&client.CreatePaddleCheckoutRequest{
		PriceId:       "pri_01m340f2f38cbr9ndj77bmpbzc",
		CustomerEmail: "  jane@example.com ",
	})
	require.NoError(t, err)
	assert.Equal(t, "jane@example.com", ts.lastCustomerBody["email"])
	assert.Equal(t, "ctm_01h_new", ts.lastTxnBody["customer_id"])
}

func TestCreatePaddleCheckout_StoredCustomerWinsOverEmail(t *testing.T) {
	ts := newPaddleCheckoutServer(t)
	ctm := "ctm_01h8441jn5pcwrfhwh78jqt8hk"
	m := newMockBilling()
	m.getSub = db.GetUserSubscriptionRow{UserID: paddleTestUserID, PaddleCustomerID: &ctm}
	l := paddleCheckoutLogic(t, m, ts.URL)

	_, err := l.CreatePaddleCheckout(&client.CreatePaddleCheckoutRequest{
		PriceId:       "pri_01m340f2f38cbr9ndj77bmpbzc",
		CustomerEmail: "jane@example.com",
	})
	require.NoError(t, err)
	assert.Nil(t, ts.lastCustomerBody, "no customer creation when a ctm_ id is stored")
	assert.Equal(t, ctm, ts.lastTxnBody["customer_id"])
}

func TestCreatePaddleCheckout_NoPrincipal(t *testing.T) {
	l := paddleCheckoutLogic(t, newMockBilling(), "http://unused")
	l.ctx = context.Background()

	_, err := l.CreatePaddleCheckout(&client.CreatePaddleCheckoutRequest{PriceId: "pri_x"})
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestCreatePaddleCheckout_InvalidPrice(t *testing.T) {
	l := paddleCheckoutLogic(t, newMockBilling(), "http://unused")

	_, err := l.CreatePaddleCheckout(&client.CreatePaddleCheckoutRequest{PriceId: "pro-monthly"})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCreatePaddleCheckout_Disabled(t *testing.T) {
	l := paddleCheckoutLogic(t, newMockBilling(), "")
	l.svcCtx.Config.Billing.Paddle.Enabled = false

	_, err := l.CreatePaddleCheckout(&client.CreatePaddleCheckoutRequest{PriceId: "pri_x"})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}
