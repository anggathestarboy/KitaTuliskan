// Package forms = app/Modules/Forms: forms + submissions + endpoint publik POST /!/forms/{handle}.
package forms

import (
	"encoding/json"
	"net/http"
	"time"

	"draftcms-lite/internal/web"
)

func Register(mux *http.ServeMux, a *web.App) {
	a.Resource(mux, web.Resource{Path: "/forms", Table: "forms", Title: "Forms", One: "Form", Perm: "configure forms",
		Cols: []string{"handle", "title", "updated_at"}, Fields: []web.Field{web.F("handle", "Handle", "text"), web.F("title", "Title", "text")}, Link: "handle"})
	a.Resource(mux, web.Resource{Path: "/forms/{parent}", Table: "form_submissions", Title: "Submissions", Parent: "form",
		Cols: []string{"id", "data", "created_at"}})

	// routes/web.php: Route::post('/!/forms/{form:handle}') — publik, dipakai Astro.
	mux.HandleFunc("POST /!/forms/{handle}", func(w http.ResponseWriter, r *http.Request) {
		var exists int
		if a.DB.QueryRow(`SELECT COUNT(*) FROM forms WHERE handle=?`, r.PathValue("handle")).Scan(&exists); exists == 0 {
			http.NotFound(w, r)
			return
		}
		r.ParseForm()
		data := map[string]string{}
		for k := range r.PostForm {
			data[k] = r.PostForm.Get(k)
		}
		b, _ := json.Marshal(data)
		// id decimal(14,4) = unix timestamp dengan 4 desimal, seperti source.
		id := float64(time.Now().UnixMicro()) / 1e6
		if _, err := a.DB.Exec(`INSERT INTO form_submissions(id,form,data,created_at,updated_at) VALUES(?,?,?,?,?)`, id, r.PathValue("handle"), string(b), web.Now(), web.Now()); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if back := r.FormValue("_redirect"); back != "" {
			http.Redirect(w, r, back, http.StatusSeeOther)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})
}
