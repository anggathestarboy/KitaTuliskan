// Package users = app/Modules/Users: UserManager/UserForm, RoleManager/RoleForm, UserGroupManager + PermissionRegistry.
// UI = React island (resources/ts/users); data lewat JSON API di /users/api, /roles/api, /user-groups/api.
package users

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
	"slices"
	"strings"

	"draftcms-lite/internal/modules/activitylog"
	"draftcms-lite/internal/modules/auth"
	"draftcms-lite/internal/modules/navigation"
	"draftcms-lite/internal/web"

	"golang.org/x/crypto/bcrypt"
)

var (
	forbidden = navigation.ErrForbidden
	reply     = navigation.Reply
	fail      = func(m string) error { return navigation.APIErr(m) }
)

// field = error validasi per field (422 {error, errors}); pesan pertama jadi toast, sisanya muncul di bawah input.
func field(name, msg string) error { return navigation.FieldErrs(msg, map[string]string{name: msg}) }

// log = LogsActivity / ActivityLogger::log: "{user} {action} {label}" (no-op bila activity log mati).
func log(a *web.App, r *http.Request, action, subject, id, label string) {
	if u := web.CurrentUser(r); u != nil {
		activitylog.Record(a, u.ID, action, subject, id, u.Name+" "+action+" "+label, nil)
	}
}

func perms(raw []byte) []string {
	out := []string{}
	json.Unmarshal(raw, &out)
	return out
}

// guard = $user->authorizePermission(): 403 kalau user tidak punya salah satu izin yang diminta.
func guard(w http.ResponseWriter, r *http.Request, perm string) bool {
	if !web.CurrentUser(r).Can(perm) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return false
	}
	return true
}

func in(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := navigation.Decode(w, r, v); err != nil {
		reply(w, nil, err)
		return false
	}
	return true
}

// api = a.Admin untuk endpoint JSON: izin dicek di dalam handler lewat err.
func api(a *web.App, mux *http.ServeMux, pattern string, h func(u *web.User, w http.ResponseWriter, r *http.Request)) {
	a.Admin(mux, pattern, func(w http.ResponseWriter, r *http.Request) { h(web.CurrentUser(r), w, r) })
}

func Register(mux *http.ServeMux, a *web.App) {
	registerUsers(mux, a)
	registerRoles(mux, a)
	registerGroups(mux, a)
}

// ---- Users ----

type roleRef struct {
	Handle string `json:"handle"`
	Title  string `json:"title"`
}

type userRow struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Email  string   `json:"email"`
	Banned bool     `json:"banned"`
	Roles  []string `json:"roles"`
	Groups []string `json:"groups"`
}

