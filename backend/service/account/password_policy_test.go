package account

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestPasswordPolicy(t *testing.T) {
	for _, password := range []string{"abc12", "中文密码六个", "abc 12", "abc\t12", strings.Repeat("a", 73)} {
		if _, err := PasswordHash(password); err == nil {
			t.Errorf("invalid password accepted")
		}
	}
	for _, password := range []string{"Ab1!xy", strings.Repeat("a", 72)} {
		hash, err := PasswordHash(password)
		if err != nil {
			t.Fatal(err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
			t.Fatal(err)
		}
	}
}
