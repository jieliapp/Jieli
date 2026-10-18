package user

import (
	"context"
	"errors"

	"github.com/jieliapp/jieli/backend/model"
)

// MessageQuotaForUser returns the message quota ceiling for the given user.
// It delegates to the model layer, which owns the single quota source of truth.
func MessageQuotaForUser(user *model.User) int {
	return model.MessageQuotaForUser(user)
}

// SetAutoCleanup persists the user's auto-cleanup preference. Enabling it only
// stores the preference; no deletion happens here. Cleanup is triggered later by
// the upload path when usage crosses the threshold.
func SetAutoCleanup(user *model.User, enabled bool) (*model.User, error) {
	if user == nil || user.ID == 0 {
		return nil, errors.New("user is required")
	}
	return model.UpdateUser(context.Background(), user.ID, func(current *model.User) error {
		current.AutoCleanupEnabled = enabled
		return nil
	})
}
