package subscription

import (
	"context"
	"time"
)

type SyncAllFunc func(context.Context) SyncAllResult

type LogFunc func(format string, args ...any)

func StartPeriodicSync(ctx context.Context, interval time.Duration, syncAll SyncAllFunc, logf LogFunc) func() {
	if interval <= 0 || syncAll == nil {
		return func() {}
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runSubscriptionSync(runCtx, syncAll, logf)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				runSubscriptionSync(runCtx, syncAll, logf)
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func runSubscriptionSync(ctx context.Context, syncAll SyncAllFunc, logf LogFunc) {
	if ctx.Err() != nil {
		return
	}
	result := syncAll(ctx)
	if logf != nil {
		logf("subscription-sync: scanned=%d synced=%d failed=%d", result.Scanned, result.Synced, result.Failed)
	}
}
