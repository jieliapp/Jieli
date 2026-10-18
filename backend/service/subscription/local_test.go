package subscription_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jieliapp/jieli/backend/model"
	subscriptionservice "github.com/jieliapp/jieli/backend/service/subscription"
)

func TestLocalExpiryNeedsNoExternalServiceAndPreservesGift(t *testing.T) {
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = model.CloseDB() })
	now := time.Now().UTC()
	for _, remaining := range []time.Duration{-time.Hour, time.Hour} {
		user := &model.User{Username: remaining.String(), APIKeyHash: remaining.String(), PlanLevel: model.PlanLevelPro}
		if err := model.UserDB.Save(user); err != nil {
			t.Fatal(err)
		}
		if err := model.SubscriptionDB.Save(&model.Subscription{UserID: user.ID, PlanLevel: model.PlanLevelPro, Status: model.SubscriptionStatusActive, StartsAtMS: now.Add(-24 * time.Hour).UnixMilli(), EndsAtMS: now.Add(remaining).UnixMilli(), Source: model.SubscriptionSourceNeo}); err != nil {
			t.Fatal(err)
		}
	}
	service := subscriptionservice.Service{Now: func() time.Time { return now }}
	result := service.ExpireDue(context.Background())
	if result.Failed != 0 || result.Synced != 1 {
		t.Fatalf("local expiry failed: %+v", result)
	}
	downgraded, err := model.UserDB.ByID(1)
	if err != nil || downgraded.PlanLevel != model.PlanLevelFree {
		t.Fatalf("expired plan was not downgraded: %+v %v", downgraded, err)
	}

	expired, err := subscriptionservice.Current(1, now)
	if err != nil || expired.Status != subscriptionservice.SummaryStatusNone {
		t.Fatalf("expired=%+v err=%v", expired, err)
	}
	active, err := subscriptionservice.Current(2, now)
	if err != nil || active.ExpiresAt == nil || !active.ExpiresAt.Equal(time.UnixMilli(now.Add(time.Hour).UnixMilli())) {
		t.Fatalf("gift expiry changed: %+v err=%v", active, err)
	}
}

func TestEntitlementRequiresCurrentProPeriod(t *testing.T) {
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "period.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = model.CloseDB() })
	now := time.Now().UTC()
	for _, tc := range []struct {
		name, plan string
		start, end time.Duration
	}{
		{"future", "pro", time.Hour, 2 * time.Hour},
		{"free", "free", -time.Hour, time.Hour},
		{"expired", "pro", -time.Hour, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := &model.User{Username: tc.name, APIKeyHash: tc.name, PlanLevel: "pro"}
			if err := model.UserDB.Save(u); err != nil {
				t.Fatal(err)
			}
			for _, sub := range []*model.Subscription{
				{UserID: u.ID, PlanLevel: "pro", Status: "active", StartsAtMS: now.Add(-2 * time.Hour).UnixMilli(), EndsAtMS: now.Add(-time.Hour).UnixMilli()},
				{UserID: u.ID, PlanLevel: tc.plan, Status: "active", StartsAtMS: now.Add(tc.start).UnixMilli(), EndsAtMS: now.Add(tc.end).UnixMilli()},
			} {
				if err := model.SubscriptionDB.Save(sub); err != nil {
					t.Fatal(err)
				}
			}
			snapshot := *u
			if err := subscriptionservice.ApplyEntitlement(&snapshot, now); err != nil {
				t.Fatal(err)
			}
			if snapshot.PlanLevel != "free" {
				t.Error("request granted Pro without a current Pro period")
			}
			result := (subscriptionservice.Service{Now: func() time.Time { return now }}).ExpireDue(context.Background())
			if result.Failed != 0 {
				t.Fatal(result)
			}
			saved, err := model.UserDB.ByID(u.ID)
			if err != nil {
				t.Fatal(err)
			}
			if saved.PlanLevel != "free" {
				t.Error("expiry retained Pro without a current Pro period")
			}
		})
	}
}
