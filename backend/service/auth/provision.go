package auth

import (
	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
	userservice "github.com/jieliapp/jieli/backend/service/user"
)

// ProvisionUser persists a brand-new user and issues their initial API key.
// Every sign-in channel (GitHub, dev, future providers) must create users
// through this so API key provisioning stays in a single place instead of
// being repeated — and forgotten — per channel.
func ProvisionUser(user *model.User) (common.APIKeyCredential, error) {
	return ProvisionUserWithIdentity(user, "local", user.Username)
}

// ProvisionUserWithIdentity persists a brand-new user with a username derived
// from a stable provider identity, then issues their initial API key.
func ProvisionUserWithIdentity(user *model.User, provider string, providerID string) (common.APIKeyCredential, error) {
	username, err := userservice.GenerateAvailableUsername(user.Username, provider+":"+providerID)
	if err != nil {
		return common.APIKeyCredential{}, err
	}
	user.Username = username
	if user.PlanLevel == "" {
		user.PlanLevel = model.PlanLevelFree
	}
	if err := model.UserDB.Save(user); err != nil {
		return common.APIKeyCredential{}, err
	}
	return RotateAPIKey(user)
}
