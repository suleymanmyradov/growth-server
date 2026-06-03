package runtime

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

type Stoppable interface{ Stop() }
type Closable interface{ Close() }

type Options struct {
	RPC          Stoppable
	REST         Stoppable
	OnShutdown   []func(context.Context) error
	ShutdownWait time.Duration
}

func Run(start func(ctx context.Context), opts Options) {
	if opts.ShutdownWait == 0 {
		opts.ShutdownWait = 10 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		start(ctx)
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	logx.Info("shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), opts.ShutdownWait)
	defer shutdownCancel()

	if opts.REST != nil {
		opts.REST.Stop()
	}
	if opts.RPC != nil {
		opts.RPC.Stop()
	}
	cancel()
	for _, fn := range opts.OnShutdown {
		_ = fn(shutdownCtx)
	}
	wg.Wait()
}
