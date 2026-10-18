package route_test

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	userservice "github.com/jieliapp/jieli/backend/service/user"
)

func TestAdminCreatedUsernameUsesEmailLocalPart(t *testing.T) {
	f := newAdminUsersFixture(t)
	res := f.request("POST", "/web/api/admin/users", `{"email":"  Alice@Example.Test  ","password":"Ab1!xyzw"}`, f.cookie)
	if res.Code != 200 {
		t.Fatalf("create: %d %s", res.Code, res.Body.String())
	}
	var body struct{ Data userservice.ProfileUser }
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Username != "alice" || body.Data.Email != "alice@example.test" || !body.Data.MustChangePassword {
		t.Fatalf("unexpected created account: %+v", body.Data)
	}
	if res := f.request("GET", "/web/api/users/alice/profile", "", nil); res.Code != 200 {
		t.Fatalf("new username is not publicly resolvable: %d %s", res.Code, res.Body.String())
	}
}

func TestAdminCreatedUsernameKeepsEmailLocalPartPunctuation(t *testing.T) {
	f := newAdminUsersFixture(t)
	res := f.request("POST", "/web/api/admin/users", `{"email":"jane.doe+test@example.test","password":"Ab1!xyzw"}`, f.cookie)
	if res.Code != 200 {
		t.Fatalf("create: %d %s", res.Code, res.Body.String())
	}
	var body struct{ Data userservice.ProfileUser }
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Username != "jane.doe+test" {
		t.Fatalf("mailbox prefix was changed: %q", body.Data.Username)
	}
	if res := f.request("GET", "/web/api/users/jane.doe+test/profile", "", nil); res.Code != 200 {
		t.Fatalf("mailbox username is not publicly resolvable: %d %s", res.Code, res.Body.String())
	}
}

func TestAdminCreatedUsernameUsesOpaqueNameOnActiveOrInactiveCollision(t *testing.T) {
	for _, inactive := range []bool{false, true} {
		t.Run(fmt.Sprintf("inactive-%t", inactive), func(t *testing.T) {
			f := newAdminUsersFixture(t)
			if inactive {
				res := f.request("PUT", fmt.Sprintf("/web/api/admin/users/%d/inactive", f.target.ID), `{"inactive":true}`, f.cookie)
				if res.Code != 200 {
					t.Fatal(res.Body.String())
				}
			}
			res := f.request("POST", "/web/api/admin/users", `{"email":"target@another.test","password":"Ab1!xyzw"}`, f.cookie)
			if res.Code != 200 {
				t.Fatalf("create duplicate mailbox prefix: %d %s", res.Code, res.Body.String())
			}
			var body struct{ Data userservice.ProfileUser }
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if !regexp.MustCompile(`^user[0-9a-f]{8}$`).MatchString(body.Data.Username) {
				t.Fatalf("collision did not use an eight-character hash: %q", body.Data.Username)
			}
			list := f.request("GET", "/web/api/admin/users", "", f.cookie)
			var users struct{ Data []userservice.ProfileUser }
			if err := json.Unmarshal(list.Body.Bytes(), &users); err != nil || list.Code != 200 {
				t.Fatal("administrative list unavailable")
			}
			foundOriginal, foundCreated := false, false
			for _, user := range users.Data {
				if user.ID == f.target.ID {
					foundOriginal = true
					if user.Username != "target" || user.Inactive != inactive {
						t.Fatal("creation changed the existing username or account state")
					}
				}
				if user.ID == body.Data.ID {
					foundCreated = true
					if user.Username != body.Data.Username || user.Email != "target@another.test" {
						t.Fatal("created account missing from list")
					}
				}
			}
			if !foundOriginal || !foundCreated {
				t.Fatal("creation removed or replaced an account")
			}
			login := f.request("POST", "/web/api/auth/login", `{"email":"target@another.test","password":"Ab1!xyzw"}`, nil)
			if login.Code != 200 {
				t.Fatalf("created account cannot sign in: %d %s", login.Code, login.Body.String())
			}
		})
	}
}
