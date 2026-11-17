// Package navigation = app/Modules/Navigation: daftar navigation + tree editor. UI = React island (resources/ts/navigation),
// data lewat JSON API di /navigations/api. Tree disimpan lewat package structure (tabel trees).
//
// Dua mode edit (NavigationTreeEditor): blueprint (form dari field blueprint, lihat content.go) bila blueprint punya field,
// selain itu "legacy tree" (hanya navigation terproteksi, mis. sidebar-admin).
package navigation

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"slices"
	"strings"

	"draftcms-lite/internal/modules/structure"
	"draftcms-lite/internal/web"
)

const (
	SidebarHandle = "sidebar-admin"
	maxDepth      = 2    // NavigationTreeService::MAX_DEPTH
	maxItems      = 2000 // batas wajar supaya payload simpan tidak bisa dipakai membengkakkan tabel trees
)

// protected = config('cms.protected_navigations'): tidak bisa diubah handle/judulnya maupun dihapus; hanya tree-nya.
var protected = []string{SidebarHandle}

func isProtected(handle string) bool { return slices.Contains(protected, handle) }

type navRow struct {
	ID            int64  `json:"id"`
	Handle        string `json:"handle"`
	Title         string `json:"title"`
	Protected     bool   `json:"protected"`
	CanEdit       bool   `json:"canEdit"`
	CanDelete     bool   `json:"canDelete"`
	HasBlueprint  bool   `json:"hasBlueprint"` // blueprint efektif punya minimal satu field
	BlueprintName string `json:"blueprint"`
	settings      []byte
}

const navCols = `id,handle,title,settings`

type scanner interface{ Scan(...any) error }

func scanNav(r scanner) (n navRow, err error) {
	err = r.Scan(&n.ID, &n.Handle, &n.Title, &n.settings)
	return
}

type apiErr string

func (e apiErr) Error() string { return string(e) }

// fieldErrs = gagal validasi form blueprint: `all` berkunci path field ("links.0.title"), `msg` = pesan pertama untuk toast.
type fieldErrs struct {
	msg string
	all map[string]string
}

func (e fieldErrs) Error() string { return e.msg }

var errForbidden = errors.New("Forbidden")

// Izin = NavigationManager/NavigationTreeEditor: "configure navigation" berlaku untuk semua navigation,
// atau izin per-navigation "{aksi} {handle} navigation".
func canEdit(u *web.User, h string) bool {
	return u.Can("configure navigation") || u.Can("edit "+h+" navigation")
}
func canDelete(u *web.User, h string) bool {
	return u.Can("configure navigation") || u.Can("delete "+h+" navigation")
}
func canView(u *web.User, h string) bool { return u.Can("view "+h+" navigation") || canEdit(u, h) }

func reply(w http.ResponseWriter, out any, err error) {
	w.Header().Set("Content-Type", "application/json")
	var ae apiErr
	var fe fieldErrs
	switch {
	case err == nil:
		if out == nil {
			out = map[string]bool{"ok": true}
		}
	case errors.Is(err, errForbidden):
		w.WriteHeader(http.StatusForbidden)
		out = map[string]string{"error": "Forbidden"}
	case errors.Is(err, sql.ErrNoRows):
		w.WriteHeader(http.StatusNotFound)
		out = map[string]string{"error": "Not found."}
	case errors.As(err, &fe):
		w.WriteHeader(http.StatusUnprocessableEntity)
		out = map[string]any{"error": fe.msg, "errors": fe.all}
	case errors.As(err, &ae):
		w.WriteHeader(http.StatusUnprocessableEntity)
		out = map[string]string{"error": ae.Error()}
	default:
		log.Println("navigation:", err)
		w.WriteHeader(http.StatusInternalServerError)
		out = map[string]string{"error": "Something went wrong."}
	}
	json.NewEncoder(w).Encode(out)
}

func decode(w http.ResponseWriter, r *http.Request, v any) error {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(v); err != nil {
		return apiErr("Invalid request.")
	}
	return nil
}

