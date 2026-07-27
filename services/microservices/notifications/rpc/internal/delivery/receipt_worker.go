package delivery

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/pkg/notifications/expo"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
)

// ReceiptWorker periodically checks Expo push receipt status for persisted
// tickets and disables stale tokens when Expo reports DeviceNotRegistered.
//
// Expo's push API is two-phase: Send returns a ticket immediately (accepted),
// but the actual delivery result is available later via getReceipts. Some
// errors — especially DeviceNotRegistered — only appear in the receipt, not in
// the initial ticket response. This worker bridges that gap.
//
// The worker is safe to run when Expo is disabled: Run exits immediately if
// the Expo client is nil.
type ReceiptWorker struct {
	devices *repository.DevicesRepo
	tickets *repository.PushTicketsRepo
	expo    *expo.Client
	// batchSize is the max number of pending tickets to check per cycle.
	batchSize int32
	// interval is how long to wait between receipt-check cycles.
	interval time.Duration
	// ticketMinAge is how long to wait after a ticket is created before checking
	// its receipt (Expo needs time to process the delivery).
	ticketMinAge time.Duration
}

// NewReceiptWorker creates a receipt worker. When expo is nil, Run is a no-op.
func NewReceiptWorker(devices *repository.DevicesRepo, tickets *repository.PushTicketsRepo, expoClient *expo.Client) *ReceiptWorker {
	return &ReceiptWorker{
		devices:      devices,
		tickets:      tickets,
		expo:         expoClient,
		batchSize:    100,
		interval:     30 * time.Second,
		ticketMinAge: 5 * time.Second,
	}
}

// Run starts the receipt worker loop. It blocks until ctx is cancelled. Call
// in a goroutine. Safe to call when Expo is disabled (exits immediately).
func (w *ReceiptWorker) Run(ctx context.Context) {
	if w.expo == nil || w.tickets == nil {
		logx.WithContext(ctx).Info("receipt worker: Expo client or tickets repo is nil, not starting")
		return
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	logx.WithContext(ctx).Infof("receipt worker: started (interval=%s, batch=%d, minAge=%s)", w.interval, w.batchSize, w.ticketMinAge)

	for {
		select {
		case <-ctx.Done():
			logx.WithContext(ctx).Info("receipt worker: stopping")
			return
		case <-ticker.C:
			if err := w.checkOnce(ctx); err != nil {
				logx.WithContext(ctx).Errorf("receipt worker: check cycle failed: %v", err)
			}
		}
	}
}

// checkOnce performs one receipt-check cycle: fetches pending tickets, checks
// their receipts at Expo, updates ticket status, and disables stale tokens.
func (w *ReceiptWorker) checkOnce(ctx context.Context) error {
	pending, err := w.tickets.ListPendingPushTickets(ctx, w.batchSize)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}

	// Skip tickets that are too young (Expo needs time to process).
	cutoff := time.Now().Add(-w.ticketMinAge)
	var eligible []db.PushTicket
	for _, t := range pending {
		if t.CreatedAt.Valid && t.CreatedAt.Time.Before(cutoff) {
			eligible = append(eligible, t)
		}
	}
	if len(eligible) == 0 {
		return nil
	}

	// Collect ticket IDs for the batch receipt check.
	ticketIDs := make([]string, 0, len(eligible))
	for _, t := range eligible {
		ticketIDs = append(ticketIDs, t.TicketID)
	}

	receipts, err := w.expo.CheckReceipts(ctx, ticketIDs)
	if err != nil {
		return err
	}

	// Process each eligible ticket against its receipt.
	for _, t := range eligible {
		receipt, ok := receipts[t.TicketID]
		if !ok {
			// Receipt not yet available; leave pending for the next cycle.
			continue
		}

		if receipt.Status == "ok" {
			if err := w.tickets.MarkPushTicketReceiptOK(ctx, t.TicketID, t.PushToken); err != nil {
				logx.WithContext(ctx).Errorf("receipt worker: mark ok for ticket %s failed: %v", t.TicketID, err)
			}
			continue
		}

		// Error receipt: record the error and disable stale tokens.
		errDetail := receipt.Details["error"]
		if errDetail == "" {
			errDetail = receipt.Message
		}
		if err := w.tickets.MarkPushTicketReceiptError(ctx, t.TicketID, t.PushToken, errDetail); err != nil {
			logx.WithContext(ctx).Errorf("receipt worker: mark error for ticket %s failed: %v", t.TicketID, err)
		}
		if errDetail == "DeviceNotRegistered" {
			if derr := w.devices.DisableDeviceByToken(ctx, t.PushToken); derr != nil {
				logx.WithContext(ctx).Errorf("receipt worker: disable stale token %s failed: %v", t.PushToken, derr)
			}
		}
	}

	// Best-effort cleanup of old processed tickets (older than 7 days).
	sevenDaysAgo := pgtype.Timestamptz{Time: time.Now().Add(-7 * 24 * time.Hour), Valid: true}
	if err := w.tickets.DeleteOldPushTickets(ctx, sevenDaysAgo); err != nil {
		logx.WithContext(ctx).Errorf("receipt worker: cleanup old tickets failed: %v", err)
	}

	return nil
}
