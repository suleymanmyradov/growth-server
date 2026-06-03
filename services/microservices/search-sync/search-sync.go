package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/configsafe"
	"github.com/suleymanmyradov/growth-server/services/microservices/search-sync/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/search-sync/internal/indexer"
	"github.com/suleymanmyradov/growth-server/services/microservices/search-sync/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/search-sync/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/search-sync/internal/syncer"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

var configFile = flag.String("f", "etc/search-sync.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	c.MustSetUp()
	logx.Infof("starting search-sync with config: %+v", configsafe.MaskSecrets(c))

	// Set defaults
	if c.Sync.PollInterval == 0 {
		c.Sync.PollInterval = 5 * time.Second
	}
	if c.Sync.BatchSize == 0 {
		c.Sync.BatchSize = 100
	}
	if c.Sync.LockTimeout == 0 {
		c.Sync.LockTimeout = 2 * time.Minute
	}
	if c.Sync.MaxAttempts == 0 {
		c.Sync.MaxAttempts = 5
	}
	if c.Sync.WorkerID == "" {
		c.Sync.WorkerID = fmt.Sprintf("search-sync-%d", os.Getpid())
	}
	if c.Meili.Index == "" {
		c.Meili.Index = "growth_search"
	}

	ctx := svc.NewServiceContext(c)
	defer ctx.Close()

	repo := repository.NewOutboxRepository(ctx.Pool)
	idx := indexer.NewMeiliIndexer(ctx.Index)
	sync := syncer.NewSyncer(repo, idx, c)

	if c.Backfill {
		logx.Info("running backfill...")
		if err := sync.Backfill(context.Background()); err != nil {
			logx.Must(fmt.Errorf("backfill failed: %w", err))
		}
		logx.Info("backfill complete")
		return
	}

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go sync.Run(rootCtx)

	logx.Info("search-sync started")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logx.Info("shutting down search-sync...")
	cancel()
	// Give workers a moment to finish current batch
	time.Sleep(500 * time.Millisecond)
}