func Register(mux *http.ServeMux, a *web.App) {
	byHandle := func(h string) (navRow, error) {
		return scanNav(a.DB.QueryRow(`SELECT `+navCols+` FROM navigations WHERE handle=?`, h))
	}

	a.Page(mux, "/navigations", "navigation/index.html", nil)
	a.Admin(mux, "GET /navigations/{handle}", func(w http.ResponseWriter, r *http.Request) {
		n, err := byHandle(r.PathValue("handle"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if !canView(web.CurrentUser(r), n.Handle) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		mode := "tree"
		if b := blueprintOf(a, n); b != nil && len(b.Fields) > 0 {
			mode = "content"
		}
		a.Render(w, r, "navigation/edit.html", map[string]any{"Handle": n.Handle, "NavTitle": n.Title, "Mode": mode})
	})

	// GET list: semua navigation (urut judul); tombol/aksi per baris sesuai izin user.
	a.Admin(mux, "GET /navigations/api/list", func(w http.ResponseWriter, r *http.Request) {
		u := web.CurrentUser(r)
		rows, err := a.DB.Query(`SELECT ` + navCols + ` FROM navigations ORDER BY title`)
		if err != nil {
			reply(w, nil, err)
			return
		}
		defer rows.Close()
		list := []navRow{}
		for rows.Next() {
			n, _ := scanNav(rows)
			n.Protected, n.CanEdit, n.CanDelete = isProtected(n.Handle), canEdit(u, n.Handle), canDelete(u, n.Handle)
			if b := blueprintOf(a, n); b != nil {
				n.BlueprintName, n.HasBlueprint = b.Handle, len(b.Fields) > 0
			}
			list = append(list, n)
		}
		reply(w, map[string]any{"navigations": list, "canCreate": u.Can("configure navigation"), "blueprints": blueprintOptions(a)}, rows.Err())
	})

	// POST save: buat/ubah navigation (judul + handle). Yang terproteksi tidak bisa diubah sama sekali.
	a.Admin(mux, "POST /navigations/api/save", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID                       int64
			Handle, Title, Blueprint string
			CreateBlueprint          bool
		}
		if err := decode(w, r, &in); err != nil {
			reply(w, nil, err)
			return
		}
		u := web.CurrentUser(r)
		var old navRow
		if in.ID != 0 {
			var err error
			if old, err = scanNav(a.DB.QueryRow(`SELECT `+navCols+` FROM navigations WHERE id=?`, in.ID)); err != nil {
				reply(w, nil, err)
				return
			}
			if !canEdit(u, old.Handle) {
				reply(w, nil, errForbidden)
				return
			}
			if isProtected(old.Handle) {
				reply(w, nil, apiErr("This navigation is protected and cannot be edited."))
				return
			}
		} else if !u.Can("configure navigation") {
			reply(w, nil, errForbidden)
			return
		}
		title, handle := strings.TrimSpace(in.Title), strings.TrimSpace(in.Handle)
		switch {
		case title == "":
			reply(w, nil, apiErr("The title field is required."))
			return
		case handle == "":
			reply(w, nil, apiErr("The handle field is required."))
			return
		case len(title) > 255 || len(handle) > 255:
			reply(w, nil, apiErr("The title and handle may not be greater than 255 characters."))
			return
		case !web.ValidHandle(handle):
			reply(w, nil, apiErr("The handle may only contain lowercase letters, numbers, dashes and underscores."))
			return
		}
		// "Create a blueprint" hanya saat buat baru (di edit tidak ada centangnya); selain itu blueprint opsional tapi harus ada.
		create := in.CreateBlueprint && in.ID == 0
		bpHandle := strings.TrimSpace(in.Blueprint)
		if create {
			bpHandle = ""
		} else if bpHandle != "" {
			var n int
			a.DB.QueryRow(`SELECT COUNT(*) FROM blueprints WHERE handle=?`, bpHandle).Scan(&n)
			if n == 0 {
				reply(w, nil, apiErr("The selected blueprint is invalid."))
				return
			}
		}
		var picked any
		if bpHandle != "" {
			picked = bpHandle
		}
		settings, _ := json.Marshal(map[string]any{"blueprint": picked})
		var taken int
		a.DB.QueryRow(`SELECT COUNT(*) FROM navigations WHERE handle=? AND id<>?`, handle, in.ID).Scan(&taken)
		if taken > 0 {
			reply(w, nil, apiErr("The handle has already been taken."))
			return
		}
		now := web.Now()
		var err error
		if in.ID == 0 {
			_, err = a.DB.Exec(`INSERT INTO navigations(handle,title,settings,created_at,updated_at) VALUES(?,?,?,?,?)`, handle, title, string(settings), now, now)
		} else {
			if _, err = a.DB.Exec(`UPDATE navigations SET handle=?, title=?, settings=?, updated_at=? WHERE id=?`, handle, title, string(settings), now, in.ID); err == nil && handle != old.Handle {
				err = structure.Rename(a.DB, old.Handle, handle, "navigation")
			}
		}
		created := false
		if err == nil && create {
			created, err = createBlueprint(a, handle)
		}
		reply(w, map[string]any{"ok": true, "blueprintCreated": created}, err)
	})

	registerContent(mux, a, byHandle)

	a.Admin(mux, "POST /navigations/api/delete", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ ID int64 }
		if err := decode(w, r, &in); err != nil {
			reply(w, nil, err)
			return
		}
		n, err := scanNav(a.DB.QueryRow(`SELECT `+navCols+` FROM navigations WHERE id=?`, in.ID))
		if err != nil {
			reply(w, nil, err)
			return
		}
		switch {
		case !canDelete(web.CurrentUser(r), n.Handle):
			reply(w, nil, errForbidden)
		case isProtected(n.Handle):
			reply(w, nil, apiErr("This navigation is protected and cannot be deleted."))
		default:
			_, err := a.DB.Exec(`DELETE FROM navigations WHERE id=?`, n.ID)
			if err == nil {
				err = structure.Delete(a.DB, n.Handle, "navigation")
			}
			reply(w, nil, err)
		}
	})

	// GET tree: tree + data pendukung form (role yang bisa melihat link, daftar page).
	a.Admin(mux, "GET /navigations/api/tree/{handle}", func(w http.ResponseWriter, r *http.Request) {
		u := web.CurrentUser(r)
		n, err := byHandle(r.PathValue("handle"))
		if err == nil && !canView(u, n.Handle) {
			err = errForbidden
		}
		if err != nil {
			reply(w, nil, err)
			return
		}
		tree := structure.Load(a.DB, n.Handle, "navigation")
		if tree == nil {
			tree = []structure.Item{}
		}
		reply(w, map[string]any{
			"navigation": map[string]any{"handle": n.Handle, "title": n.Title, "protected": isProtected(n.Handle)},
			"tree":       tree, "roles": assignableRoles(a), "pages": pages(a), "canEdit": canEdit(u, n.Handle),
		}, nil)
	})

	// POST tree: simpan seluruh tree (edit dilakukan di memori oleh UI, persis seperti tombol Save di source).
	a.Admin(mux, "POST /navigations/api/tree/{handle}", func(w http.ResponseWriter, r *http.Request) {
		n, err := byHandle(r.PathValue("handle"))
		if err != nil {
			reply(w, nil, err)
			return
		}
		if !canEdit(web.CurrentUser(r), n.Handle) {
			reply(w, nil, errForbidden)
			return
		}
		if b := blueprintOf(a, n); (b != nil && len(b.Fields) > 0) || !isProtected(n.Handle) { // editor tree hanya untuk navigation terproteksi tanpa field blueprint
			reply(w, nil, apiErr("This navigation has no blueprint yet."))
			return
		}
		var in struct{ Tree []node }
		if err := decode(w, r, &in); err != nil {
			reply(w, nil, err)
			return
		}
		items, err := sanitize(in.Tree, 1, newCtx(a))
		if err == nil {
			err = structure.Save(a.DB, n.Handle, "navigation", items)
		}
		reply(w, nil, err)
	})
}

