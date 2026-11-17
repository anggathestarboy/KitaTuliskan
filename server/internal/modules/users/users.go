// Package users = app/Modules/Users: users, roles, user groups. Permission "super" = Super Admin.
package users

import (
	"net/http"
	"strings"

	"draftcms-lite/internal/web"

	"golang.org/x/crypto/bcrypt"
)

// Permissions statis = PermissionRegistry::controlPanelGroup + usersGroup. Group per-collection/site menyusul.
var Permissions = []string{"super", "access cp", "configure sites", "configure fields", "configure form fields", "manage preferences",
	"configure redirects", "view users", "edit users", "create users", "delete users", "ban users", "change passwords", "edit user groups", "edit roles"}

func Register(mux *http.ServeMux, a *web.App) {
	a.Resource(mux, web.Resource{Path: "/users", Table: "users", Title: "Users", One: "User", Perm: "delete users", NoCreate: true,
		Cols:   []string{"name", "email", "email_verified_at", "banned_at", "created_at"},
		Fields: []web.Field{web.F("name", "Name", "text"), web.F("email", "Email", "email"), web.F("password", "Password", "password"), web.F("role", "Role handle", "text")}})
	// create user: password + role, jadi bukan Resource generik
	a.Admin(mux, "POST /users", func(w http.ResponseWriter, r *http.Request) {
		if !web.CurrentUser(r).Can("create users") {
			http.Error(w, "Forbidden", 403)
			return
		}
		pw := r.FormValue("password")
		if len(pw) < 8 {
			web.Flash(w, "Password min 8 characters.")
			http.Redirect(w, r, a.CP("/users"), http.StatusFound)
			return
		}
		id, hash, err := Create(a, r.FormValue("name"), strings.ToLower(r.FormValue("email")), pw, r.FormValue("role"))
		_ = hash
		if err != nil {
			web.Flash(w, err.Error())
		} else {
			web.Flash(w, "User "+id+" created.")
		}
		http.Redirect(w, r, a.CP("/users"), http.StatusFound)
	})
	a.Admin(mux, "POST /users/{id}/ban", func(w http.ResponseWriter, r *http.Request) {
		if !web.CurrentUser(r).Can("ban users") {
			http.Error(w, "Forbidden", 403)
			return
		}
		a.DB.Exec(`UPDATE users SET banned_at=IF(banned_at IS NULL,?,NULL) WHERE id=?`, web.Now(), r.PathValue("id"))
		a.DB.Exec(`DELETE FROM sessions WHERE user_id=?`, r.PathValue("id"))
		http.Redirect(w, r, a.CP("/users"), http.StatusFound)
	})
	a.Resource(mux, web.Resource{Path: "/roles", Table: "roles", Title: "Roles", One: "Role", Perm: "edit roles",
		Cols: []string{"handle", "title", "permissions"}, Fields: []web.Field{web.F("handle", "Handle", "text"), web.F("title", "Title", "text")},
		Extra: map[string]string{"permissions": `'["access cp"]'`}})
	a.Resource(mux, web.Resource{Path: "/user-groups", Table: "user_groups", Title: "User Groups", One: "User group", Perm: "edit user groups",
		Cols: []string{"handle", "title"}, Fields: []web.Field{web.F("handle", "Handle", "text"), web.F("title", "Title", "text")}})
}

// Create = users:create-super-admin / UserForm: buat user terverifikasi + attach role (by handle).
func Create(a *web.App, name, email, pw, role string) (string, string, error) {
	hash, _ := bcrypt.GenerateFromPassword([]byte(pw), 12)
	id := web.UUID()
	if _, err := a.DB.Exec(`INSERT INTO users(id,name,email,email_verified_at,password,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, id, name, email, web.Now(), hash, web.Now(), web.Now()); err != nil {
		return "", "", err
	}
	if role != "" {
		a.DB.Exec(`INSERT IGNORE INTO role_user(role_id,user_id) SELECT id,? FROM roles WHERE handle=?`, id, role)
	}
	return id, string(hash), nil
}

// EnsureSuperAdminRole = migration 0001_01_01_000010: role super_admin dengan permission ["super"].
func EnsureSuperAdminRole(a *web.App) {
	a.DB.Exec(`INSERT IGNORE INTO roles(handle,title,permissions,created_at,updated_at) VALUES('super_admin','Super Admin','["super"]',?,?)`, web.Now(), web.Now())
}