func registerUsers(mux *http.ServeMux, a *web.App) {
	a.Admin(mux, "GET /users", func(w http.ResponseWriter, r *http.Request) {
		if guard(w, r, "view users") {
			a.Render(w, r, "users/index.html", nil)
		}
	})
	a.Admin(mux, "GET /users/create", func(w http.ResponseWriter, r *http.Request) {
		if guard(w, r, "create users") {
			a.Render(w, r, "users/form.html", map[string]any{"ID": ""})
		}
	})
	a.Admin(mux, "GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r, "edit users") {
			return
		}
		var n int
		a.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE id=?`, r.PathValue("id")).Scan(&n)
		if n == 0 {
			http.NotFound(w, r)
			return
		}
		a.Render(w, r, "users/form.html", map[string]any{"ID": r.PathValue("id")})
	})

	// pairs: user_id → [judul] untuk tabel pivot (roles / user_groups).
	pairs := func(q string) map[string][]string {
		out := map[string][]string{}
		rows, err := a.DB.Query(q)
		if err != nil {
			return out
		}
		defer rows.Close()
		for rows.Next() {
			var id, title string
			rows.Scan(&id, &title)
			out[id] = append(out[id], title)
		}
		return out
	}

	api(a, mux, "GET /users/api/list", func(u *web.User, w http.ResponseWriter, r *http.Request) {
		if !u.Can("view users") {
			reply(w, nil, forbidden)
			return
		}
		roles := pairs(`SELECT ru.user_id, r.title FROM role_user ru JOIN roles r ON r.id=ru.role_id ORDER BY r.id`)
		groups := pairs(`SELECT gu.user_id, g.title FROM user_group_user gu JOIN user_groups g ON g.id=gu.user_group_id ORDER BY g.id`)
		rows, err := a.DB.Query(`SELECT id,name,email,banned_at FROM users ORDER BY name`)
		if err != nil {
			reply(w, nil, err)
			return
		}
		defer rows.Close()
		list := []userRow{}
		for rows.Next() {
			var x userRow
			var banned sql.NullString
			rows.Scan(&x.ID, &x.Name, &x.Email, &banned)
			x.Banned, x.Roles, x.Groups = banned.Valid, roles[x.ID], groups[x.ID]
			if x.Roles == nil {
				x.Roles = []string{}
			}
			if x.Groups == nil {
				x.Groups = []string{}
			}
			list = append(list, x)
		}
		reply(w, map[string]any{"users": list, "me": u.ID, "canCreate": u.Can("create users"), "canEdit": u.Can("edit users"),
			"canReset": u.Can("send password reset"), "canBan": u.Can("ban users"), "canDelete": u.Can("delete users")}, rows.Err())
	})

	// self-protection = UserManager: tidak bisa hapus/ban/unban diri sendiri; bulk melewati akun sendiri.
	others := func(u *web.User, ids []string) (out []string) {
		for _, id := range ids {
			if id != u.ID {
				out = append(out, id)
			}
		}
		return
	}
	name := func(id string) (n string, err error) {
		err = a.DB.QueryRow(`SELECT name FROM users WHERE id=?`, id).Scan(&n)
		return
	}
	del := func(r *http.Request, id string) error {
		n, err := name(id)
		if err != nil {
			return err
		}
		for _, q := range []string{`DELETE FROM sessions WHERE user_id=?`, `DELETE FROM role_user WHERE user_id=?`, `DELETE FROM user_group_user WHERE user_id=?`, `DELETE FROM users WHERE id=?`} {
			if _, err = a.DB.Exec(q, id); err != nil {
				return err
			}
		}
		log(a, r, "deleted", "user", id, "user '"+n+"'")
		return nil
	}
	ban := func(r *http.Request, id string, on bool) error {
		n, err := name(id)
		if err != nil {
			return err
		}
		val, action := any(nil), "unbanned"
		if on {
			val, action = web.Now(), "banned"
		}
		if _, err = a.DB.Exec(`UPDATE users SET banned_at=?, updated_at=? WHERE id=?`, val, web.Now(), id); err != nil {
			return err
		}
		log(a, r, action, "user", id, "user '"+n+"'")
		return nil
	}

	api(a, mux, "POST /users/api/delete", func(u *web.User, w http.ResponseWriter, r *http.Request) {
		var b struct{ ID string }
		if !in(w, r, &b) {
			return
		}
		switch {
		case !u.Can("delete users"):
			reply(w, nil, forbidden)
		case b.ID == u.ID:
			reply(w, nil, fail("You can't delete your own account."))
		default:
			reply(w, nil, del(r, b.ID))
		}
	})
	api(a, mux, "POST /users/api/ban", func(u *web.User, w http.ResponseWriter, r *http.Request) {
		var b struct {
			ID  string
			Ban bool
		}
		if !in(w, r, &b) {
			return
		}
		switch {
		case !u.Can("ban users"):
			reply(w, nil, forbidden)
		case b.ID == u.ID && b.Ban:
			reply(w, nil, fail("You can't ban your own account."))
		case b.ID == u.ID:
			reply(w, nil, fail("You can't unban your own account."))
		default:
			reply(w, nil, ban(r, b.ID, b.Ban))
		}
	})
	api(a, mux, "POST /users/api/bulk", func(u *web.User, w http.ResponseWriter, r *http.Request) {
		var b struct {
			Action string
			IDs    []string
		}
		if !in(w, r, &b) {
			return
		}
		perm := map[string]string{"delete": "delete users", "ban": "ban users", "unban": "ban users"}[b.Action]
		if perm == "" || !u.Can(perm) {
			reply(w, nil, forbidden)
			return
		}
		ids := others(u, b.IDs)
		var err error
		for _, id := range ids {
			switch b.Action {
			case "delete":
				err = del(r, id)
			default:
				err = ban(r, id, b.Action == "ban")
			}
			if err != nil && err != sql.ErrNoRows {
				break
			}
			err = nil
		}
		reply(w, map[string]int{"count": len(ids)}, err)
	})
	api(a, mux, "POST /users/api/reset", func(u *web.User, w http.ResponseWriter, r *http.Request) {
		var b struct{ ID string }
		if !in(w, r, &b) {
			return
		}
		if !u.Can("send password reset") {
			reply(w, nil, forbidden)
			return
		}
		var n, email string
		if err := a.DB.QueryRow(`SELECT name,email FROM users WHERE id=?`, b.ID).Scan(&n, &email); err != nil {
			reply(w, nil, err)
			return
		}
		auth.SendResetLink(a, strings.ToLower(email))
		activitylog.Record(a, u.ID, "password_reset_sent", "user", b.ID, u.Name+" sent a password reset link to '"+n+"'", nil)
		reply(w, nil, nil)
	})

	// ---- UserForm ----
	options := func(q string) []roleRef {
		out := []roleRef{}
		rows, err := a.DB.Query(q)
		if err != nil {
			return out
		}
		defer rows.Close()
		for rows.Next() {
			var x roleRef
			rows.Scan(&x.Handle, &x.Title)
			out = append(out, x)
		}
		return out
	}
	handles := func(q, id string) []string {
		out := []string{}
		rows, err := a.DB.Query(q, id)
		if err != nil {
			return out
		}
		defer rows.Close()
		for rows.Next() {
			var h string
			rows.Scan(&h)
			out = append(out, h)
		}
		return out
	}
	const userRoles = `SELECT r.handle FROM roles r JOIN role_user ru ON ru.role_id=r.id WHERE ru.user_id=? ORDER BY r.handle`
	const userGroups = `SELECT g.handle FROM user_groups g JOIN user_group_user gu ON gu.user_group_id=g.id WHERE gu.user_id=? ORDER BY g.handle`

	api(a, mux, "GET /users/api/form", func(u *web.User, w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if !u.Can(map[bool]string{true: "edit users", false: "create users"}[id != ""]) {
			reply(w, nil, forbidden)
			return
		}
		out := map[string]any{"roles": options(`SELECT handle,title FROM roles ORDER BY title`), "groups": options(`SELECT handle,title FROM user_groups ORDER BY title`),
			"roleHandles": []string{}, "groupHandles": []string{}, "canChangePassword": u.Can("change passwords")}
		if id != "" {
			var n, email string
			if err := a.DB.QueryRow(`SELECT name,email FROM users WHERE id=?`, id).Scan(&n, &email); err != nil {
				reply(w, nil, err)
				return
			}
			out["user"] = map[string]string{"id": id, "name": n, "email": email}
			out["roleHandles"], out["groupHandles"] = handles(userRoles, id), handles(userGroups, id)
		}
		reply(w, out, nil)
	})

	api(a, mux, "POST /users/api/save", func(u *web.User, w http.ResponseWriter, r *http.Request) {
		var b struct {
			ID, Name, Email, Password, PasswordMode string
			RoleHandles, GroupHandles               []string
		}
		if !in(w, r, &b) {
			return
		}
		editing := b.ID != ""
		if !u.Can(map[bool]string{true: "edit users", false: "create users"}[editing]) {
			reply(w, nil, forbidden)
			return
		}
		b.Name, b.Email = strings.TrimSpace(b.Name), strings.TrimSpace(b.Email)
		linkMode := !editing && b.PasswordMode == "reset_link"
		var taken int
		a.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE email=? AND id<>?`, b.Email, b.ID).Scan(&taken)
		_, mailErr := mail.ParseAddress(b.Email)
		switch {
		case b.Name == "":
			reply(w, nil, field("name", "The name field is required."))
			return
		case len(b.Name) > 255:
			reply(w, nil, field("name", "The name may not be greater than 255 characters."))
			return
		case b.Email == "":
			reply(w, nil, field("email", "The email field is required."))
			return
		case mailErr != nil || !strings.Contains(b.Email, "@"):
			reply(w, nil, field("email", "The email must be a valid email address."))
			return
		case len(b.Email) > 255:
			reply(w, nil, field("email", "The email may not be greater than 255 characters."))
			return
		case taken > 0:
			reply(w, nil, field("email", "The email has already been taken."))
			return
		case b.Password == "" && !editing && !linkMode:
			reply(w, nil, field("password", "The password field is required."))
			return
		case b.Password != "" && len(b.Password) < 8:
			reply(w, nil, field("password", "The password must be at least 8 characters."))
			return
		}
		for _, c := range []struct {
			table, label string
			vals         []string
		}{{"roles", "roleHandles", b.RoleHandles}, {"user_groups", "groupHandles", b.GroupHandles}} {
			for _, h := range c.vals {
				var n int
				a.DB.QueryRow("SELECT COUNT(*) FROM "+c.table+" WHERE handle=?", h).Scan(&n)
				if n == 0 {
					reply(w, nil, field(c.label, "The selected "+c.label+" is invalid."))
					return
				}
			}
		}

		rolesBefore, groupsBefore := []string{}, []string{}
		if editing {
			var n int
			if a.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE id=?`, b.ID).Scan(&n); n == 0 {
				reply(w, nil, sql.ErrNoRows)
				return
			}
			rolesBefore, groupsBefore = handles(userRoles, b.ID), handles(userGroups, b.ID)
			// Mengubah password/role/group user yang sudah ada lebih sensitif daripada nama/email: izin tambahan.
			sorted := func(s []string) []string { c := slices.Clone(s); slices.Sort(c); return slices.Compact(c) }
			switch {
			case b.Password != "" && !u.Can("change passwords"),
				!slices.Equal(sorted(b.RoleHandles), sorted(rolesBefore)) && !u.Can("assign user roles"),
				!slices.Equal(sorted(b.GroupHandles), sorted(groupsBefore)) && !u.Can("assign user groups"):
				reply(w, nil, forbidden)
				return
			}
		}

		id := b.ID
		if editing {
			set, args := `name=?, email=?, updated_at=?`, []any{b.Name, b.Email, web.Now()}
			if b.Password != "" {
				hash, _ := bcrypt.GenerateFromPassword([]byte(b.Password), 12)
				set, args = set+`, password=?`, append(args, string(hash))
			}
			if _, err := a.DB.Exec(`UPDATE users SET `+set+` WHERE id=?`, append(args, id)...); err != nil {
				reply(w, nil, err)
				return
			}
			log(a, r, "updated", "user", id, "user '"+b.Name+"'")
		} else {
			pw := b.Password
			if linkMode {
				pw = web.Token() // tak bisa dipakai login: user wajib lewat link reset
			}
			var err error
			if id, _, err = Create(a, b.Name, b.Email, pw, ""); err != nil {
				reply(w, nil, err)
				return
			}
			if linkMode {
				auth.SendResetLink(a, strings.ToLower(b.Email))
			}
			log(a, r, "created", "user", id, "user '"+b.Name+"'")
		}

		// Sync pivot tidak memicu event model, jadi dicatat eksplisit.
		sync := func(pivot, col, table string, vals []string) error {
			if _, err := a.DB.Exec("DELETE FROM "+pivot+" WHERE user_id=?", id); err != nil {
				return err
			}
			for _, h := range slices.Compact(slices.Sorted(slices.Values(vals))) {
				if _, err := a.DB.Exec("INSERT IGNORE INTO "+pivot+"("+col+",user_id) SELECT id,? FROM "+table+" WHERE handle=?", id, h); err != nil {
					return err
				}
			}
			return nil
		}
		if err := sync("role_user", "role_id", "roles", b.RoleHandles); err != nil {
			reply(w, nil, err)
			return
		}
		if err := sync("user_group_user", "user_group_id", "user_groups", b.GroupHandles); err != nil {
			reply(w, nil, err)
			return
		}
		sorted := func(s []string) []string { c := slices.Clone(s); slices.Sort(c); return slices.Compact(c) }
		diff := func(action, key, label string, before, after []string) {
			if before, after = sorted(before), sorted(after); !slices.Equal(before, after) {
				activitylog.Record(a, u.ID, action, "user", id, fmt.Sprintf("%s changed %s for user '%s'", u.Name, label, b.Name), map[string]any{key: map[string]any{"from": before, "to": after}})
			}
		}
		diff("roles_changed", "roles", "roles", rolesBefore, b.RoleHandles)
		diff("groups_changed", "groups", "groups", groupsBefore, b.GroupHandles)

		msg := "User saved."
		if linkMode {
			msg = "User created. A password-reset email was sent to " + b.Email + "."
		}
		web.Flash(w, msg)
		reply(w, map[string]any{"ok": true}, nil)
	})
}

// Create = users:create-super-admin / UserForm: buat user terverifikasi + attach role (by handle).
// Terverifikasi karena middleware 'verified' di Go sungguhan menahan akun tanpa email_verified_at (di source User tidak MustVerifyEmail).
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

// ---- Roles ----

func registerRoles(mux *http.ServeMux, a *web.App) {
	a.Admin(mux, "GET /roles", func(w http.ResponseWriter, r *http.Request) {
		if guard(w, r, "edit roles") {
			a.Render(w, r, "users/roles.html", nil)
		}
	})
	a.Admin(mux, "GET /roles/create", func(w http.ResponseWriter, r *http.Request) {
		if guard(w, r, "edit roles") {
			a.Render(w, r, "users/role-form.html", map[string]any{"Handle": ""})
		}
	})
	a.Admin(mux, "GET /roles/{handle}", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r, "edit roles") {
			return
		}
		var n int
		a.DB.QueryRow(`SELECT COUNT(*) FROM roles WHERE handle=?`, r.PathValue("handle")).Scan(&n)
		if n == 0 {
			http.NotFound(w, r)
			return
		}
		a.Render(w, r, "users/role-form.html", map[string]any{"Handle": r.PathValue("handle")})
	})

	api(a, mux, "GET /roles/api/list", func(u *web.User, w http.ResponseWriter, r *http.Request) {
		if !u.Can("edit roles") {
			reply(w, nil, forbidden)
			return
		}
		rows, err := a.DB.Query(`SELECT r.id,r.handle,r.title,r.permissions,
			(SELECT COUNT(*) FROM role_user WHERE role_id=r.id),(SELECT COUNT(*) FROM role_user_group WHERE role_id=r.id) FROM roles r ORDER BY r.title`)
		if err != nil {
			reply(w, nil, err)
			return
		}
		defer rows.Close()
		list := []map[string]any{}
		for rows.Next() {
			var id int64
			var h, t string
			var raw []byte
			var uc, gc int
			rows.Scan(&id, &h, &t, &raw, &uc, &gc)
			ps := perms(raw)
			list = append(list, map[string]any{"id": id, "handle": h, "title": t, "super": slices.Contains(ps, "super"), "permissions": len(ps), "users": uc, "groups": gc})
		}
		reply(w, map[string]any{"roles": list}, rows.Err())
	})

	api(a, mux, "GET /roles/api/form", func(u *web.User, w http.ResponseWriter, r *http.Request) {
		if !u.Can("edit roles") {
			reply(w, nil, forbidden)
			return
		}
		out := map[string]any{"groups": Groups(a)}
		if h := r.URL.Query().Get("handle"); h != "" {
			var id int64
			var title string
			var raw []byte
			if err := a.DB.QueryRow(`SELECT id,title,permissions FROM roles WHERE handle=?`, h).Scan(&id, &title, &raw); err != nil {
				reply(w, nil, err)
				return
			}
			ps := perms(raw)
			// 'super' tampil sebagai switch sendiri, bukan checkbox di tree.
			out["role"] = map[string]any{"id": id, "title": title, "handle": h, "superAdmin": slices.Contains(ps, "super"),
				"permissions": slices.DeleteFunc(ps, func(s string) bool { return s == "super" })}
		}
		reply(w, out, nil)
	})

	api(a, mux, "POST /roles/api/save", func(u *web.User, w http.ResponseWriter, r *http.Request) {
		var b struct {
			ID            int64
			Title, Handle string
			SuperAdmin    bool
			Permissions   []string
		}
		if !in(w, r, &b) {
			return
		}
		if !u.Can("edit roles") {
			reply(w, nil, forbidden)
			return
		}
		b.Title, b.Handle = strings.TrimSpace(b.Title), strings.TrimSpace(b.Handle)
		// Buang nilai yang sudah tidak ada di registry sebelum validasi (Rule::in akan menolak seluruh save).
		valid := Values(a)
		keep := []string{}
		if !b.SuperAdmin {
			for _, p := range b.Permissions {
				if slices.Contains(valid, p) && !slices.Contains(keep, p) {
					keep = append(keep, p)
				}
			}
		} else {
			keep = []string{"super"}
		}
		var taken int
		a.DB.QueryRow(`SELECT COUNT(*) FROM roles WHERE handle=? AND id<>?`, b.Handle, b.ID).Scan(&taken)
		switch {
		case b.Title == "":
			reply(w, nil, field("title", "The title field is required."))
			return
		case b.Handle == "":
			reply(w, nil, field("handle", "The handle field is required."))
			return
		case len(b.Title) > 255 || len(b.Handle) > 255:
			reply(w, nil, field("handle", "The title and handle may not be greater than 255 characters."))
			return
		case taken > 0:
			reply(w, nil, field("handle", "The handle has already been taken."))
			return
		}
		raw, _ := json.Marshal(keep)
		var err error
		if b.ID == 0 {
			_, err = a.DB.Exec(`INSERT INTO roles(handle,title,permissions,created_at,updated_at) VALUES(?,?,?,?,?)`, b.Handle, b.Title, string(raw), web.Now(), web.Now())
		} else {
			_, err = a.DB.Exec(`UPDATE roles SET handle=?, title=?, permissions=?, updated_at=? WHERE id=?`, b.Handle, b.Title, string(raw), web.Now(), b.ID)
		}
		if err == nil {
			log(a, r, map[bool]string{true: "created", false: "updated"}[b.ID == 0], "role", web.Itoa(b.ID), "role '"+b.Title+"'")
			web.Flash(w, "Role saved.")
		}
		reply(w, map[string]any{"ok": true}, err)
	})

	api(a, mux, "POST /roles/api/delete", func(u *web.User, w http.ResponseWriter, r *http.Request) {
		var b struct{ ID int64 }
		if !in(w, r, &b) {
			return
		}
		if !u.Can("edit roles") {
			reply(w, nil, forbidden)
			return
		}
		var title string
		var uc, gc int
		if err := a.DB.QueryRow(`SELECT title,(SELECT COUNT(*) FROM role_user WHERE role_id=?),(SELECT COUNT(*) FROM role_user_group WHERE role_id=?) FROM roles WHERE id=?`, b.ID, b.ID, b.ID).Scan(&title, &uc, &gc); err != nil {
			reply(w, nil, err)
			return
		}
		if uc > 0 || gc > 0 {
			var parts []string
			if uc > 0 {
				parts = append(parts, plural(uc, "user"))
			}
			if gc > 0 {
				parts = append(parts, plural(gc, "user group"))
			}
			reply(w, nil, fail(fmt.Sprintf("Cannot delete \"%s\" — it's still assigned to %s. Unassign it first.", title, strings.Join(parts, " and "))))
			return
		}
		_, err := a.DB.Exec(`DELETE FROM roles WHERE id=?`, b.ID)
		if err == nil {
			log(a, r, "deleted", "role", web.Itoa(b.ID), "role '"+title+"'")
		}
		reply(w, nil, err)
	})
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// ---- User groups ----

func registerGroups(mux *http.ServeMux, a *web.App) {
	a.Admin(mux, "GET /user-groups", func(w http.ResponseWriter, r *http.Request) {
		if guard(w, r, "edit user groups") {
			a.Render(w, r, "users/groups.html", nil)
		}
	})

	api(a, mux, "GET /user-groups/api/list", func(u *web.User, w http.ResponseWriter, r *http.Request) {
		if !u.Can("edit user groups") {
			reply(w, nil, forbidden)
			return
		}
		roles := map[int64][]roleRef{}
		if rows, err := a.DB.Query(`SELECT rg.user_group_id,r.handle,r.title FROM role_user_group rg JOIN roles r ON r.id=rg.role_id ORDER BY r.id`); err == nil {
			for rows.Next() {
				var gid int64
				var x roleRef
				rows.Scan(&gid, &x.Handle, &x.Title)
				roles[gid] = append(roles[gid], x)
			}
			rows.Close()
		}
		rows, err := a.DB.Query(`SELECT g.id,g.handle,g.title,(SELECT COUNT(*) FROM user_group_user WHERE user_group_id=g.id) FROM user_groups g ORDER BY g.title`)
		if err != nil {
			reply(w, nil, err)
			return
		}
		defer rows.Close()
		list := []map[string]any{}
		for rows.Next() {
			var id int64
			var h, t string
			var uc int
			rows.Scan(&id, &h, &t, &uc)
			rs, hs := roles[id], []string{}
			if rs == nil {
				rs = []roleRef{}
			}
			for _, x := range rs {
				hs = append(hs, x.Handle)
			}
			list = append(list, map[string]any{"id": id, "handle": h, "title": t, "roles": rs, "roleHandles": hs, "users": uc})
		}
		avail := []roleRef{}
		if rs, err := a.DB.Query(`SELECT handle,title FROM roles ORDER BY title`); err == nil {
			for rs.Next() {
				var x roleRef
				rs.Scan(&x.Handle, &x.Title)
				avail = append(avail, x)
			}
			rs.Close()
		}
		reply(w, map[string]any{"groups": list, "roles": avail}, rows.Err())
	})

	api(a, mux, "POST /user-groups/api/save", func(u *web.User, w http.ResponseWriter, r *http.Request) {
		var b struct {
			ID            int64
			Title, Handle string
			RoleHandles   []string
		}
		if !in(w, r, &b) {
			return
		}
		if !u.Can("edit user groups") {
			reply(w, nil, forbidden)
			return
		}
		b.Title, b.Handle = strings.TrimSpace(b.Title), strings.TrimSpace(b.Handle)
		var taken int
		a.DB.QueryRow(`SELECT COUNT(*) FROM user_groups WHERE handle=? AND id<>?`, b.Handle, b.ID).Scan(&taken)
		switch {
		case b.Title == "":
			reply(w, nil, field("title", "The title field is required."))
			return
		case b.Handle == "":
			reply(w, nil, field("handle", "The handle field is required."))
			return
		case len(b.Title) > 255 || len(b.Handle) > 255:
			reply(w, nil, field("handle", "The title and handle may not be greater than 255 characters."))
			return
		case taken > 0:
			reply(w, nil, field("handle", "The handle has already been taken."))
			return
		}
		for _, h := range b.RoleHandles {
			var n int
			if a.DB.QueryRow(`SELECT COUNT(*) FROM roles WHERE handle=?`, h).Scan(&n); n == 0 {
				reply(w, nil, field("roleHandles", "The selected roleHandles is invalid."))
				return
			}
		}
		id := b.ID
		if id == 0 {
			res, err := a.DB.Exec(`INSERT INTO user_groups(handle,title,created_at,updated_at) VALUES(?,?,?,?)`, b.Handle, b.Title, web.Now(), web.Now())
			if err != nil {
				reply(w, nil, err)
				return
			}
			id, _ = res.LastInsertId()
		} else if _, err := a.DB.Exec(`UPDATE user_groups SET handle=?, title=?, updated_at=? WHERE id=?`, b.Handle, b.Title, web.Now(), id); err != nil {
			reply(w, nil, err)
			return
		}
		a.DB.Exec(`DELETE FROM role_user_group WHERE user_group_id=?`, id)
		for _, h := range b.RoleHandles {
			a.DB.Exec(`INSERT IGNORE INTO role_user_group(role_id,user_group_id) SELECT id,? FROM roles WHERE handle=?`, id, h)
		}
		log(a, r, map[bool]string{true: "created", false: "updated"}[b.ID == 0], "user_group", web.Itoa(id), "user group '"+b.Title+"'")
		reply(w, map[string]any{"ok": true}, nil)
	})

	api(a, mux, "POST /user-groups/api/delete", func(u *web.User, w http.ResponseWriter, r *http.Request) {
		var b struct{ ID int64 }
		if !in(w, r, &b) {
			return
		}
		if !u.Can("edit user groups") {
			reply(w, nil, forbidden)
			return
		}
		var title string
		if err := a.DB.QueryRow(`SELECT title FROM user_groups WHERE id=?`, b.ID).Scan(&title); err != nil {
			reply(w, nil, err)
			return
		}
		for _, q := range []string{`DELETE FROM role_user_group WHERE user_group_id=?`, `DELETE FROM user_group_user WHERE user_group_id=?`, `DELETE FROM user_groups WHERE id=?`} {
			if _, err := a.DB.Exec(q, b.ID); err != nil {
				reply(w, nil, err)
				return
			}
		}
		log(a, r, "deleted", "user_group", web.Itoa(b.ID), "user group '"+title+"'")
		reply(w, nil, nil)
	})
}
