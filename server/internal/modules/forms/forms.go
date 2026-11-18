// Package forms = app/Modules/Forms: FormManager, FormSubmissionManager, SubmissionController (detail + export),
// FormSubmissionController (POST publik /!/forms/{handle}, validasi dari blueprint).
package forms

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"draftcms-lite/internal/web"
)

var handleRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type Settings struct {
	Blueprint        string   `json:"blueprint"`
	StoreSubmissions *bool    `json:"store_submissions"`
	RedirectURL      string   `json:"redirect_url"`
	PreviewFields    []string `json:"preview_fields"`
}

// Form = Models/Form + effectiveBlueprint().
type Form struct {
	ID       int64
	Handle   string
	Title    string
	Settings Settings
}

func (f Form) StoresSubmissions() bool {
	return f.Settings.StoreSubmissions == nil || *f.Settings.StoreSubmissions
}

func Register(mux *http.ServeMux, a *web.App) {
	guard := func(w http.ResponseWriter, r *http.Request) bool {
		if !web.CurrentUser(r).Can("configure forms") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return false
		}
		return true
	}

	// ---- FormManager ----
	// renderList: daftar + modal. errs/form terisi saat POST gagal validasi → modal dibuka lagi dengan nilai yang diketik (= :show="$errors->isNotEmpty()").
	renderList := func(w http.ResponseWriter, r *http.Request, errs map[string]string, form map[string]any) {
		rows := []map[string]any{}
		for _, f := range allForms(a) {
			var n int
			a.DB.QueryRow(`SELECT COUNT(*) FROM form_submissions WHERE form=?`, f.Handle).Scan(&n)
			bp := effectiveBlueprint(a, f)
			rows = append(rows, map[string]any{"id": f.ID, "handle": f.Handle, "title": f.Title, "blueprint": or(bp, "—"),
				"blueprintPick": or(f.Settings.Blueprint, bp), "createBlueprint": false, "storeSubmissions": f.StoresSubmissions(), "redirectUrl": f.Settings.RedirectURL, "previewFields": nonNil(f.Settings.PreviewFields), "submissions": n})
		}
		bps, _ := web.Rows(a.DB, `SELECT namespace,handle,data FROM blueprints ORDER BY namespace,handle`)
		blueprints := []map[string]any{}
		fieldsByBlueprint := map[string][]map[string]string{} // untuk checkbox "Preview columns"
		for _, b := range bps {
			h, ns := str(b["handle"]), str(b["namespace"])
			blueprints = append(blueprints, map[string]any{"handle": h, "label": or(strings.TrimPrefix(ns+"."+h, "."), h)})
			fs := []map[string]string{}
			for _, f := range fieldsOf(str(b["data"])) {
				fs = append(fs, map[string]string{"handle": str(f["handle"]), "display": or(str(f["display"]), str(f["handle"]))})
			}
			fieldsByBlueprint[h] = fs
		}
		a.Render(w, r, "forms/index.html", map[string]any{"Rows": rows, "Blueprints": blueprints, "FieldsByBlueprint": fieldsByBlueprint, "Errors": errs, "Form": form})
	}
	a.Admin(mux, "GET /forms", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		renderList(w, r, map[string]string{}, nil)
	})
	a.Admin(mux, "POST /forms", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		f := func(k string) string { return strings.TrimSpace(r.FormValue(k)) }
		id, _ := strconv.ParseInt(f("id"), 10, 64)
		createBlueprint := id == 0 && f("create_blueprint") == "1"
		errs := map[string]string{}
		if !handleRe.MatchString(f("handle")) || len(f("handle")) > 255 {
			errs["handle"] = "The handle must be lowercase letters, numbers and underscores, starting with a letter."
		} else if n := 0; a.DB.QueryRow(`SELECT COUNT(*) FROM forms WHERE handle=? AND id<>?`, f("handle"), id).Scan(&n) == nil && n > 0 {
			errs["handle"] = "The handle has already been taken."
		}
		if f("title") == "" || len(f("title")) > 255 {
			errs["title"] = "The title field is required."
		}
		bp := ""
		if !createBlueprint {
			bp = f("blueprint")
			if n := 0; bp != "" && (a.DB.QueryRow(`SELECT COUNT(*) FROM blueprints WHERE handle=?`, bp).Scan(&n) != nil || n == 0) {
				errs["blueprint"] = "The selected blueprint is invalid."
			}
		}
		if u := f("redirect_url"); u != "" {
			if p, err := url.Parse(u); err != nil || p.Scheme == "" || p.Host == "" || len(u) > 2048 {
				errs["redirect_url"] = "The redirect URL must be a valid URL."
			}
		}
		if len(errs) > 0 {
			renderList(w, r, errs, map[string]any{"id": id, "handle": f("handle"), "title": f("title"), "blueprintPick": f("blueprint"), "createBlueprint": createBlueprint,
				"storeSubmissions": f("store_submissions") == "1", "redirectUrl": f("redirect_url"), "previewFields": nonNil(r.Form["preview_fields"])})
			return
		}
		store := f("store_submissions") == "1"
		settings, _ := json.Marshal(map[string]any{"blueprint": nullable(bp), "store_submissions": store, "redirect_url": nullable(f("redirect_url")), "preview_fields": nonNil(r.Form["preview_fields"])})
		if id == 0 {
			a.DB.Exec(`INSERT INTO forms(handle,title,settings,created_at,updated_at) VALUES(?,?,?,?,?)`, f("handle"), f("title"), string(settings), web.Now(), web.Now())
		} else {
			a.DB.Exec(`UPDATE forms SET handle=?,title=?,settings=?,updated_at=? WHERE id=?`, f("handle"), f("title"), string(settings), web.Now(), id)
		}
		if createBlueprint { // Blueprint::firstOrCreate(namespace forms.{handle}, handle)
			var n int
			a.DB.QueryRow(`SELECT COUNT(*) FROM blueprints WHERE namespace=? AND handle=?`, "forms."+f("handle"), f("handle")).Scan(&n)
			if n == 0 {
				a.DB.Exec(`INSERT INTO blueprints(namespace,handle,data,created_at,updated_at) VALUES(?,?,?,?,?)`, "forms."+f("handle"), f("handle"), `{"tabs":[{"handle":"main","display":"Main","sections":[]}],"fields":[]}`, web.Now(), web.Now())
				web.Flash(w, "Form saved — now add fields to its blueprint below.")
				http.Redirect(w, r, a.CP("/blueprints"), http.StatusFound)
				return
			}
		}
		web.Flash(w, "Form saved.")
		http.Redirect(w, r, a.CP("/forms"), http.StatusFound)
	})
	a.Admin(mux, "POST /forms/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		var title, handle string
		if a.DB.QueryRow(`SELECT title,handle FROM forms WHERE id=?`, r.PathValue("id")).Scan(&title, &handle) == nil {
			var n int
			a.DB.QueryRow(`SELECT COUNT(*) FROM form_submissions WHERE form=?`, handle).Scan(&n)
			if n > 0 {
				web.Flash(w, fmt.Sprintf(`Cannot delete "%s" — it still has %d submissions. Delete those first.`, title, n))
			} else {
				a.DB.Exec(`DELETE FROM forms WHERE id=?`, r.PathValue("id"))
				web.Flash(w, "Form deleted.")
			}
		}
		http.Redirect(w, r, a.CP("/forms"), http.StatusFound)
	})

	// ---- FormSubmissionManager ----
	a.Admin(mux, "GET /forms/{handle}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, a.CP("/forms/"+r.PathValue("handle")+"/submissions"), http.StatusFound)
	})
	a.Admin(mux, "GET /forms/{handle}/submissions", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		form, ok := findForm(a, r.PathValue("handle"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		fields := formFields(a, form)
		cols := fields
		if len(form.Settings.PreviewFields) > 0 {
			cols = nil
			for _, f := range fields {
				for _, h := range form.Settings.PreviewFields {
					if str(f["handle"]) == h {
						cols = append(cols, f)
					}
				}
			}
		}
		rows, _ := web.Rows(a.DB, `SELECT id,data,created_at FROM form_submissions WHERE form=? ORDER BY id DESC`, form.Handle)
		subs := []map[string]any{}
		for _, s := range rows {
			var data map[string]any
			json.Unmarshal([]byte(str(s["data"])), &data)
			cells := []any{}
			for _, f := range cols {
				cells = append(cells, cellValue(a, f, data[str(f["handle"])], false))
			}
			subs = append(subs, map[string]any{"id": str(s["id"]), "cells": cells, "created": fmtDate(str(s["created_at"]), "2006-01-02 | 15:04")})
		}
		a.Render(w, r, "forms/submissions.html", map[string]any{"Form": form, "Cols": cols, "Rows": subs})
	})
	a.Admin(mux, "POST /forms/{handle}/submissions/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		a.DB.Exec(`DELETE FROM form_submissions WHERE id=? AND form=?`, r.PathValue("id"), r.PathValue("handle"))
		web.Flash(w, "Submission deleted.")
		http.Redirect(w, r, a.CP("/forms/"+r.PathValue("handle")+"/submissions"), http.StatusFound)
	})

	// ---- SubmissionController ----
	detail := func(w http.ResponseWriter, r *http.Request) (Form, map[string]any, []map[string]any, bool) {
		if !guard(w, r) {
			return Form{}, nil, nil, false
		}
		form, ok := findForm(a, r.PathValue("handle"))
		rows, _ := web.Rows(a.DB, `SELECT id,data,created_at FROM form_submissions WHERE id=? AND form=?`, r.PathValue("id"), r.PathValue("handle"))
		if !ok || len(rows) == 0 {
			http.NotFound(w, r)
			return Form{}, nil, nil, false
		}
		return form, rows[0], formFields(a, form), true
	}
	a.Admin(mux, "GET /forms/{handle}/submissions/{id}", func(w http.ResponseWriter, r *http.Request) {
		form, sub, fields, ok := detail(w, r)
		if !ok {
			return
		}
		var data map[string]any
		json.Unmarshal([]byte(str(sub["data"])), &data)
		values := []map[string]any{}
		for _, f := range fields {
			values = append(values, map[string]any{"label": or(str(f["display"]), str(f["handle"])), "html": cellValue(a, f, data[str(f["handle"])], true)})
		}
		a.Render(w, r, "forms/submission.html", map[string]any{"Form": form, "ID": str(sub["id"]), "Created": fmtDate(str(sub["created_at"]), "2006-01-02 15:04"), "Values": values})
	})
	a.Admin(mux, "GET /forms/{handle}/submissions/{id}/export/txt", func(w http.ResponseWriter, r *http.Request) {
		_, sub, fields, ok := detail(w, r)
		if !ok {
			return
		}
		var data map[string]any
		json.Unmarshal([]byte(str(sub["data"])), &data)
		lines := []string{}
		for _, f := range fields {
			v := data[str(f["handle"])]
			s := plain(v)
			if str(f["type"]) == "bard" {
				s = ToText(plain(v))
			}
			lines = append(lines, or(str(f["display"]), str(f["handle"]))+": "+s)
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="submission-`+str(sub["id"])+`.txt"`)
		w.Write([]byte(strings.Join(lines, "\n")))
	})
	a.Admin(mux, "GET /forms/{handle}/submissions/{id}/export/json", func(w http.ResponseWriter, r *http.Request) {
		_, sub, _, ok := detail(w, r)
		if !ok {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="submission-`+str(sub["id"])+`.json"`)
		w.Write([]byte(or(str(sub["data"]), "{}")))
	})

	// ---- FormSubmissionController: routes/web.php Route::post('/!/forms/{form:handle}') — publik, dipakai Astro ----
	mux.HandleFunc("POST /!/forms/{handle}", func(w http.ResponseWriter, r *http.Request) {
		form, ok := findForm(a, r.PathValue("handle"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		r.ParseForm()
		fields := formFields(a, form)
		data, errs := validateSubmission(fields, r.PostForm)
		if len(errs) > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]any{"message": "The given data was invalid.", "errors": errs})
			return
		}
		if form.StoresSubmissions() {
			b, _ := json.Marshal(data)
			// id decimal(14,4) = microtime seperti source (ponytail: tabrakan mungkin dalam 0.1ms yang sama; ganti sequence/uuid kalau volume tinggi).
			id := fmt.Sprintf("%.4f", float64(time.Now().UnixMicro())/1e6)
			if _, err := a.DB.Exec(`INSERT INTO form_submissions(id,form,data,created_at,updated_at) VALUES(?,?,?,?,?)`, id, form.Handle, string(b), web.Now(), web.Now()); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		back := or(form.Settings.RedirectURL, or(r.FormValue("_redirect"), r.Referer()))
		if back == "" {
			w.WriteHeader(http.StatusCreated)
			return
		}
		http.Redirect(w, r, back, http.StatusSeeOther)
	})
}

