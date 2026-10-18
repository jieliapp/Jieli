package account

import (
	"context"
	"errors"

	"github.com/burugo/thing"
	thingcommon "github.com/burugo/thing/common"
	"github.com/jieliapp/jieli/backend/model"
	userservice "github.com/jieliapp/jieli/backend/service/user"
)

// Edit updates administrator-managed identity fields in one serialized claim transaction.
// Status preserves the existing route's validation and conflict response contract.
func Edit(ctx context.Context, id, actorID int64, email, username, name, hash string) (*model.User, int, error) {
	// Serialize email claims with administrator creation.
	accountCreation.Lock()
	defer accountCreation.Unlock()
	status := 500
	var updated *model.User
	err := thing.Transaction(ctx, func(tx *thing.TxSession) error {
		users, err := thing.UseTx[*model.User](tx)
		if err != nil {
			return err
		}
		current, err := users.ByID(id)
		if err != nil {
			if errors.Is(err, thingcommon.ErrNotFound) {
				status = 404
				return errors.New("user not found")
			}
			return err
		}
		if current.Deleted {
			status = 400
			return errors.New("restore user before editing")
		}
		if username != current.Username {
			username, err = userservice.ValidateUsername(username)
			if err != nil {
				status = 400
				return err
			}
		}
		for _, claim := range []struct{ field, value, message string }{
			{"email", email, "email unavailable"}, {"login_email", email, "email unavailable"}, {"username", username, "username taken"},
		} {
			count, err := users.Where(claim.field+" = ? AND id != ?", claim.value, id).WithDeleted().Count()
			if err != nil {
				return err
			}
			if count != 0 {
				status = 409
				return errors.New(claim.message)
			}
		}
		if current.Email != email {
			current.EmailManaged = true
		}
		current.Email = email
		current.Username = username
		current.DisplayName = name
		if current.LoginEmail != "" || hash != "" {
			current.LoginEmail = email
		}
		if hash != "" {
			current.PasswordHash = hash
			current.MustChangePassword = id != actorID
		}
		if err := users.Save(current); err != nil {
			return err
		}
		updated = current
		return nil
	})
	return updated, status, err
}

// SetInactive deactivates or restores a user without deleting their identity or history.
func SetInactive(ctx context.Context, id, actorID int64, inactive bool) (*model.User, int, error) {
	if inactive && id == actorID {
		return nil, 400, errors.New("cannot disable your own account")
	}
	status := 500
	var updated *model.User
	err := thing.Transaction(ctx, func(tx *thing.TxSession) error {
		users, err := thing.UseTx[*model.User](tx)
		if err != nil {
			return err
		}
		updated, err = users.ByID(id)
		if err != nil {
			if errors.Is(err, thingcommon.ErrNotFound) {
				status = 404
				return errors.New("user not found")
			}
			return err
		}
		if updated.Deleted == inactive {
			return nil
		}
		if inactive {
			return users.SoftDelete(updated)
		}
		updated.Deleted = false
		return users.Save(updated)
	})
	return updated, status, err
}
