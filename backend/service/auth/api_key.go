package auth

import (
	"context"
	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
)

// apiKeyEncryptionSecret encrypts API keys at rest. It is injected once at
// startup via SetAPIKeyEncryptionSecret so that package-internal callers (such
// as EnsureDevUser) can rotate keys without threading config through.
var apiKeyEncryptionSecret string

// SetAPIKeyEncryptionSecret configures the secret used to encrypt stored API
// keys. It must be called during startup before any key is rotated.
func SetAPIKeyEncryptionSecret(secret string) {
	apiKeyEncryptionSecret = secret
}

func RotateAPIKey(user *model.User) (common.APIKeyCredential, error) {
	credential, err := common.NewAPIKey()
	if err != nil {
		return common.APIKeyCredential{}, err
	}
	encrypted, err := common.EncryptSecret(credential.Plaintext, apiKeyEncryptionSecret)
	if err != nil {
		return common.APIKeyCredential{}, err
	}
	_, err = model.UpdateUser(context.Background(), user.ID, func(current *model.User) error {
		current.APIKeyHash = credential.Hash
		current.APIKeyPrefix = credential.Prefix
		current.APIKeyEncrypted = encrypted
		return nil
	})
	if err != nil {
		return common.APIKeyCredential{}, err
	}
	user.APIKeyHash = credential.Hash
	user.APIKeyPrefix = credential.Prefix
	user.APIKeyEncrypted = encrypted
	return credential, nil
}