// validateSubmission = rulesFor() + EvaluatesFieldConditions: input "data[handle]" (seperti source) atau "handle" polos.
func validateSubmission(fields []map[string]any, post url.Values) (map[string]any, map[string]string) {
	get := func(h string) []string {
		for _, k := range []string{"data[" + h + "]", "data[" + h + "][]", h, h + "[]"} {
			if v, ok := post[k]; ok {
				return v
			}
		}
		return nil
	}
	raw := map[string]any{}
	for _, f := range fields {
		h := str(f["handle"])
		v := get(h)
		switch {
		case str(f["type"]) == "checkbox":
			raw[h] = v
		case len(v) > 0:
			raw[h] = v[0]
		}
	}
	data, errs := map[string]any{}, map[string]string{}
	for _, f := range fields {
		h, label := str(f["handle"]), or(str(f["display"]), str(f["handle"]))
		if !fieldVisible(f, raw) {
			continue
		}
		v := raw[h]
		empty := v == nil || v == "" || (str(f["type"]) == "checkbox" && len(v.([]string)) == 0)
		if empty {
			if req, _ := f["required"].(bool); req {
				errs[h] = "The " + label + " field is required."
			}
			continue
		}
		opts := strs(f["options"])
		switch str(f["type"]) {
		case "number":
			if _, err := strconv.ParseFloat(v.(string), 64); err != nil {
				errs[h] = "The " + label + " field must be a number."
			}
		case "toggle":
			if s := v.(string); s != "0" && s != "1" && s != "true" && s != "false" {
				errs[h] = "The " + label + " field must be true or false."
			}
		case "date":
			if _, err := time.Parse("2006-01-02", v.(string)); err != nil {
				errs[h] = "The " + label + " field must be a valid date."
			}
		case "select", "radio":
			if !contains(opts, v.(string)) {
				errs[h] = "The selected " + label + " is invalid."
			}
		case "checkbox":
			for _, s := range v.([]string) {
				if !contains(opts, s) {
					errs[h] = "The selected " + label + " is invalid."
				}
			}
		}
		if _, bad := errs[h]; !bad {
			data[h] = v
		}
	}
	return data, errs
}

