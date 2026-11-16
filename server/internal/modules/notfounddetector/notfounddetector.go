// Package notfounddetector = app/Modules/NotFoundDetector (NotFoundScanner menyusul).
package notfounddetector

import (
	"net/http"

	"draftcms-lite/internal/web"
)

func Register(mux *http.ServeMux, a *web.App) {
	a.Page(mux, "/404-detector", "notfounddetector/index.html", nil)
}
