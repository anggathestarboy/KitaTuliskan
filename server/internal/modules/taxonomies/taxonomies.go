// Package taxonomies = app/Modules/Taxonomies: taxonomies + terms (child).
package taxonomies

import (
	"net/http"

	"draftcms-lite/internal/web"
)

func Register(mux *http.ServeMux, a *web.App) {
	a.Resource(mux, web.Resource{Path: "/taxonomies", Table: "taxonomies", Title: "Taxonomies", One: "Taxonomy", Perm: "configure taxonomies",
		Cols: []string{"handle", "title", "updated_at"}, Fields: []web.Field{web.F("handle", "Handle", "text"), web.F("title", "Title", "text")}, Link: "handle"})
	a.Resource(mux, web.Resource{Path: "/taxonomies/{parent}", Table: "taxonomy_terms", Title: "Terms", One: "Term", Parent: "taxonomy",
		Cols: []string{"slug", "site", "blueprint", "updated_at"}, Fields: []web.Field{web.F("slug", "Slug", "text"), web.F("site", "Site", "text")}, Extra: map[string]string{"data": "'{}'"}})
}