// fieldVisible = EvaluatesFieldConditions::fieldIsVisible().
func fieldVisible(f map[string]any, scope map[string]any) bool {
	c, _ := f["conditions"].(map[string]any)
	mode := str(c["mode"])
	if c == nil || mode == "" || mode == "always" {
		return true
	}
	actual, expected := scope[str(c["field"])], str(c["value"])
	list, isList := actual.([]string)
	equals := func() bool {
		if isList {
			return contains(list, expected)
		}
		return plain(actual) == expected
	}
	var m bool
	switch str(c["operator"]) {
	case "not":
		m = !equals()
	case "strict":
		m = actual == expected
	case "contains":
		if isList {
			m = contains(list, expected)
		} else {
			m = strings.Contains(plain(actual), expected)
		}
	case "contains_any":
		for _, e := range strings.Split(expected, ",") {
			if e = strings.TrimSpace(e); e != "" && isList && contains(list, e) {
				m = true
			}
		}
	default:
		m = equals()
	}
	if mode == "show" {
		return m
	}
	return !m
}

// ---- helpers ----

func str(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	}
	return fmt.Sprint(v)
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func strs(v any) []string {
	out := []string{}
	if l, ok := v.([]any); ok {
		for _, x := range l {
			out = append(out, str(x))
		}
	}
	return out
}

