// Package web: render template, session cookie (tabel sessions), flash, auth + permission middleware.
// Dipakai bersama semua modul. Perubahan di sini wajib dikabari ke dev lain.
package web

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	texttemplate "text/template"
	"time"
)

type User struct {
	ID, Name, Email string
	EmailVerifiedAt sql.NullString
	BannedAt        sql.NullString
	Preferences     sql.NullString
	Permissions     []string // gabungan roles langsung + roles lewat user_groups
}

func (u User) Initials() string {
	var s string
	for _, w := range strings.Fields(u.Name) {
		s += string(w[0])
	}
	return strings.ToUpper(s)
}

// IsSuper / Can = User::isSuper() / hasPermission() di source.
func (u User) IsSuper() bool        { return slices.Contains(u.Permissions, "super") }
func (u User) Can(perm string) bool { return u.IsSuper() || slices.Contains(u.Permissions, perm) }

type App struct {
	DB     *sql.DB
	Prefix string // config('cms.route'), contoh "/admin-panel"
	tpls   map[string]*template.Template
	dir    string
	reload bool // APP_DEBUG=true: parse ulang template tiap request, tanpa restart server
}

var sessionTTL = 120 * time.Minute

func New(db *sql.DB, prefix, templateDir string) *App {
	a := &App{DB: db, Prefix: "/" + strings.Trim(prefix, "/"), dir: templateDir, reload: os.Getenv("APP_DEBUG") == "true"}
	a.parse()
	return a
}

func (a *App) parse() {
	templateDir := a.dir
	a.tpls = map[string]*template.Template{}
	layouts, _ := filepath.Glob(filepath.Join(templateDir, "layouts", "*.html"))
	pages, _ := filepath.Glob(filepath.Join(templateDir, "*", "*.html"))
	for _, p := range pages {
		p = filepath.ToSlash(p) // Windows: glob mengembalikan "auth\login.html"
		if strings.Contains(p, "/layouts/") {
			continue
		}
		name := strings.TrimPrefix(p, filepath.ToSlash(templateDir)+"/")
		t, err := template.New(filepath.Base(p)).Funcs(funcs).ParseFiles(append(layouts, p)...)
		if err != nil {
			log.Println("template", name, err)
			continue
		}
		a.tpls[name] = t
	}
}

// CP = URL di dalam admin panel. a.CP("/collections") → "/admin-panel/collections".
func (a *App) CP(path string) string { return a.Prefix + path }

type ctxKey int

const userKey ctxKey = 0

// Render: data["User"], data["Flash"], data["Path"], data["CP"], data["Sidebar"] selalu tersedia.
func (a *App) Render(w http.ResponseWriter, r *http.Request, name string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	u := CurrentUser(r)
	data["User"] = u
	data["Path"] = r.URL.Path
	data["CP"] = a.Prefix
	data["AppName"] = "Draft CMS"
	if u != nil {
		data["Sidebar"] = a.sidebar(u)
	}
	if c, err := r.Cookie("flash"); err == nil {
		data["Flash"], _ = hex.DecodeString(c.Value)
		http.SetCookie(w, &http.Cookie{Name: "flash", MaxAge: -1, Path: "/"})
	}
	if a.reload {
		a.parse()
	}
	t, ok := a.tpls[name]
	if !ok {
		http.Error(w, "template not found: "+name, 500)
		return
	}
	if err := t.ExecuteTemplate(w, filepath.Base(name), data); err != nil {
		log.Println("render", name, err)
	}
}

type SidebarItem struct {
	Title, URL, Target string
	Type               string
	PageID             string `json:"page_id"`
	Children           []SidebarItem
	Roles              []string
}