// node = item tree dari UI. `Data` dibawa apa adanya supaya field blueprint nanti tidak hilang saat tree disimpan.
type node struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Title    string          `json:"title"`
	URL      string          `json:"url"`
	Target   string          `json:"target"`
	PageID   string          `json:"page_id"`
	Roles    []string        `json:"roles"`
	Data     json.RawMessage `json:"data"`
	Children []node          `json:"children"`
}

type sctx struct {
	roles map[string]bool
	seen  map[string]bool
	count int
}

func newCtx(a *web.App) *sctx {
	c := &sctx{roles: map[string]bool{}, seen: map[string]bool{}}
	for _, r := range assignableRoles(a) {
		c.roles[r["handle"]] = true
	}
	return c
}

// validURL: link yang boleh dipakai di sidebar. Menolak javascript:/data: dll (XSS lewat link yang dirender sebagai <a href>).
func validURL(u string) bool {
	if u == "" {
		return true
	}
	l := strings.ToLower(u)
	for _, p := range []string{"/", "#", "http://", "https://", "mailto:", "tel:"} {
		if strings.HasPrefix(l, p) {
			return true
		}
	}
	return false
}

// sanitize = NavigationTreeService::normalize + batas kedalaman: tipe valid, target valid, url hanya untuk non-page,
// page_id hanya untuk page, role hanya yang ada, id unik.
func sanitize(in []node, depth int, c *sctx) ([]structure.Item, error) {
	if depth > maxDepth && len(in) > 0 {
		return nil, apiErr("Maximum tree depth reached.")
	}
	out := make([]structure.Item, 0, len(in))
	for _, n := range in {
		if c.count++; c.count > maxItems {
			return nil, apiErr("Too many menu items.")
		}
		if n.Type != "page" && n.Type != "internal" && n.Type != "external" {
			return nil, apiErr("Invalid navigation item type.")
		}
		title, url := strings.TrimSpace(n.Title), strings.TrimSpace(n.URL)
		if len(title) > 255 || len(url) > 2048 || len(n.PageID) > 64 {
			return nil, apiErr("A menu item field is too long.")
		}
		it := structure.Item{ID: n.ID, Type: n.Type, Title: title, Target: "_self", Data: n.Data}
		if n.Target == "_blank" {
			it.Target = "_blank"
		}
		if n.Type == "page" {
			it.PageID = n.PageID
		} else {
			if !validURL(url) {
				return nil, apiErr("The URL \"" + url + "\" is not allowed. Use a path starting with /, http(s)://, mailto: or tel:.")
			}
			it.URL = url
		}
		if it.ID == "" || len(it.ID) > 64 || c.seen[it.ID] {
			it.ID = web.UUID()
		}
		c.seen[it.ID] = true
		for _, r := range n.Roles {
			if c.roles[r] && !slices.Contains(it.Roles, r) {
				it.Roles = append(it.Roles, r)
			}
		}
		var err error
		if it.Children, err = sanitize(n.Children, depth+1, c); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

// assignableRoles = availableRoles(): semua role kecuali Super Admin (sudah melihat semuanya).
func assignableRoles(a *web.App) []map[string]string {
	out := []map[string]string{}
	rows, err := a.DB.Query(`SELECT handle,title,permissions FROM roles ORDER BY title`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var h, t string
		var raw []byte
		rows.Scan(&h, &t, &raw)
		var perms []string
		json.Unmarshal(raw, &perms)
		if !slices.Contains(perms, "super") {
			out = append(out, map[string]string{"handle": h, "title": t})
		}
	}
	return out
}

// pages = Entry::orderBy('slug'): pilihan untuk item bertipe "page" (judul dari data.title, fallback slug).
func pages(a *web.App) []map[string]string {
	out := []map[string]string{}
	rows, err := a.DB.Query(`SELECT id,slug,data FROM entries ORDER BY slug`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var slug sql.NullString
		var raw []byte
		rows.Scan(&id, &slug, &raw)
		var d struct{ Title string }
		json.Unmarshal(raw, &d)
		t := d.Title
		if t == "" {
			t = slug.String
		}
		out = append(out, map[string]string{"id": id, "title": t})
	}
	return out
}

// SeedAdminSidebar = `php artisan admin:sidebar {--force}`: navigation "sidebar-admin" + tree default.
// Hanya membuat bila belum ada; force = reset tree ke default. Mengembalikan true bila tree ditulis.
func SeedAdminSidebar(a *web.App, force bool) bool {
	var n int
	a.DB.QueryRow(`SELECT COUNT(*) FROM trees WHERE handle=? AND type='navigation'`, SidebarHandle).Scan(&n)
	if n > 0 && !force {
		return false
	}
	a.DB.Exec(`INSERT IGNORE INTO navigations(handle,title,settings,created_at,updated_at) VALUES(?,'Sidebar Admin','{"blueprint":null}',?,?)`, SidebarHandle, web.Now(), web.Now())
	cp := a.CP
	items := []struct{ t, u string }{
		{"Dashboard", cp("/dashboard")}, {"Media Library", cp("/media")}, {"Collections", cp("/collections")}, {"Pages", cp("/collections/pages")},
		{"Taxonomies", cp("/taxonomies")}, {"Globals", cp("/globals")}, {"Blueprints", cp("/blueprints")}, {"Fieldsets", cp("/fieldsets")},
		{"Navigations", cp("/navigations")}, {"Forms", cp("/forms")}, {"Redirect Manager", cp("/redirect-manager")}, {"Sitemap Manager", cp("/sitemap")},
		{"Sites", cp("/sites")}, {"Site Settings", cp("/site-settings")}, {"Package Updates", cp("/package-updates")}, {"Users", cp("/users")},
		{"Roles", cp("/roles")}, {"User Groups", cp("/user-groups")}, {"Activity Log", cp("/activity-log")}, {"About Draft CMS", cp("/about")},
	}
	var tree []structure.Item
	for _, it := range items {
		tree = append(tree, structure.Item{ID: web.UUID(), Type: "internal", Title: it.t, URL: it.u, Target: "_self"})
	}
	return structure.Save(a.DB, SidebarHandle, "navigation", tree) == nil
}
