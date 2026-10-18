package subscription

import (
	"context"

	"github.com/burugo/thing"
	"github.com/jieliapp/jieli/backend/model"
)

// ExpireDue applies stored membership expiry locally, without changing history.
func (s Service) ExpireDue(ctx context.Context) SyncAllResult {
	var result SyncAllResult
	now := s.now().UnixMilli()
	for ctx.Err() == nil {
		rows, err := model.SubscriptionDB.Where("status = ? AND ends_at_ms <= ?", model.SubscriptionStatusActive, now).Fetch(0, 100)
		if err != nil {
			result.Failed++
			return result
		}
		if len(rows) == 0 {
			return result
		}
		for _, row := range rows {
			result.Scanned++
			err = thing.Transaction(ctx, func(tx *thing.TxSession) error {
				subscriptions, err := thing.UseTx[*model.Subscription](tx)
				if err != nil {
					return err
				}
				current, err := subscriptions.ByID(row.ID)
				if err != nil {
					return err
				}
				if current.Status != model.SubscriptionStatusActive || current.EndsAtMS > now {
					return nil
				}
				current.Status = model.SubscriptionStatusExpired
				if err = subscriptions.Save(current); err != nil {
					return err
				}
				return RefreshPlan(tx, row.UserID, now)
			})
			if err != nil {
				result.Failed++
				return result
			}
			result.Synced++
		}
	}
	return result
}