// sidebar = resources/views/layouts/app/sidebar.blade.php: dibaca dari navigation "sidebar-admin" (tabel trees),
// disaring per role user (whitelist), super melihat semua.
func (a *App) sidebar(u *User) []SidebarItem {
	var raw sql.NullString
	a.DB.QueryRow(`SELECT tree FROM trees WHERE handle='sidebar-admin' AND type='navigation' LIMIT 1`).Scan(&raw)
	var items []SidebarItem
	json.Unmarshal([]byte(raw.String), &items)
	a.resolvePages(items)
	if u.IsSuper() {
		return items
	}
	var roles []string
	rows, _ := a.DB.Query(`SELECT r.handle FROM roles r JOIN role_user ru ON ru.role_id=r.id WHERE ru.user_id=?
		UNION SELECT r.handle FROM roles r JOIN role_user_group rg ON rg.role_id=r.id JOIN user_group_user gu ON gu.user_group_id=rg.user_group_id WHERE gu.user_id=?`, u.ID, u.ID)
	for rows != nil && rows.Next() {
		var h string
		rows.Scan(&h)
		roles = append(roles, h)
	}
	var filter func([]SidebarItem) []SidebarItem
	filter = func(in []SidebarItem) (out []SidebarItem) {
		for _, it := range in {
			if !slices.ContainsFunc(it.Roles, func(r string) bool { return slices.Contains(roles, r) }) {
				continue
			}
			had := len(it.Children) > 0
			it.Children = filter(it.Children)
			if had && len(it.Children) == 0 {
				continue
			}
			out = append(out, it)
		}
		return
	}
	return filter(items)
}

func Flash(w http.ResponseWriter, msg string) {
	http.SetCookie(w, &http.Cookie{Name: "flash", Value: hex.EncodeToString([]byte(msg)), Path: "/", HttpOnly: true})
}

func Token() string {
	b := make([]byte, 20)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// UUID v4 (stdlib saja). Source pakai uuid7 via Str::uuid7(); v4 cukup, kolom sama-sama CHAR(36).
func UUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

func Now() string { return time.Now().UTC().Format("2006-01-02 15:04:05") }

func (a *App) Login(w http.ResponseWriter, r *http.Request, userID string) {
	id := Token()
	a.DB.Exec(`INSERT INTO sessions(id,user_id,ip_address,user_agent,payload,last_activity) VALUES(?,?,?,?,'',?)`,
		id, userID, r.RemoteAddr, r.UserAgent(), time.Now().Unix())
	http.SetCookie(w, &http.Cookie{Name: "session", Value: id, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: strings.HasPrefix(os.Getenv("APP_URL"), "https")})
}

func (a *App) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("session"); err == nil {
		a.DB.Exec(`DELETE FROM sessions WHERE id=?`, c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "session", MaxAge: -1, Path: "/"})
}

func CurrentUser(r *http.Request) *User {
	u, _ := r.Context().Value(userKey).(*User)
	return u
}

// LoadUser: middleware global, isi context dengan user + permissions kalau cookie session valid.
func (a *App) LoadUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("session"); err == nil {
			var u User
			err := a.DB.QueryRow(`SELECT u.id,u.name,u.email,u.email_verified_at,u.banned_at,u.preferences FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.id=? AND s.last_activity>?`,
				c.Value, time.Now().Add(-sessionTTL).Unix()).Scan(&u.ID, &u.Name, &u.Email, &u.EmailVerifiedAt, &u.BannedAt, &u.Preferences)
			if err == nil {
				a.DB.Exec(`UPDATE sessions SET last_activity=? WHERE id=?`, time.Now().Unix(), c.Value)
				u.Permissions = a.permissions(u.ID)
				r = r.WithContext(context.WithValue(r.Context(), userKey, &u))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// permissions = User::permissions(): roles langsung + roles lewat user groups.
func (a *App) permissions(userID string) (out []string) {
	rows, err := a.DB.Query(`SELECT r.permissions FROM roles r JOIN role_user ru ON ru.role_id=r.id WHERE ru.user_id=?
		UNION SELECT r.permissions FROM roles r JOIN role_user_group rg ON rg.role_id=r.id JOIN user_group_user gu ON gu.user_group_id=rg.user_group_id WHERE gu.user_id=?`, userID, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		var perms []string
		rows.Scan(&raw)
		json.Unmarshal(raw, &perms)
		for _, p := range perms {
			if !slices.Contains(out, p) {
				out = append(out, p)
			}
		}
	}
	return
}

// Auth = middleware ['auth','verified'] (+ EnsureUserIsNotBanned dari modul Auth).
func (a *App) Auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r)
		switch {
		case u == nil:
			http.Redirect(w, r, a.CP("/login"), http.StatusFound)
		case u.BannedAt.Valid:
			a.Logout(w, r)
			Flash(w, "Your account has been banned.")
			http.Redirect(w, r, a.CP("/login"), http.StatusFound)
		case !u.EmailVerifiedAt.Valid:
			http.Redirect(w, r, a.CP("/verify-email"), http.StatusFound)
		default:
			next(w, r)
		}
	}
}

