package auth

import (
	"context"
	"errors"
	"strconv"

	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
	userservice "github.com/jieliapp/jieli/backend/service/user"
)

var ErrInvalidOAuthCode = errors.New("invalid oauth code")

type GitHubUser struct {
	ID        int64
	Login     string
	Name      string
	Email     string
	AvatarURL string
}

type GitHubOAuthClient interface {
	UserByCode(code string) (GitHubUser, error)
}

type GitHubOAuthService struct {
	client GitHubOAuthClient
}

func NewGitHubOAuthService(client GitHubOAuthClient) GitHubOAuthService {
	return GitHubOAuthService{client: client}
}

func (service GitHubOAuthService) Authenticate(code string) (*model.User, common.APIKeyCredential, error) {
	if code == "" {
		return nil, common.APIKeyCredential{}, ErrInvalidOAuthCode
	}
	githubUser, err := service.client.UserByCode(code)
	if err != nil {
		return nil, common.APIKeyCredential{}, err
	}
	githubID := strconv.FormatInt(githubUser.ID, 10)
	email := canonicalOAuthEmail(githubUser.Email)
	users, err := model.UserDB.Where("github_id = ?", githubID).WithDeleted().Fetch(0, 1)
	if err != nil {
		return nil, common.APIKeyCredential{}, err
	}
	if len(users) == 1 {
		if users[0].Deleted {
			return nil, common.APIKeyCredential{}, ErrInactiveUser
		}
		user, err := model.UpdateUser(context.Background(), users[0].ID, func(user *model.User) error {
			if user.Username == "" {
				username, err := userservice.GenerateAvailableUsername(githubUser.Login, "github:"+githubID)
				if err != nil {
					return err
				}
				user.Username = username
			}
			if user.DisplayName == "" {
				user.DisplayName = githubUser.Name
			}
			if !user.EmailManaged {
				user.Email = email
			}
			if user.AvatarURL == "" {
				user.AvatarURL = githubUser.AvatarURL
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
		Username:    githubUser.Login,
		DisplayName: githubUser.Name,
		Email:       email,
		AvatarURL:   githubUser.AvatarURL,
		GitHubID:    githubID,
	}
	credential, err := ProvisionUserWithIdentity(user, "github", githubID)
	if err != nil {
		return nil, common.APIKeyCredential{}, err
	}
	return user, credential, nil
}
