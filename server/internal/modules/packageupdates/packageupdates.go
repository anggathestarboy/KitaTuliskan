// Package packageupdates = app/Modules/PackageUpdates: di lite menampilkan go.mod (PackageUpdateChecker menyusul).
package packageupdates

import (
	"net/http"
	"os"

	"draftcms-lite/internal/web"
)

func Register(mux *http.ServeMux, a *web.App) {
	a.Page(mux, "/package-updates", "packageupdates/index.html", func(r *http.Request) map[string]any {
		b, _ := os.ReadFile("go.mod")
		return map[string]any{"GoMod": string(b)}
	})
}
