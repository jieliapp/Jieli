package model

import "github.com/burugo/thing"

type Subscription struct {
	thing.BaseModel
	UserID                 int64  `json:"user_id" db:"user_id,index"`
	PlanLevel              string `json:"plan_level" db:"plan_level,index"`
	Status                 string `json:"status" db:"status,index"`
	StartsAtMS             int64  `json:"starts_at_ms" db:"starts_at_ms,index"`
	EndsAtMS               int64  `json:"ends_at_ms" db:"ends_at_ms,index"`
	Source                 string `json:"source" db:"source,index"`
	ExternalSubscriptionID string `json:"external_subscription_id" db:"external_subscription_id,index"`
	ExternalGroupID        int64  `json:"external_group_id" db:"external_group_id,index"`
	LastSyncedAtMS         int64  `json:"last_synced_at_ms" db:"last_synced_at_ms"`
	MetadataJSON           string `json:"-" db:"metadata_json,default:''"`
}

var SubscriptionDB *thing.Thing[*Subscription]

const (
	SubscriptionStatusActive    = "active"
	SubscriptionStatusExpired   = "expired"
	SubscriptionStatusCancelled = "cancelled"

	SubscriptionSourceNeo = "neo_subscription"
)

func (Subscription) TableName() string {
	return "subscriptions"
}
