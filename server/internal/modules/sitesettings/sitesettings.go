// Package sitesettings = app/Modules/SiteSettings: satu baris site_settings; publik /robots.txt dan /llms.txt.
package sitesettings

import (
	"database/sql"
	"net/http"

	"draftcms-lite/internal/web"
)

func Register(mux *http.ServeMux, a *web.App) {
	load := func() (robots, llms sql.NullString) {
		a.DB.QueryRow(`SELECT robots_txt,llms_txt FROM site_settings ORDER BY id LIMIT 1`).Scan(&robots, &llms)
		return
	}
	a.Page(mux, "/site-settings", "sitesettings/index.html", func(r *http.Request) map[string]any {
		robots, llms := load()
		return map[string]any{"RobotsTxt": robots.String, "LlmsTxt": llms.String}
	})
	a.Admin(mux, "POST /site-settings", func(w http.ResponseWriter, r *http.Request) {
		var n int
		a.DB.QueryRow(`SELECT COUNT(*) FROM site_settings`).Scan(&n)
		if n == 0 {
			a.DB.Exec(`INSERT INTO site_settings(robots_txt,llms_txt,created_at,updated_at) VALUES(?,?,?,?)`, r.FormValue("robots_txt"), r.FormValue("llms_txt"), web.Now(), web.Now())
		} else {
			a.DB.Exec(`UPDATE site_settings SET robots_txt=?,llms_txt=?,updated_at=? ORDER BY id LIMIT 1`, r.FormValue("robots_txt"), r.FormValue("llms_txt"), web.Now())
		}
		web.Flash(w, "Saved.")
		http.Redirect(w, r, a.CP("/site-settings"), http.StatusFound)
	})
	txt := func(pick func(robots, llms sql.NullString) string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte(pick(load())))
		}
	}
	mux.HandleFunc("GET /robots.txt", txt(func(r, _ sql.NullString) string { return r.String }))
	mux.HandleFunc("GET /llms.txt", txt(func(_, l sql.NullString) string { return l.String }))
}
