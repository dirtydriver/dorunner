package main

import (
	"context"
	"time"
	_ "time/tzdata" // embed zoneinfo so TZ works regardless of the runtime image

	"github.com/dirtydriver/dorunner/internal"
)

func main() {
	cfg := internal.LoadConfig()

	do := internal.NewDOClient(cfg)
	github := internal.NewGitHubClient(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go do.StartSnapshotRefresher(ctx, cfg.RunnerTTL)
	go do.StartCleanupCron(ctx, time.Minute*10, cfg.RunnerTTL)

	wbh := internal.NewWebhookHandler(cfg, do, github)

	internal.StartServer(cfg, wbh)
}
