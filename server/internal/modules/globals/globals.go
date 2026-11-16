// Package globals = app/Modules/Globals: global_sets + variables (GlobalSetForm menyusul).
package globals

import (
	"net/http"

	"draftcms-lite/internal/web"
)

func Register(mux *http.ServeMux, a *web.App) {
	a.Resource(mux, web.Resource{Path: "/globals", Table: "global_sets", Title: "Globals", One: "Global set", Perm: "configure globals",
		Cols: []string{"handle", "title", "updated_at"}, Fields: []web.Field{web.F("handle", "Handle", "text"), web.F("title", "Title", "text")}, Extra: map[string]string{"settings": "'{}'"}, Link: "handle"})
	a.Resource(mux, web.Resource{Path: "/globals/{parent}", Table: "global_set_variables", Title: "Variables", One: "Variable", Parent: "handle",
		Cols: []string{"locale", "data", "updated_at"}, Fields: []web.Field{web.F("locale", "Locale", "text"), web.F("data", "Data (JSON)", "textarea")}})
}