// plain = formatValue(): array → "a, b".
func plain(v any) string {
	switch t := v.(type) {
	case []string:
		return strings.Join(t, ", ")
	case []any:
		return strings.Join(strs(t), ", ")
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	return str(v)
}

func fmtDate(s, layout string) string {
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		return s
	}
	return t.Format(layout)
}

func parseForm(row map[string]any) Form {
	f := Form{ID: web.Int64(row["id"]), Handle: str(row["handle"]), Title: str(row["title"])}
	json.Unmarshal([]byte(str(row["settings"])), &f.Settings)
	return f
}

func allForms(a *web.App) []Form {
	rows, _ := web.Rows(a.DB, `SELECT * FROM forms ORDER BY title`)
	out := []Form{}
	for _, r := range rows {
		out = append(out, parseForm(r))
	}
	return out
}

func findForm(a *web.App, handle string) (Form, bool) {
	rows, _ := web.Rows(a.DB, `SELECT * FROM forms WHERE handle=?`, handle)
	if len(rows) == 0 {
		return Form{}, false
	}
	return parseForm(rows[0]), true
}

// effectiveBlueprint = Form::effectiveBlueprint(): settings.blueprint, fallback namespace "forms.{handle}". Mengembalikan handle ("" = tidak ada).
func effectiveBlueprint(a *web.App, f Form) string {
	var h string
	if f.Settings.Blueprint != "" {
		a.DB.QueryRow(`SELECT handle FROM blueprints WHERE handle=? LIMIT 1`, f.Settings.Blueprint).Scan(&h)
		return h
	}
	a.DB.QueryRow(`SELECT handle FROM blueprints WHERE namespace=? LIMIT 1`, "forms."+f.Handle).Scan(&h)
	return h
}

