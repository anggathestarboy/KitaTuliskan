// Package navigation = app/Modules/Navigation: navigations + tree editor (/navigations/{handle}). Tree disimpan lewat Structure.
package navigation

import (
	"encoding/json"
	"net/http"

	"draftcms-lite/internal/modules/structure"
	"draftcms-lite/internal/web"
)

func Register(mux *http.ServeMux, a *web.App) {
	a.Resource(mux, web.Resource{Path: "/navigations", Table: "navigations", Title: "Navigations", One: "Navigation", Perm: "configure navigation",
		Cols: []string{"handle", "title", "updated_at"}, Fields: []web.Field{web.F("handle", "Handle", "text"), web.F("title", "Title", "text")}, Link: "handle"})
	a.Page(mux, "/navigations/{handle}", "navigation/index.html", func(r *http.Request) map[string]any {
		b, _ := json.MarshalIndent(structure.Load(a.DB, r.PathValue("handle"), "navigation"), "", "  ")
		return map[string]any{"Handle": r.PathValue("handle"), "Tree": string(b)}
	})
}

// SeedAdminSidebar = `admin:sidebar`: navigation "sidebar-admin" + tree default. Idempoten (hanya kalau belum ada).
func SeedAdminSidebar(a *web.App) {
	var n int
	a.DB.QueryRow(`SELECT COUNT(*) FROM trees WHERE handle='sidebar-admin' AND type='navigation'`).Scan(&n)
	if n > 0 {
		return
	}
	a.DB.Exec(`INSERT IGNORE INTO navigations(handle,title,created_at,updated_at) VALUES('sidebar-admin','Sidebar Admin',?,?)`, web.Now(), web.Now())
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
		tree = append(tree, structure.Item{ID: web.UUID(), Type: "internal", Title: it.t, URL: it.u, Roles: []string{"super_admin"}})
	}
	structure.Save(a.DB, "sidebar-admin", "navigation", tree)
}
