package subscription_test

import (
	"context"
	"testing"
	"time"

	subscriptionservice "github.com/jieliapp/jieli/backend/service/subscription"
)

func TestStartPeriodicSyncShouldRunImmediatelyAndStopOnCancel(t *testing.T) {
	ctx := context.Background()
	calls := make(chan struct{}, 2)

	stop := subscriptionservice.StartPeriodicSync(ctx, time.Hour, func(context.Context) subscriptionservice.SyncAllResult {
		calls <- struct{}{}
		return subscriptionservice.SyncAllResult{Scanned: 1, Synced: 1}
	}, nil)
	defer stop()

	select {
	case <-calls:
	case <-time.After(time.Second):
		t.Fatalf("expected periodic sync to run immediately")
	}
	stop()
	select {
	case <-calls:
		t.Fatalf("expected periodic sync to stop")
	case <-time.After(20 * time.Millisecond):
	}
}
