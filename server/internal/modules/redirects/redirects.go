// Package redirects = app/Modules/Redirects: redirect-manager + middleware ApplyRedirects (publik).
package redirects

import (
	"net/http"
	"regexp"
	"slices"
	"strings"

	"draftcms-lite/internal/web"
)

var codes = []string{"301", "302", "307", "308", "410", "451"}

func Register(mux *http.ServeMux, a *web.App) {
	base := a.CP("/redirect-manager")
	guard := func(w http.ResponseWriter, r *http.Request) bool {
		if !web.CurrentUser(r).Can("configure redirects") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return false
		}
		return true
	}
	a.Admin(mux, "GET /redirect-manager", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		rows, _ := web.Rows(a.DB, `SELECT * FROM redirects ORDER BY source`)
		a.Render(w, r, "redirects/index.html", map[string]any{"Rows": rows, "Base": base})
	})
	// POST = RedirectManager::save(): id kosong → insert, ada → update.
	a.Admin(mux, "POST /redirect-manager", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		defer http.Redirect(w, r, base, http.StatusFound)
		f := func(k string) string { return strings.TrimSpace(r.FormValue(k)) }
		// datetime-local kirim "" saat kosong → NULL; "2026-01-01T10:00" → "2026-01-01 10:00"
		dt := func(k string) any {
			if v := f(k); v != "" {
				return strings.Replace(v, "T", " ", 1)
			}
			return nil
		}
		mt, code := f("match_type"), f("status_code")
		if f("source") == "" || f("target") == "" || !slices.Contains(codes, code) || !slices.Contains([]string{"exact", "prefix", "regex"}, mt) {
			web.Flash(w, "Source, target, a valid status code and match type are required.")
			return
		}
		keep := mt != "exact" && f("keep_path") == "1" // exact tidak punya sisa path
		vals := []any{f("source"), f("target"), code, mt, f("is_active") == "1", f("is_external") == "1", keep, dt("start_at"), dt("end_at"), web.Now()}
		var err error
		if id := f("id"); id != "" {
			_, err = a.DB.Exec(`UPDATE redirects SET source=?,target=?,status_code=?,match_type=?,is_active=?,is_external=?,keep_path=?,start_at=?,end_at=?,updated_at=? WHERE id=?`, append(vals, id)...)
		} else {
			_, err = a.DB.Exec(`INSERT INTO redirects(source,target,status_code,match_type,is_active,is_external,keep_path,start_at,end_at,updated_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, append(vals, web.Now())...)
		}
		if err != nil {
			web.Flash(w, err.Error())
		} else {
			web.Flash(w, "Redirect saved.")
		}
	})
	a.Admin(mux, "POST /redirect-manager/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		if _, err := a.DB.Exec(`DELETE FROM redirects WHERE id=?`, r.PathValue("id")); err != nil {
			web.Flash(w, err.Error())
		} else {
			web.Flash(w, "Redirect deleted.")
		}
		http.Redirect(w, r, base, http.StatusFound)
	})
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
			if hit = strings.HasPrefix(path, src); hit && web.Int64(rd["keep_path"]) == 1 {
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
			http.Redirect(w, r, target, int(web.Int64(rd["status_code"])))
			return true
		}
	}
	return false
}
