// Package about = app/Modules/About.
package about

import (
	"net/http"
	"runtime"

	"draftcms-lite/internal/web"
)

const Version = "lite-0.1.0"

func Register(mux *http.ServeMux, a *web.App) {
	a.Page(mux, "/about", "about/index.html", func(r *http.Request) map[string]any {
		var v string
		a.DB.QueryRow(`SELECT VERSION()`).Scan(&v)
		return map[string]any{"Version": Version, "Go": runtime.Version(), "DB": "MySQL " + v}
	})
}
