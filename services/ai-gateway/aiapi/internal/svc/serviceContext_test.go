package svc

import (
	"context"
	"errors"
	"testing"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/config"
	clientrpc "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client"
	clientbilling "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/billingservice"
	clientpb "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"google.golang.org/grpc"
)

// mockBilling implements the client billing RPC surface for tests.
type mockBilling struct {
	planCode string
	status   string
	err      error
}

func (m *mockBilling) GetBillingOverview(_ context.Context, _ *clientbilling.GetBillingOverviewRequest, _ ...grpc.CallOption) (*clientbilling.GetBillingOverviewResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &clientbilling.GetBillingOverviewResponse{
		Entitlements: &clientpb.Entitlements{
			PlanCode: m.planCode,
			Status:   m.status,
		},
	}, nil
}

func (m *mockBilling) TrackUpgradeEvent(_ context.Context, _ *clientbilling.TrackUpgradeEventRequest, _ ...grpc.CallOption) (*clientbilling.TrackUpgradeEventResponse, error) {
	return nil, nil
}
func (m *mockBilling) CreateCustomerPortalSession(_ context.Context, _ *clientbilling.CreateCustomerPortalSessionRequest, _ ...grpc.CallOption) (*clientbilling.CreateCustomerPortalSessionResponse, error) {
	return nil, nil
}
func (m *mockBilling) HandleRevenueCatWebhook(_ context.Context, _ *clientbilling.HandleRevenueCatWebhookRequest, _ ...grpc.CallOption) (*clientbilling.HandleRevenueCatWebhookResponse, error) {
	return nil, nil
}
func (m *mockBilling) HandlePaddleWebhook(_ context.Context, _ *clientbilling.HandlePaddleWebhookRequest, _ ...grpc.CallOption) (*clientbilling.HandlePaddleWebhookResponse, error) {
	return nil, nil
}
func (m *mockBilling) CreatePaddleCheckout(_ context.Context, _ *clientbilling.CreatePaddleCheckoutRequest, _ ...grpc.CallOption) (*clientbilling.CreatePaddleCheckoutResponse, error) {
	return nil, nil
}
func (m *mockBilling) ListSubscriptionStatuses(_ context.Context, _ *clientbilling.ListSubscriptionStatusesRequest, _ ...grpc.CallOption) (*clientbilling.ListSubscriptionStatusesResponse, error) {
	return nil, nil
}

// mockEdgeQuotaStore is a test double for the edge quota store.
type mockEdgeQuotaStore struct {
	checkOK bool
	err     error
}

