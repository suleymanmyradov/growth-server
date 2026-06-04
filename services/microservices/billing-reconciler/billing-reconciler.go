package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/configsafe"
	"github.com/suleymanmyradov/growth-server/services/microservices/billing-reconciler/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/billing-reconciler/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/billing-reconciler/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

var configFile = flag.String("f", "etc/billing-reconciler.yaml", "the config file")
var dryRun = flag.Bool("dry-run", false, "print what would be changed without applying")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	logx.Infof("starting billing-reconciler with config: %+v", configsafe.MaskSecrets(c))

	ctx := svc.NewServiceContext(c)
	defer ctx.Close()

	reconciled, err := reconcileExpiredSubscriptions(context.Background(), ctx, *dryRun)
	if err != nil {
		logx.Must(fmt.Errorf("reconciliation failed: %w", err))
	}

	logx.Infof("billing-reconciler complete: %d subscriptions reconciled", reconciled)
	os.Exit(0)
}

// reconcileExpiredSubscriptions finds active/trialing subscriptions whose
// cancel_at_period_end=true and current_period_end has passed, and downgrades
// them to the free plan. This protects against missed webhooks.
func reconcileExpiredSubscriptions(ctx context.Context, s *svc.ServiceContext, dry bool) (int, error) {
	const batchSize = 100

	repo := repository.NewBillingRepository(s.Pool)

	expiredSubs, err := repo.ListExpiredActiveSubscriptions(ctx, batchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to list expired subscriptions: %w", err)
	}

	if len(expiredSubs) == 0 {
		return 0, nil
	}

	freePlan, err := repo.GetPlanByCode(ctx, "free")
	if err != nil {
		return 0, fmt.Errorf("failed to get free plan: %w", err)
	}

	reconciled := 0
	for _, sub := range expiredSubs {
		logx.Infof("reconciling expired subscription: user=%s status=%s period_end=%s",
			sub.UserID, sub.Status, sub.CurrentPeriodEnd.Time.Format(time.RFC3339))

		if dry {
			continue
		}

		_, upsertErr := repo.UpsertUserSubscription(ctx, repository.UpsertUserSubscriptionParams{
			UserID:               sub.UserID,
			PlanID:               freePlan.ID,
			Status:               repository.SubscriptionStatusTypeExpired,
			BillingInterval:      sub.BillingInterval,
			CurrentPeriodStart:   sub.CurrentPeriodStart,
			CurrentPeriodEnd:     sub.CurrentPeriodEnd,
			TrialEnd:             sub.TrialEnd,
			CancelAtPeriodEnd:    false,
			StripeCustomerID:     sub.StripeCustomerID,
			StripeSubscriptionID: sub.StripeSubscriptionID,
		})
		if upsertErr != nil {
			logx.Errorf("failed to downgrade expired subscription for user %s: %v", sub.UserID, upsertErr)
			continue
		}
		reconciled++
	}

	return reconciled, nil
}
