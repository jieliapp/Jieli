package thread

import (
	"errors"

	"github.com/jieliapp/jieli/backend/model"
)

// RestoreDeletedThreads makes the user's soft-deleted, non-empty threads visible again.
func RestoreDeletedThreads(user *model.User) (int, error) {
	if user == nil || user.ID == 0 {
		return 0, errors.New("user is required")
	}
	rows, err := model.ThreadDB.
		Where("user_id = ?", user.ID).
		WithDeleted().
		All()
	if err != nil {
		return 0, err
	}
	restored := 0
	for _, row := range rows {
		if !row.Deleted || row.MessageCount <= 0 {
			continue
		}
		row.Deleted = false
		if err := model.ThreadDB.Save(row); err != nil {
			return restored, err
		}
		restored++
	}
	return restored, nil
}
