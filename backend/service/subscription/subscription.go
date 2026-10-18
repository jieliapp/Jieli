package subscription

import (
	"errors"

	"github.com/burugo/thing"
	"time"

	"github.com/jieliapp/jieli/backend/model"
)

const SummaryStatusNone = "none"

type Summary struct {
	Status    string     `json:"status"`
	PlanLevel string     `json:"plan_level"`
	StartsAt  *time.Time `json:"starts_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Source    string     `json:"source,omitempty"`
}

type SyncAllResult struct {
	Scanned int
	Synced  int
	Failed  int
}

type Service struct {
	Now func() time.Time
}

func Current(userID int64, now time.Time) (Summary, error) {
	if userID == 0 {
		return Summary{}, errors.New("user id is required")
	}
	rows, err := effective(model.SubscriptionDB, userID, now.UnixMilli()).Fetch(0, 1)
	if err != nil {
		return Summary{}, err
	}
	if len(rows) == 0 {
		return Summary{Status: SummaryStatusNone, PlanLevel: model.PlanLevelFree}, nil
	}
	return summaryFromModel(rows[0]), nil
}

// ApplyEntitlement resolves membership at request time, so a scheduler interval
// never extends a paid entitlement. Explicit gifts without subscription rows
// retain their existing plan.
func ApplyEntitlement(user *model.User, now time.Time) error {
	if user == nil || user.ID == 0 {
		return errors.New("user required")
	}
	return apply(user, model.SubscriptionDB, now.UnixMilli())
}

// effective is the single definition of a currently effective Pro grant.
func effective(subscriptions *thing.Thing[*model.Subscription], userID, now int64) *thing.CachedResult[*model.Subscription] {
	return subscriptions.Where("user_id = ? AND status = ? AND plan_level = ? AND starts_at_ms <= ? AND ends_at_ms > ?", userID, model.SubscriptionStatusActive, model.PlanLevelPro, now, now).Order("ends_at_ms DESC, id DESC")
}

func apply(user *model.User, subscriptions *thing.Thing[*model.Subscription], now int64) error {
	active, err := effective(subscriptions, user.ID, now).Fetch(0, 1)
	if err != nil {
		return err
	}
	if len(active) > 0 {
		user.PlanLevel = model.PlanLevelPro
		return nil
	}
	history, err := subscriptions.Where("user_id = ?", user.ID).Fetch(0, 1)
	if err != nil {
		return err
	}
	// Direct gifts predate subscription rows and retain their stored plan.
	if len(history) > 0 {
		user.PlanLevel = model.PlanLevelFree
	}
	return nil
}

// RefreshPlan persists the same entitlement decision inside the caller's fact transaction.
func RefreshPlan(tx *thing.TxSession, userID, now int64) error {
	users, err := thing.UseTx[*model.User](tx)
	if err != nil {
		return err
	}
	user, err := users.ByID(userID)
	if err != nil {
		return err
	}
	subscriptions, err := thing.UseTx[*model.Subscription](tx)
	if err != nil {
		return err
	}
	if err := apply(user, subscriptions, now); err != nil {
		return err
	}
	return users.Save(user)
}

func summaryFromModel(stored *model.Subscription) Summary {
	if stored == nil {
		return Summary{Status: SummaryStatusNone, PlanLevel: model.PlanLevelFree}
	}
	startsAt := time.UnixMilli(stored.StartsAtMS).UTC()
	expiresAt := time.UnixMilli(stored.EndsAtMS).UTC()
	return Summary{
		Status:    stored.Status,
		PlanLevel: stored.PlanLevel,
		StartsAt:  &startsAt,
		ExpiresAt: &expiresAt,
		Source:    stored.Source,
	}
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func unixMS(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UTC().UnixMilli()
}
