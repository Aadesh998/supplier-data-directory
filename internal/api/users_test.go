package api_test

import "testing"

func TestAdminUserManagement(t *testing.T) {
	e := newEnv(t)
	owner := e.login("owner")
	admin := e.login("admin")

	// Only owners manage users.
	admin.must("GET", "/api/v1/admin/users", nil, 403, nil)
	admin.must("POST", "/api/v1/admin/users", map[string]string{"email": "x@team.test", "password": "long-enough-pass", "role": "reviewer"}, 403, nil)

	owner.must("POST", "/api/v1/admin/users", map[string]string{"email": "short@team.test", "password": "short", "role": "reviewer"}, 422, nil)
	var created struct {
		ID   string `json:"id"`
		Role string `json:"role"`
	}
	owner.must("POST", "/api/v1/admin/users", map[string]string{"email": "new@team.test", "display_name": "New Reviewer",
		"password": "long-enough-pass", "role": "reviewer"}, 201, &created)
	owner.must("POST", "/api/v1/admin/users", map[string]string{"email": "new@team.test", "password": "long-enough-pass", "role": "reviewer"}, 409, nil)

	// The new user can sign in; deactivation ends their session and blocks login.
	nu := e.anon()
	var out struct {
		CSRFToken string `json:"csrf_token"`
	}
	nu.must("POST", "/api/v1/admin/login", map[string]string{"email": "new@team.test", "password": "long-enough-pass"}, 200, &out)
	nu.csrf = out.CSRFToken
	nu.must("GET", "/api/v1/admin/me", nil, 200, nil)

	owner.must("PATCH", "/api/v1/admin/users/"+created.ID, map[string]any{"role": "admin"}, 200, &created)
	if created.Role != "admin" {
		t.Fatalf("role = %q", created.Role)
	}
	nu.must("GET", "/api/v1/admin/me", nil, 401, nil) // role change revokes sessions
	owner.must("PATCH", "/api/v1/admin/users/"+created.ID, map[string]any{"active": false}, 200, nil)
	e.anon().must("POST", "/api/v1/admin/login", map[string]string{"email": "new@team.test", "password": "long-enough-pass"}, 401, nil)

	// Reactivate + password reset.
	owner.must("PATCH", "/api/v1/admin/users/"+created.ID, map[string]any{"active": true}, 200, nil)
	owner.must("POST", "/api/v1/admin/users/"+created.ID+"/password", map[string]string{"password": "another-long-pass"}, 200, nil)
	e.anon().must("POST", "/api/v1/admin/login", map[string]string{"email": "new@team.test", "password": "another-long-pass"}, 200, nil)

	// Owners cannot lock themselves out.
	var me struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	owner.must("GET", "/api/v1/admin/me", nil, 200, &me)
	if me.User.ID == "" {
		t.Fatal("me.user.id missing")
	}
	owner.must("PATCH", "/api/v1/admin/users/"+me.User.ID, map[string]any{"active": false}, 409, nil)
	owner.must("PATCH", "/api/v1/admin/users/"+me.User.ID, map[string]any{"role": "admin"}, 409, nil)
	owner.must("PATCH", "/api/v1/admin/users/00000000-0000-0000-0000-000000000000", map[string]any{"active": false}, 404, nil)
}
