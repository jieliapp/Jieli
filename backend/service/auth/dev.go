package auth

import (
	"github.com/jieliapp/jieli/backend/model"
)

// DevUserGitHubID is the synthetic GitHub identity used for the local
// development user provisioned by dev mode. It must never collide with a real
// GitHub numeric ID, so it uses a non-numeric sentinel value.
const DevUserGitHubID = "dev-local"

// EnsureDevUser returns the local development user, creating it on first use.
// It is only called when dev mode is explicitly enabled.
func EnsureDevUser() (*model.User, error) {
	users, err := model.UserDB.Where("github_id = ?", DevUserGitHubID).WithDeleted().Fetch(0, 1)
	if err != nil {
		return nil, err
	}
	if len(users) == 1 {
		if users[0].Deleted {
			return nil, ErrInactiveUser
		}
		return users[0], nil
	}

	user := &model.User{
		Username:        "dev",
		DisplayName:     "Dev User",
		Email:           "dev@localhost",
		GitHubID:        DevUserGitHubID,
		PreferredLocale: "",
	}
	if _, err := ProvisionUser(user); err != nil {
		return nil, err
	}
	return user, nil
}
