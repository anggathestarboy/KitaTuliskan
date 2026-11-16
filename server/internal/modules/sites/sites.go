// Package sites = app/Modules/Sites.
package sites

import (
	"net/http"

	"draftcms-lite/internal/web"
)

func Register(mux *http.ServeMux, a *web.App) {
	a.Resource(mux, web.Resource{Path: "/sites", Table: "sites", Title: "Sites", One: "Site", Perm: "configure sites", Order: "`order`, id",
		Cols:   []string{"handle", "name", "url", "locale", "lang", "order"},
		Fields: []web.Field{web.F("handle", "Handle", "text"), web.F("name", "Name", "text"), web.F("url", "URL", "text"), web.F("locale", "Locale", "text"), web.F("lang", "Lang", "text")},
		Extra:  map[string]string{"attributes": "'{}'"}})
}