func (m *mockEdgeQuotaStore) CheckUserQuota(_ context.Context, _ string, _ int64) (bool, error) {
	return m.checkOK, m.err
}
func (m *mockEdgeQuotaStore) UserDailyTokens(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (m *mockEdgeQuotaStore) IncrUserTokens(_ context.Context, _ string, _ int64) error { return nil }
func (m *mockEdgeQuotaStore) CheckGlobalQuota(_ context.Context, _ int64) (bool, error) {
	return true, nil
}
func (m *mockEdgeQuotaStore) IncrGlobalCost(_ context.Context, _ int64) error { return nil }
func (m *mockEdgeQuotaStore) CheckUserVoiceQuota(_ context.Context, _ string, _ int64) (bool, error) {
	return m.checkOK, m.err
}
func (m *mockEdgeQuotaStore) IncrUserVoiceSeconds(_ context.Context, _ string, _ int64) error {
	return nil
}

func newTestSvcCtx(billing clientbilling.BillingService, store ai.QuotaStore, proCap, freeCap int64) *ServiceContext {
	return &ServiceContext{
		ClientRpc:  &clientrpc.Service{BillingService: billing},
		QuotaStore: store,
		Config: config.Config{
			AI: ai.Config{
				Quota: ai.QuotaConfig{
					UserDailyTokenCap:     proCap,
					FreeUserDailyTokenCap: freeCap,
				},
			},
		},
	}
}

// Compile-time assertion that the mock satisfies the billing interface.
var _ clientbilling.BillingService = (*mockBilling)(nil)

func TestDailyTokenCap_ProPlan(t *testing.T) {
	s := newTestSvcCtx(&mockBilling{planCode: "pro", status: "active"}, nil, 500000, 100000)
	if got := s.DailyTokenCap(context.Background(), "user-1"); got != 500000 {
		t.Fatalf("DailyTokenCap(pro) = %d, want 500000", got)
	}
}

func TestDailyTokenCap_FreePlan(t *testing.T) {
	s := newTestSvcCtx(&mockBilling{planCode: "free", status: "free"}, nil, 500000, 100000)
	if got := s.DailyTokenCap(context.Background(), "user-1"); got != 100000 {
		t.Fatalf("DailyTokenCap(free) = %d, want 100000", got)
	}
}

func TestDailyTokenCap_ExpiredProGetsFreeCap(t *testing.T) {
	s := newTestSvcCtx(&mockBilling{planCode: "pro", status: "expired"}, nil, 500000, 100000)
	if got := s.DailyTokenCap(context.Background(), "user-1"); got != 100000 {
		t.Fatalf("DailyTokenCap(expired pro) = %d, want 100000", got)
	}
}

func TestDailyTokenCap_BillingErrorFallsBackToFreeCap(t *testing.T) {
	s := newTestSvcCtx(&mockBilling{err: context.DeadlineExceeded}, nil, 500000, 100000)
	if got := s.DailyTokenCap(context.Background(), "user-1"); got != 100000 {
		t.Fatalf("DailyTokenCap(billing error) = %d, want 100000", got)
	}
}

func TestDailyTokenCap_NoPlanAwareness(t *testing.T) {
	s := newTestSvcCtx(&mockBilling{planCode: "free", status: "free"}, nil, 500000, 0)
	if got := s.DailyTokenCap(context.Background(), "user-1"); got != 500000 {
		t.Fatalf("DailyTokenCap(free cap unset) = %d, want 500000", got)
	}
}

func TestCheckDailyTokenQuota_FailsClosedWhenNoStore(t *testing.T) {
	// A configured cap with a missing quota store must block, not silently
	// allow — a Redis outage can never disable enforcement.
	s := newTestSvcCtx(&mockBilling{planCode: "free", status: "free"}, nil, 500000, 100000)
	err := s.CheckDailyTokenQuota(context.Background(), "user-1")
	if err == nil {
		t.Fatal("CheckDailyTokenQuota with nil store = nil, want fail-closed quota error")
	}
	if !errors.Is(err, ai.ErrQuotaExceeded) {
		t.Fatalf("CheckDailyTokenQuota error = %v, want QuotaError wrapping ErrQuotaExceeded", err)
	}
}

func TestCheckDailyTokenQuota_DisabledWhenNoCap(t *testing.T) {
	// No caps configured → no enforcement, nil store is fine.
	s := newTestSvcCtx(&mockBilling{planCode: "free", status: "free"}, nil, 0, 0)
	if err := s.CheckDailyTokenQuota(context.Background(), "user-1"); err != nil {
		t.Fatalf("CheckDailyTokenQuota with no caps = %v, want nil", err)
	}
}

func TestCheckDailyTokenQuota_Exceeded(t *testing.T) {
	s := newTestSvcCtx(&mockBilling{planCode: "free", status: "free"}, &mockEdgeQuotaStore{checkOK: false}, 500000, 100000)
	if err := s.CheckDailyTokenQuota(context.Background(), "user-1"); err == nil {
		t.Fatal("CheckDailyTokenQuota exceeded = nil, want ErrQuotaExceeded")
	}
}

func TestCheckDailyTokenQuota_StoreErrorFailsClosed(t *testing.T) {
	s := newTestSvcCtx(&mockBilling{planCode: "free", status: "free"}, &mockEdgeQuotaStore{checkOK: false, err: context.DeadlineExceeded}, 500000, 100000)
	err := s.CheckDailyTokenQuota(context.Background(), "user-1")
	if err == nil {
		t.Fatal("CheckDailyTokenQuota store error = nil, want fail-closed quota error")
	}
	if !errors.Is(err, ai.ErrQuotaExceeded) {
		t.Fatalf("CheckDailyTokenQuota error = %v, want QuotaError wrapping ErrQuotaExceeded", err)
	}
}

func TestCheckDailyVoiceQuota(t *testing.T) {
	newSvc := func(store ai.QuotaStore, voiceCap int64) *ServiceContext {
		s := newTestSvcCtx(&mockBilling{planCode: "free", status: "free"}, store, 500000, 100000)
		s.Config.AI.Quota.UserDailyVoiceSecondsCap = voiceCap
		return s
	}

	t.Run("disabled when cap unset", func(t *testing.T) {
		if err := newSvc(nil, 0).CheckDailyVoiceQuota(context.Background(), "u"); err != nil {
			t.Fatalf("no cap = %v, want nil", err)
		}
	})
	t.Run("fails closed when no store", func(t *testing.T) {
		if err := newSvc(nil, 1800).CheckDailyVoiceQuota(context.Background(), "u"); err == nil {
			t.Fatal("nil store with cap = nil, want fail-closed")
		}
	})
	t.Run("fails closed on store error", func(t *testing.T) {
		if err := newSvc(&mockEdgeQuotaStore{err: context.DeadlineExceeded}, 1800).CheckDailyVoiceQuota(context.Background(), "u"); err == nil {
			t.Fatal("store error = nil, want fail-closed")
		}
	})
	t.Run("rejects when over cap", func(t *testing.T) {
		err := newSvc(&mockEdgeQuotaStore{checkOK: false}, 1800).CheckDailyVoiceQuota(context.Background(), "u")
		if err == nil || !errors.Is(err, ai.ErrQuotaExceeded) {
			t.Fatalf("over cap = %v, want quota error", err)
		}
	})
	t.Run("allows when under cap", func(t *testing.T) {
		if err := newSvc(&mockEdgeQuotaStore{checkOK: true}, 1800).CheckDailyVoiceQuota(context.Background(), "u"); err != nil {
			t.Fatalf("under cap = %v, want nil", err)
		}
	})
}
