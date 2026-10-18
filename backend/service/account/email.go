package account

import (
	"context"
	"errors"
	"strconv"
	"time"

	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"github.com/burugo/thing"
	"github.com/jieliapp/jieli/backend/model"
	authservice "github.com/jieliapp/jieli/backend/service/auth"
)

func tokenDigest(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// IssueToken stores a short-lived email claim; an empty result hides unavailable identities.
func IssueToken(email, purpose, displayName, hash string) (string, error) {
	users, err := model.UserDB.Where("email = ? OR login_email = ?", email, email).WithDeleted().Fetch(0, 2)
	if err != nil {
		return "", err
	}
	if purpose == "register" && len(users) > 0 || purpose == "reset" && (len(users) != 1 || users[0].Deleted || users[0].LoginEmail == "") {
		return "", nil
	}
	var random [32]byte
	if _, err = rand.Read(random[:]); err != nil {
		return "", err
	}
	token := hex.EncodeToString(random[:])
	tokens, err := thing.Use[*model.EmailToken]()
	if err != nil {
		return "", err
	}
	stored := &model.EmailToken{TokenHash: tokenDigest(token), Email: email, Purpose: purpose, DisplayName: displayName, PasswordHash: hash, ExpiresAtMS: time.Now().Add(30 * time.Minute).UnixMilli()}
	if err = tokens.Save(stored); err != nil {
		return "", err
	}
	return token, nil
}

// CompleteToken consumes one claim and changes credentials atomically.
func (s Service) CompleteToken(ctx context.Context, input Input) error {
	accountCreation.Lock()
	defer accountCreation.Unlock()
	return thing.Transaction(ctx, func(tx *thing.TxSession) error {
		tokens, err := thing.UseTx[*model.EmailToken](tx)
		if err != nil {
			return err
		}
		rows, err := tokens.Where("token_hash = ?", tokenDigest(input.Token)).Fetch(0, 1)
		if err != nil {
			return err
		}
		if len(rows) != 1 || rows[0].Used || rows[0].ExpiresAtMS <= time.Now().UnixMilli() {
			return errors.New("invalid or expired verification token")
		}
		token := rows[0]
		claimed, err := tx.Exec("UPDATE email_tokens SET used = ? WHERE id = ? AND used = ?", true, token.ID, false)
		if err != nil {
			return err
		}
		tx.Touch("email_tokens")
		count, err := claimed.RowsAffected()
		if err != nil {
			return err
		}
		if count != 1 {
			return errors.New("verification token already used")
		}
		if token.Purpose == "register" {
			settings, err := thing.UseTx[*model.SiteSettings](tx)
			if err != nil {
				return err
			}
			rows, err := settings.Where("name = ?", model.SiteSettingRegistrationEnabled).Fetch(0, 1)
			if err != nil {
				return err
			}
			if len(rows) != 1 || rows[0].Value != strconv.FormatBool(true) {
				return errors.New("registration is disabled")
			}
			if _, err = s.newEmailAccount(tx, token.Email, token.PasswordHash, token.DisplayName, false, false); err != nil {
				return err
			}
		} else if token.Purpose == "reset" {
			hash, err := PasswordHash(input.Password)
			if err != nil {
				return err
			}
			users, err := thing.UseTx[*model.User](tx)
			if err != nil {
				return err
			}
			matches, err := users.Where("login_email = ?", token.Email).WithDeleted().Fetch(0, 1)
			if err != nil {
				return err
			}
			if len(matches) != 1 {
				return errors.New("account unavailable")
			}
			user := matches[0]
			if user.Deleted {
				return authservice.ErrInactiveUser
			}
			user.PasswordHash = hash
			user.MustChangePassword = false
			if err = users.Save(user); err != nil {
				return err
			}
		} else {
			return errors.New("invalid token purpose")
		}
		token.Used = true
		token.PasswordHash = ""
		return tokens.Save(token)
	})
}
