package auth_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jieliapp/jieli/backend/model"
	"github.com/jieliapp/jieli/backend/service/auth"
)

func TestInactiveOAuthAndDevIdentityCannotProvisionReplacement(t *testing.T) {
	for _, scenario := range []string{"github", "google", "dev", "github-email", "google-email", "github-login-email", "google-login-email"} {
		t.Run(scenario, func(t *testing.T) {
			provider := strings.SplitN(scenario, "-", 2)[0]
			t.Cleanup(closeModelDB(t))
			if err := model.InitSQLite(filepath.Join(t.TempDir(), "inactive.db")); err != nil {
				t.Fatal(err)
			}
			user := &model.User{Username: "inactiveidentity", Email: "inactive@example.test", APIKeyHash: "preserved"}
			switch scenario {
			case "github":
				user.GitHubID = "13579"
			case "google":
				user.GoogleID = "inactive-google"
			case "dev":
				user.GitHubID = auth.DevUserGitHubID
			case "github-email", "google-email":
				user.Email = "new@example.test"
			case "github-login-email", "google-login-email":
				user.LoginEmail = "new@example.test"
			}
			if err := model.UserDB.Save(user); err != nil {
				t.Fatal(err)
			}
			if err := model.UserDB.SoftDelete(user); err != nil {
				t.Fatal(err)
			}
			var err error
			switch provider {
			case "github":
				_, _, err = auth.NewGitHubOAuthService(fakeGitHubOAuthClient{code: "code", user: auth.GitHubUser{ID: 13579, Login: "replacement", Email: "new@example.test"}}).Authenticate("code")
			case "google":
				_, _, err = auth.NewGoogleOAuthService(fakeGoogleOAuthClient{code: "code", user: auth.GoogleUser{ID: "inactive-google", Email: "new@example.test"}}).Authenticate("code")
			case "dev":
				_, err = auth.EnsureDevUser()
			}
			if !errors.Is(err, auth.ErrInactiveUser) {
				t.Fatal("inactive identity authenticated")
			}
			count, err := model.UserDB.Where("id > ?", 0).WithDeleted().Count()
			if err != nil || count != 1 {
				t.Fatal("login provisioned a replacement")
			}
			stored, err := model.UserDB.ByID(user.ID)
			if err != nil || !stored.Deleted || stored.APIKeyHash != "preserved" || stored.Email != user.Email {
				t.Fatal("inactive identity mutated")
			}
		})
	}
}
