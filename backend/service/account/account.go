package account

import (
	"context"
	"errors"
	"strings"
	"sync"

	"crypto/sha256"
	"encoding/hex"
	"github.com/burugo/thing"
	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
	"golang.org/x/crypto/bcrypt"
	"net/mail"
)

type Service struct{ EncryptionSecret string }
type Input struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	Token       string `json:"token"`
}

// accountCreation serializes email claims and initial administrator setup.
var accountCreation sync.Mutex

var ErrAlreadyInitialized = errors.New("administrator already initialized")

func Email(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 {
		return "", errors.New("valid email required")
	}
	return email, nil
}

func PasswordHash(password string) (string, error) {
	if len(password) < 6 || len(password) > 72 {
		return "", errors.New("password must contain 6 to 72 characters")
	}
	for _, char := range password {
		if char < 33 || char > 126 {
			return "", errors.New("password may contain only English letters, numbers and symbols (no spaces)")
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func (s Service) Create(ctx context.Context, input Input, admin, temporary bool) (*model.User, error) {
	if temporary && len(input.Password) < 8 {
		return nil, errors.New("temporary password must contain at least 8 characters")
	}
	email, err := Email(input.Email)
	if err != nil {
		return nil, err
	}
	hash, err := PasswordHash(input.Password)
	if err != nil {
		return nil, err
	}
	accountCreation.Lock()
	defer accountCreation.Unlock()
	var user *model.User
	err = thing.Transaction(ctx, func(tx *thing.TxSession) error {
		var err error
		user, err = s.newEmailAccount(tx, email, hash, "", admin, temporary)
		if err != nil || !admin {
			return err
		}
		settings, err := thing.UseTx[*model.SiteSettings](tx)
		if err != nil {
			return err
		}
		rows, err := settings.Where("name = ?", model.SiteSettingRegistrationEnabled).Fetch(0, 1)
		if err != nil {
			return err
		}
		registration := &model.SiteSettings{Name: model.SiteSettingRegistrationEnabled, Value: "false", ValueType: "bool"}
		if len(rows) != 0 {
			registration.ID = rows[0].ID
		}
		return settings.Save(registration)
	})
	return user, err
}

func (s Service) newEmailAccount(tx *thing.TxSession, email, hash, displayName string, admin, temporary bool) (*model.User, error) {
	users, err := thing.UseTx[*model.User](tx)
	if err != nil {
		return nil, err
	}
	if admin {
		count, err := users.Where("is_admin = ?", true).WithDeleted().Count()
		if err != nil {
			return nil, err
		}
		if count != 0 {
			return nil, ErrAlreadyInitialized
		}
	}
	count, err := users.Where("email = ? OR login_email = ?", email, email).WithDeleted().Count()
	if err != nil {
		return nil, err
	}
	if count != 0 {
		return nil, errors.New("email unavailable")
	}
	credential, err := common.NewAPIKey()
	if err != nil {
		return nil, err
	}
	encrypted, err := common.EncryptSecret(credential.Plaintext, s.EncryptionSecret)
	if err != nil {
		return nil, err
	}
	// Opaque generated usernames avoid collisions with existing OAuth identities.
	digest := sha256.Sum256([]byte(credential.Plaintext))
	username := "user" + hex.EncodeToString(digest[:4])
	if temporary {
		localPart := strings.Split(email, "@")[0]
		count, err := users.Where("username = ?", localPart).WithDeleted().Count()
		if err != nil {
			return nil, err
		}
		if count == 0 {
			username = localPart
		}
	}
	if displayName == "" {
		displayName = strings.Split(email, "@")[0]
	}
	user := &model.User{Username: username, DisplayName: displayName, Email: email, LoginEmail: email, PasswordHash: hash, IsAdmin: admin, MustChangePassword: temporary, PlanLevel: model.PlanLevelFree, APIKeyHash: credential.Hash, APIKeyPrefix: credential.Prefix, APIKeyEncrypted: encrypted}
	return user, users.Save(user)
}

// ChangePassword uses a compare-and-swap so a concurrent reset cannot be overwritten.
func ChangePassword(ctx context.Context, user *model.User, hash string) (bool, error) {
	changed := false
	err := thing.Transaction(ctx, func(tx *thing.TxSession) error {
		result, err := tx.Exec("UPDATE users SET password_hash = ?, must_change_password = ? WHERE id = ? AND password_hash = ?", hash, false, user.ID, user.PasswordHash)
		if err != nil {
			return err
		}
		tx.Touch("users")
		count, err := result.RowsAffected()
		changed = count == 1
		return err
	})
	return changed, err
}

// Authenticate checks the current stored password for an email login identity.
func Authenticate(rawEmail, password string) (*model.User, error) {
	email, err := Email(rawEmail)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}
	users, err := model.UserDB.Where("login_email = ?", email).Fetch(0, 1)
	if err != nil || len(users) != 1 || bcrypt.CompareHashAndPassword([]byte(users[0].PasswordHash), []byte(password)) != nil {
		return nil, errors.New("invalid credentials")
	}
	return users[0], nil
}