func fieldsOf(data string) []map[string]any {
	var d struct {
		Fields []map[string]any `json:"fields"`
	}
	json.Unmarshal([]byte(data), &d)
	return d.Fields
}

// formFields = Form::fields().
func formFields(a *web.App, f Form) []map[string]any {
	var data string
	if f.Settings.Blueprint != "" {
		a.DB.QueryRow(`SELECT data FROM blueprints WHERE handle=? LIMIT 1`, f.Settings.Blueprint).Scan(&data)
	} else {
		a.DB.QueryRow(`SELECT data FROM blueprints WHERE namespace=? LIMIT 1`, "forms."+f.Handle).Scan(&data)
	}
	return fieldsOf(data)
}

// cellValue = partials/submission-value.blade.php: HTML aman untuk sel tabel (full=false) atau halaman detail (full=true).
func cellValue(a *web.App, f map[string]any, v any, full bool) string {
	esc := func(s string) string {
		return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;").Replace(s)
	}
	switch str(f["type"]) {
	case "assets":
		ids := []any{}
		if l, ok := v.([]any); ok {
			ids = l
		} else if v != nil && v != "" {
			ids = []any{v}
		}
		links := []string{}
		for _, id := range ids {
			rows, _ := web.Rows(a.DB, `SELECT m.container,m.path,m.filename,m.meta,c.disk FROM assets_meta m JOIN asset_containers c ON c.handle=m.container WHERE m.id=?`, plain(id))
			if len(rows) == 0 {
				continue
			}
			m := rows[0]
			var meta struct {
				Data struct{ Title string } `json:"data"`
			}
			json.Unmarshal([]byte(str(m["meta"])), &meta)
			u := "/storage/" + str(m["container"]) + "/" + str(m["path"])
			if str(m["disk"]) != "public" {
				u = a.CP("/media/file/" + str(m["container"]) + "/" + str(m["path"]))
			}
			links = append(links, `<a href="`+esc(u)+`" target="_blank" rel="noopener" class="underline decoration-zinc-300 underline-offset-4 hover:decoration-current">`+esc(or(meta.Data.Title, str(m["filename"])))+`</a>`)
		}
		if len(links) == 0 {
			return "—"
		}
		return strings.Join(links, ", ")
	case "bard":
		if full {
			return `<div class="[&_a]:underline [&_blockquote]:border-l-2 [&_blockquote]:pl-3 [&_h1]:text-xl [&_h1]:font-bold [&_h2]:text-lg [&_h2]:font-semibold [&_h3]:font-semibold [&_img]:my-2 [&_img]:max-w-full [&_img]:rounded-lg [&_ol]:list-decimal [&_ol]:pl-5 [&_ul]:list-disc [&_ul]:pl-5">` + Sanitize(plain(v)) + `</div>`
		}
		return esc(Excerpt(plain(v)))
	}
	if v == nil || plain(v) == "" {
		return "—"
	}
	return esc(plain(v))
}