// Admin = Auth + 'permission:access cp' + prefix CMS. Semua route modul admin lewat sini.
// a.Admin(mux, "GET /collections", h) → "GET /admin-panel/collections".
func (a *App) Admin(mux *http.ServeMux, pattern string, h http.HandlerFunc) {
	method, path, _ := strings.Cut(pattern, " ")
	mux.HandleFunc(method+" "+a.CP(path), a.Auth(func(w http.ResponseWriter, r *http.Request) {
		if !CurrentUser(r).Can("access cp") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		h(w, r)
	}))
}

// Guest: hanya untuk yang belum login.
func (a *App) Guest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if CurrentUser(r) != nil {
			http.Redirect(w, r, a.CP("/dashboard"), http.StatusFound)
			return
		}
		next(w, r)
	}
}

var funcs = template.FuncMap{
	"now":       time.Now,
	"even":      func(i int) bool { return i%2 == 0 },
	"inc":       func(i int) int { return i + 1 },
	"list":      func(v ...any) []any { return v },
	"hasPrefix": strings.HasPrefix,
	"lower":     strings.ToLower,
	"vite":      viteTags,
	"split":     strings.Split,
	"safe":      func(s string) template.HTML { return template.HTML(s) },
	// attrs: html/template menolak template.HTML di posisi nama atribut (jadi "ZgotmplZ"); harus HTMLAttr.
	"attrs": func(v any) template.HTMLAttr {
		if v == nil {
			return ""
		}
		return template.HTMLAttr(fmt.Sprint(v))
	},
	// json: nilai → JSON yang sudah di-HTML-escape, aman ditaruh di dalam atribut "..." yang dibungkus safe.
	"json": func(v any) string { b, _ := json.Marshal(v); return template.HTMLEscapeString(string(b)) },
	"cond": func(c, a, b any) any {
		if ok, _ := texttemplate.IsTrue(c); ok {
			return a
		}
		return b
	},
	"dict": func(kv ...any) map[string]any {
		m := map[string]any{}
		for i := 0; i+1 < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1]
		}
		return m
	},
	"merge": func(base map[string]any, kv ...any) map[string]any {
		m := map[string]any{}
		for k, v := range base {
			m[k] = v
		}
		for i := 0; i+1 < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1]
		}
		return m
	},
}

// ConfirmPassword = middleware 'password.confirm' (3 jam, seperti Laravel).
func (a *App) ConfirmPassword(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie("pw_confirmed"); err != nil {
			http.Redirect(w, r, a.CP("/user/confirm-password?next="+r.URL.Path), http.StatusFound)
			return
		}
		next(w, r)
	}
}

// resolvePages = NavigationTreeService::resolveUrl: item bertipe "page" diarahkan ke slug entry-nya saat render,
// sehingga perubahan slug otomatis ikut tanpa menyentuh tree yang tersimpan.
func (a *App) resolvePages(items []SidebarItem) {
	for i := range items {
		if it := &items[i]; it.Type == "page" {
			var slug sql.NullString
			a.DB.QueryRow(`SELECT slug FROM entries WHERE id=?`, it.PageID).Scan(&slug)
			it.URL = ""
			if slug.Valid && slug.String != "" {
				it.URL = "/" + strings.TrimLeft(slug.String, "/")
			}
		}
		a.resolvePages(items[i].Children)
	}
}
