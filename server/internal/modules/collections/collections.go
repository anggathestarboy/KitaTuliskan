// Package collections = app/Modules/Collections: collections + entries (child), preview, PageController (publik → di Astro).
package collections

import (
	"net/http"

	"draftcms-lite/internal/web"
)

func Register(mux *http.ServeMux, a *web.App) {
	a.Resource(mux, web.Resource{Path: "/collections", Table: "collections", Title: "Collections", One: "Collection", Perm: "configure collections",
		Cols: []string{"handle", "title", "updated_at"}, Fields: []web.Field{web.F("handle", "Handle", "text"), web.F("title", "Title", "text")}, Link: "handle"})
	// GET/POST /collections/{handle} = EntryManager. Edit form (EntryForm) menyusul: /collections/{handle}/entries/{id}.
	a.Resource(mux, web.Resource{Path: "/collections/{parent}", Table: "entries", Title: "Entries", One: "Entry", Parent: "collection", Order: "`order`, `date` DESC, created_at DESC",
		Cols:   []string{"slug", "status", "site", "date", "blueprint", "updated_at"},
		Fields: []web.Field{web.F("slug", "Slug", "text"), web.F("site", "Site", "text"), web.F("date", "Date", "date")},
		Extra:  map[string]string{"id": "UUID()", "data": "'{}'", "status": "'draft'", "published": "0"}})
}
