// Package redirects = app/Modules/Redirects: redirect-manager + middleware ApplyRedirects (publik).
package redirects

import (
	"net/http"
	"regexp"
	"strings"

	"draftcms-lite/internal/web"
)

func Register(mux *http.ServeMux, a *web.App) {
	a.Resource(mux, web.Resource{Path: "/redirect-manager", Table: "redirects", Title: "Redirect Manager", One: "Redirect", Perm: "configure redirects",
		Cols:   []string{"source", "target", "status_code", "match_type", "is_active", "hits"},
		Fields: []web.Field{web.F("source", "Source", "text"), web.F("target", "Target", "text"), web.F("status_code", "Status code", "number"), web.F("match_type", "Match type (exact|prefix|regex)", "text")}})
}

// Apply = Http/Middleware/ApplyRedirects: dipanggil untuk request publik yang tidak cocok route lain.
// Mengembalikan true kalau sudah di-redirect.
func Apply(a *web.App, w http.ResponseWriter, r *http.Request) bool {
	rows, _ := web.Rows(a.DB, `SELECT id,source,target,status_code,match_type,is_external,keep_path FROM redirects WHERE is_active=1
		AND (start_at IS NULL OR start_at<=NOW()) AND (end_at IS NULL OR end_at>=NOW())`)
	path := r.URL.Path
	for _, rd := range rows {
		src, target := rd["source"].(string), rd["target"].(string)
		hit := false
		switch rd["match_type"] {
		case "prefix":
			if hit = strings.HasPrefix(path, src); hit && rd["keep_path"].(int64) == 1 {
				target = strings.TrimSuffix(target, "/") + strings.TrimPrefix(path, src)
			}
		case "regex":
			if re, err := regexp.Compile(src); err == nil && re.MatchString(path) {
				hit, target = true, re.ReplaceAllString(path, target)
			}
		default:
			hit = path == src
		}
		if hit {
			a.DB.Exec(`UPDATE redirects SET hits=hits+1 WHERE id=?`, rd["id"])
			http.Redirect(w, r, target, int(rd["status_code"].(int64)))
			return true
		}
	}
	return false
}
