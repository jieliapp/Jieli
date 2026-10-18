package auth

import (
	"context"
	"strings"

	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
	userservice "github.com/jieliapp/jieli/backend/service/user"
)

type GoogleUser struct {
	ID         string
	Email      string
	Name       string
	PictureURL string
}

type GoogleOAuthClient interface {
	UserByCode(code string) (GoogleUser, error)
}

type GoogleOAuthService struct {
	client GoogleOAuthClient
}

func NewGoogleOAuthService(client GoogleOAuthClient) GoogleOAuthService {
	return GoogleOAuthService{client: client}
}

func (service GoogleOAuthService) Authenticate(code string) (*model.User, common.APIKeyCredential, error) {
	if code == "" {
		return nil, common.APIKeyCredential{}, ErrInvalidOAuthCode
	}
	googleUser, err := service.client.UserByCode(code)
	if err != nil {
		return nil, common.APIKeyCredential{}, err
	}
	email := canonicalOAuthEmail(googleUser.Email)
	users, err := model.UserDB.Where("google_id = ?", googleUser.ID).WithDeleted().Fetch(0, 1)
	if err != nil {
		return nil, common.APIKeyCredential{}, err
	}
	if len(users) == 1 {
		if users[0].Deleted {
			return nil, common.APIKeyCredential{}, ErrInactiveUser
		}
		user, err := model.UpdateUser(context.Background(), users[0].ID, func(user *model.User) error {
			if user.Username == "" {
				username, err := userservice.GenerateAvailableUsername(usernameFromGoogleEmail(email), "google:"+googleUser.ID)
				if err != nil {
					return err
				}
				user.Username = username
			}
			if user.DisplayName == "" {
				user.DisplayName = googleUser.Name
			}
			if !user.EmailManaged {
				user.Email = email
			}
			if user.AvatarURL == "" {
				user.AvatarURL = googleUser.PictureURL
			}
			return nil
		})
		if err != nil {
			return nil, common.APIKeyCredential{}, err
		}
		return user, common.APIKeyCredential{}, nil
	}
	exists, err := oauthEmailExists(email)
	if err != nil {
		return nil, common.APIKeyCredential{}, err
	}
	if exists {
		return nil, common.APIKeyCredential{}, ErrOAuthEmailAccountExists
	}
	user := &model.User{
		Username:    usernameFromGoogleEmail(email),
		DisplayName: googleUser.Name,
		Email:       email,
		AvatarURL:   googleUser.PictureURL,
		GoogleID:    googleUser.ID,
	}
	credential, err := ProvisionUserWithIdentity(user, "google", googleUser.ID)
	if err != nil {
		return nil, common.APIKeyCredential{}, err
	}
	return user, credential, nil
}

func usernameFromGoogleEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return "google-user"
	}
	if at := strings.Index(email, "@"); at > 0 {
		return email[:at]
	}
	return email
}
