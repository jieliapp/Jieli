package auth

import (
	"errors"
	"strings"

	"github.com/jieliapp/jieli/backend/model"
)

var ErrOAuthEmailAccountExists = errors.New("oauth email account already exists")
var ErrInactiveUser = errors.New("account inactive")

func oauthEmailExists(email string) (bool, error) {
	email = canonicalOAuthEmail(email)
	if email == "" {
		return false, nil
	}
	users, err := model.UserDB.Where("email = ? OR login_email = ?", email, email).WithDeleted().Order("deleted DESC").Fetch(0, 1)
	if err != nil {
		return false, err
	}
	if len(users) > 0 && users[0].Deleted {
		return false, ErrInactiveUser
	}
	return len(users) > 0, nil
}

func canonicalOAuthEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
