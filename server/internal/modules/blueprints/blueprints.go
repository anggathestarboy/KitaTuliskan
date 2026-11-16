// Package blueprints = app/Modules/Blueprints: blueprints + fieldsets. Field builder (HasFieldBuilder) menyusul.
// Tipe field di source: text textarea markdown bard toggle date integer number select radio array assets taxonomy author link replicator section.
package blueprints

import (
	"net/http"

	"draftcms-lite/internal/web"
)

func Register(mux *http.ServeMux, a *web.App) {
	a.Resource(mux, web.Resource{Path: "/blueprints", Table: "blueprints", Title: "Blueprints", One: "Blueprint", Perm: "configure fields",
		Cols: []string{"namespace", "handle", "updated_at"}, Fields: []web.Field{web.F("namespace", "Namespace", "text"), web.F("handle", "Handle", "text")}, Extra: map[string]string{"data": `'{"fields":[]}'`}})
	a.Resource(mux, web.Resource{Path: "/fieldsets", Table: "fieldsets", Title: "Fieldsets", One: "Fieldset", Perm: "configure fields",
		Cols: []string{"handle", "updated_at"}, Fields: []web.Field{web.F("handle", "Handle", "text")}, Extra: map[string]string{"data": `'{"fields":[]}'`}})
}
