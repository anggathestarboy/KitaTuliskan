// Package blueprints = app/Modules/Blueprints: BlueprintManager/BlueprintForm + FieldsetManager/FieldsetForm.
// Logika pohon field (HasFieldBuilder) ada di fields.go; editornya di templates/layouts/field-editor.html (Alpine).
package blueprints

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"draftcms-lite/internal/modules/activitylog"
	"draftcms-lite/internal/web"
)

type Tab struct {
	Handle   string    `json:"handle"`
	Display  string    `json:"display"`
	Sections []Section `json:"sections"`
}
type Section struct {
	Handle  string `json:"handle"`
	Display string `json:"display"`
}

// State = seluruh properti publik BlueprintForm/FieldsetForm yang dikirim balik oleh Alpine saat save.
type State struct {
	ID                       int64             `json:"id"`
	Namespace                string            `json:"namespace"`
	Handle                   string            `json:"handle"`
	Template                 string            `json:"template"`
	FieldsCollapsedByDefault bool              `json:"fieldsCollapsedByDefault"`
	Tabs                     []Tab             `json:"tabs"`
	Fields                   []Field           `json:"fields"`
	Errors                   map[string]string `json:"errors"`
}

var categories = []string{"collections", "globals", "taxonomies", "navigations", "forms"}

func Register(mux *http.ServeMux, a *web.App) {
	guard := func(w http.ResponseWriter, r *http.Request) bool {
		if !web.CurrentUser(r).Can("configure fields") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return false
		}
		return true
	}

	// ---- BlueprintManager ----
	a.Admin(mux, "GET /blueprints", func(w http.ResponseWriter, r *http.Request) {
		rows, _ := web.Rows(a.DB, `SELECT id,namespace,handle,data FROM blueprints ORDER BY namespace,handle`)
		groups := map[string][]map[string]any{}
		for _, row := range rows {
			ns, _ := row["namespace"].(string)
			cat := "other"
			if p, _, _ := strings.Cut(ns, "."); p != "" && contains(categories, p) {
				cat = p
			}
			row["fields"] = fieldCount(row["data"])
			groups[cat] = append(groups[cat], row)
		}
		a.Render(w, r, "blueprints/index.html", map[string]any{"Groups": groups, "Empty": len(rows) == 0})
	})
	a.Admin(mux, "POST /blueprints/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		var handle string
		a.DB.QueryRow(`SELECT handle FROM blueprints WHERE id=?`, r.PathValue("id")).Scan(&handle)
		a.DB.Exec(`DELETE FROM blueprints WHERE id=?`, r.PathValue("id"))
		activitylog.Record(a, web.CurrentUser(r).ID, "deleted", "blueprint", r.PathValue("id"), "blueprint '"+handle+"'", nil)
		web.Flash(w, "Blueprint deleted.")
		http.Redirect(w, r, a.CP("/blueprints"), http.StatusFound)
	})

	// Turbo membuang respons form berstatus 200; POST yang merender ulang form (= simpan gagal) wajib 422 supaya error tampil.
	rejected := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnprocessableEntity)
		}
	}

	// ---- BlueprintForm ----
	renderBlueprint := func(w http.ResponseWriter, r *http.Request, st State) {
		rejected(w, r)
		a.Render(w, r, "blueprints/form.html", map[string]any{"State": st, "Namespaces": namespaces(a), "Templates": templates(), "Lookups": lookups(a), "CollectionFlags": collectionFlags(a), "Editor": editorData()})
	}
	a.Admin(mux, "GET /blueprints/create", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		f := FieldDefaults()
		f.Section = "main"
		renderBlueprint(w, r, State{Tabs: []Tab{{Handle: "main", Display: "Main", Sections: []Section{{Handle: "main", Display: "Main"}}}}, Fields: []Field{f}, Errors: map[string]string{}})
	})
	a.Admin(mux, "GET /blueprints/{id}/edit", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		st, ok := loadBlueprint(a, r.PathValue("id"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		renderBlueprint(w, r, st)
	})
	a.Admin(mux, "POST /blueprints/save", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		var st State
		if err := json.Unmarshal([]byte(r.FormValue("state")), &st); err != nil {
			http.Error(w, "Bad state: "+err.Error(), 400)
			return
		}
		st.Fields = NormalizeTree(st.Fields)
		st.Errors = map[string]string{}
		nsValues := []string{}
		for _, n := range namespaces(a) {
			nsValues = append(nsValues, n.Value)
		}
		if st.Namespace != "" && !contains(nsValues, st.Namespace) {
			st.Errors["namespace"] = "The selected namespace is invalid."
		}
		if !handleRe.MatchString(st.Handle) {
			st.Errors["handle"] = "The handle field format is invalid."
		} else if n := 0; a.DB.QueryRow(`SELECT COUNT(*) FROM blueprints WHERE handle=? AND namespace<=>? AND id<>?`, st.Handle, nullable(st.Namespace), st.ID).Scan(&n) == nil && n > 0 {
			st.Errors["handle"] = "The handle has already been taken."
		}
		if st.Template != "" && !contains(templates(), st.Template) {
			st.Errors["template"] = "The selected template is invalid."
		}
		tabHandles, sectionsByTab := validateTabs(&st)
		if len(st.Fields) == 0 {
			st.Errors["fields"] = "Add at least one field."
		}
		for k, v := range ValidateTree(st.Fields, "fields", tabHandles, sectionsByTab, lookupsFor(lookups(a)), st.Fields) {
			st.Errors[k] = v
		}
		if len(st.Errors) > 0 {
			renderBlueprint(w, r, st)
			return
		}
		tabs := make([]map[string]any, len(st.Tabs))
		for i, t := range st.Tabs {
			secs := make([]map[string]any, len(t.Sections))
			for j, s := range t.Sections {
				secs[j] = map[string]any{"handle": s.Handle, "display": or(s.Display, Headline(s.Handle))}
			}
			tabs[i] = map[string]any{"handle": t.Handle, "display": or(t.Display, Headline(t.Handle)), "sections": secs}
		}
		fields := make([]map[string]any, len(st.Fields))
		for i, f := range st.Fields {
			fields[i] = ToStorage(f, true)
		}
		data, _ := json.Marshal(map[string]any{"template": nullable(st.Template), "fields_collapsed_by_default": st.FieldsCollapsedByDefault, "tabs": tabs, "fields": fields})
		id, err := upsert(a, "blueprints", st.ID, []string{"namespace", "handle", "data"}, nullable(st.Namespace), st.Handle, string(data))
		if err != nil {
			st.Errors["save"] = "Could not save: " + err.Error() // flash baru terbaca di request berikutnya, jadi tampilkan di form ini
			renderBlueprint(w, r, st)
			return
		}
		activitylog.Record(a, web.CurrentUser(r).ID, map[bool]string{true: "created", false: "updated"}[st.ID == 0], "blueprint", web.Itoa(id), "blueprint '"+st.Handle+"'", nil)
		web.Flash(w, "Blueprint saved.")
		http.Redirect(w, r, a.CP("/blueprints/"+web.Itoa(id)+"/edit"), http.StatusFound)
	})

	// ---- FieldsetManager ----
	a.Admin(mux, "GET /fieldsets", func(w http.ResponseWriter, r *http.Request) {
		rows, _ := web.Rows(a.DB, `SELECT id,handle,data FROM fieldsets ORDER BY handle`)
		for _, row := range rows {
			row["fields"] = fieldCount(row["data"])
		}
		a.Render(w, r, "blueprints/fieldsets.html", map[string]any{"Rows": rows})
	})
	a.Admin(mux, "POST /fieldsets/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		var handle string
		a.DB.QueryRow(`SELECT handle FROM fieldsets WHERE id=?`, r.PathValue("id")).Scan(&handle)
		a.DB.Exec(`DELETE FROM fieldsets WHERE id=?`, r.PathValue("id"))
		activitylog.Record(a, web.CurrentUser(r).ID, "deleted", "fieldset", r.PathValue("id"), "fieldset '"+handle+"'", nil)
		web.Flash(w, "Fieldset deleted.")
		http.Redirect(w, r, a.CP("/fieldsets"), http.StatusFound)
	})

	// ---- FieldsetForm ----
	renderFieldset := func(w http.ResponseWriter, r *http.Request, st State) {
		rejected(w, r)
		a.Render(w, r, "blueprints/fieldset-form.html", map[string]any{"State": st, "Lookups": lookups(a), "Editor": editorData()})
	}
	a.Admin(mux, "GET /fieldsets/create", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		renderFieldset(w, r, State{Tabs: fieldsetTabs, Fields: []Field{FieldDefaults()}, Errors: map[string]string{}})
	})
	a.Admin(mux, "GET /fieldsets/{id}/edit", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		st, ok := loadFieldset(a, r.PathValue("id"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		renderFieldset(w, r, st)
	})
	a.Admin(mux, "POST /fieldsets/save", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		var st State
		if err := json.Unmarshal([]byte(r.FormValue("state")), &st); err != nil {
			http.Error(w, "Bad state: "+err.Error(), 400)
			return
		}
		st.Tabs, st.Fields, st.Errors = fieldsetTabs, NormalizeTree(st.Fields), map[string]string{}
		if !handleRe.MatchString(st.Handle) {
			st.Errors["handle"] = "The handle field format is invalid."
		} else if n := 0; a.DB.QueryRow(`SELECT COUNT(*) FROM fieldsets WHERE handle=? AND id<>?`, st.Handle, st.ID).Scan(&n) == nil && n > 0 {
			st.Errors["handle"] = "The handle has already been taken."
		}
		if len(st.Fields) == 0 {
			st.Errors["fields"] = "Add at least one field."
		}
		for k, v := range ValidateTree(st.Fields, "fields", nil, nil, lookupsFor(lookups(a)), st.Fields) {
			st.Errors[k] = v
		}
		if len(st.Errors) > 0 {
			renderFieldset(w, r, st)
			return
		}
		fields := make([]map[string]any, len(st.Fields))
		for i, f := range st.Fields {
			fields[i] = ToStorage(f, false)
		}
		data, _ := json.Marshal(map[string]any{"fields_collapsed_by_default": st.FieldsCollapsedByDefault, "fields": fields})
		id, err := upsert(a, "fieldsets", st.ID, []string{"handle", "data"}, st.Handle, string(data))
		if err != nil {
			st.Errors["save"] = "Could not save: " + err.Error()
			renderFieldset(w, r, st)
			return
		}
		activitylog.Record(a, web.CurrentUser(r).ID, map[bool]string{true: "created", false: "updated"}[st.ID == 0], "fieldset", web.Itoa(id), "fieldset '"+st.Handle+"'", nil)
		web.Flash(w, "Fieldset saved.")
		http.Redirect(w, r, a.CP("/fieldsets/"+web.Itoa(id)+"/edit"), http.StatusFound)
	})
}

// fieldsetTabs = FieldsetForm::TABS: satu tab implisit "main" tanpa section.
var fieldsetTabs = []Tab{{Handle: "main", Display: "Main", Sections: []Section{}}}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func fieldCount(data any) int {
	var d struct {
		Fields []any `json:"fields"`
	}
	json.Unmarshal([]byte(anyToString(data)), &d)
	return len(d.Fields)
}

// upsert: id 0 → INSERT, selain itu UPDATE. Mengembalikan id.
func upsert(a *web.App, table string, id int64, cols []string, vals ...any) (int64, error) {
	if id == 0 {
		set := strings.Join(cols, ",")
		res, err := a.DB.Exec("INSERT INTO "+table+"("+set+",created_at,updated_at) VALUES("+strings.Repeat("?,", len(cols))+"?,?)", append(vals, web.Now(), web.Now())...)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	set := make([]string, len(cols))
	for i, c := range cols {
		set[i] = c + "=?"
	}
	_, err := a.DB.Exec("UPDATE "+table+" SET "+strings.Join(set, ",")+",updated_at=? WHERE id=?", append(vals, web.Now(), id)...)
	return id, err
}

func loadBlueprint(a *web.App, id string) (State, bool) {
	rows, _ := web.Rows(a.DB, `SELECT * FROM blueprints WHERE id=?`, id)
	if len(rows) == 0 {
		return State{}, false
	}
	row := rows[0]
	var data map[string]any
	json.Unmarshal([]byte(anyToString(row["data"])), &data)
	st := State{ID: web.Int64(row["id"]), Namespace: anyToString(row["namespace"]), Handle: anyToString(row["handle"]), Template: anyToString(data["template"]), Errors: map[string]string{}}
	st.FieldsCollapsedByDefault, _ = data["fields_collapsed_by_default"].(bool)
	if raw, _ := json.Marshal(data["tabs"]); len(raw) > 0 {
		json.Unmarshal(raw, &st.Tabs)
	}
	for i := range st.Tabs {
		if st.Tabs[i].Sections == nil {
			st.Tabs[i].Sections = []Section{}
		}
	}
	if len(st.Tabs) == 0 || st.Tabs[0].Handle != "main" {
		st.Tabs = append([]Tab{{Handle: "main", Display: "Main", Sections: []Section{}}}, st.Tabs...)
	}
	raw, _ := data["fields"].([]any)
	st.Fields = FromStorage(raw)
	if len(st.Fields) == 0 {
		st.Fields = []Field{FieldDefaults()}
	}
	return st, true
}

func loadFieldset(a *web.App, id string) (State, bool) {
	rows, _ := web.Rows(a.DB, `SELECT * FROM fieldsets WHERE id=?`, id)
	if len(rows) == 0 {
		return State{}, false
	}
	row := rows[0]
	var data map[string]any
	json.Unmarshal([]byte(anyToString(row["data"])), &data)
	st := State{ID: web.Int64(row["id"]), Handle: anyToString(row["handle"]), Tabs: fieldsetTabs, Errors: map[string]string{}}
	st.FieldsCollapsedByDefault, _ = data["fields_collapsed_by_default"].(bool)
	raw, _ := data["fields"].([]any)
	st.Fields = FromStorage(raw)
	if len(st.Fields) == 0 {
		st.Fields = []Field{FieldDefaults()}
	}
	return st, true
}

// validateTabs = BlueprintForm::validateTabs(): index 0 selalu "main".
func validateTabs(st *State) ([]string, map[string][]string) {
	if len(st.Tabs) == 0 {
		st.Tabs = []Tab{{Handle: "main", Display: "Main"}}
	}
	st.Tabs[0].Handle, st.Tabs[0].Display = "main", "Main"
	for i := range st.Tabs { // nil → [] supaya JSON ke Alpine bukan null (tab.sections.length)
		if st.Tabs[i].Sections == nil {
			st.Tabs[i].Sections = []Section{}
		}
	}
	tabHandles := []string{"main"}
	byTab := map[string][]string{}
	for i, t := range st.Tabs {
		h := "main"
		if i != 0 {
			h = strings.TrimSpace(t.Handle)
			key := "tabs." + web.Itoa(int64(i)) + ".handle"
			if h == "" || !handleRe.MatchString(h) || h == "main" {
				st.Errors[key] = `Tab handle must be lowercase letters, numbers and underscores, starting with a letter, and not "main".`
			} else if contains(tabHandles, h) {
				st.Errors[key] = "Tab handles must be unique."
			} else {
				tabHandles = append(tabHandles, h)
			}
		}
		secs := []string{}
		for j, s := range t.Sections {
			sh := strings.TrimSpace(s.Handle)
			key := "tabs." + web.Itoa(int64(i)) + ".sections." + web.Itoa(int64(j)) + ".handle"
			if sh == "" || !handleRe.MatchString(sh) {
				st.Errors[key] = "Section handle must be lowercase letters, numbers and underscores, starting with a letter."
			} else if contains(secs, sh) {
				st.Errors[key] = "Section handles must be unique within a tab."
			} else {
				secs = append(secs, sh)
			}
		}
		byTab[h] = secs
	}
	return tabHandles, byTab
}

// namespaces = availableNamespaces(): collections.*, globals.*, taxonomies.*, navigations.*, forms.*.
func namespaces(a *web.App) []Option {
	out := []Option{}
	for _, t := range [][2]string{{"collections", "collections"}, {"global_sets", "globals"}, {"taxonomies", "taxonomies"}, {"navigations", "navigations"}, {"forms", "forms"}} {
		rows, _ := web.Rows(a.DB, "SELECT handle,title FROM "+t[0]+" ORDER BY title")
		for _, r := range rows {
			v := t[1] + "." + anyToString(r["handle"])
			out = append(out, Option{v, anyToString(r["title"]) + " (" + v + ")"})
		}
	}
	return out
}

// templates = availableTemplates(): source membaca resources/views/pages/**/*.blade.php; di sini view Astro
// site/src/views/pages/**/*.astro (dirender oleh site/src/pages/[...slug].astro), notasi titik sama ("pages.static.homepage").
func templates() []string {
	dir := os.Getenv("SITE_VIEWS_DIR")
	if dir == "" {
		dir = "../site/src/views"
	}
	out := []string{}
	filepath.WalkDir(filepath.Join(dir, "pages"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".astro") {
			rel, _ := filepath.Rel(dir, p)
			out = append(out, strings.ReplaceAll(strings.TrimSuffix(filepath.ToSlash(rel), ".astro"), "/", "."))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func handles(a *web.App, table, order string) []string {
	rows, _ := web.Rows(a.DB, "SELECT handle FROM "+table+" ORDER BY "+order)
	out := []string{}
	for _, r := range rows {
		out = append(out, anyToString(r["handle"]))
	}
	return out
}

func lookups(a *web.App) map[string]any {
	titled := func(table string) []Option {
		rows, _ := web.Rows(a.DB, "SELECT handle,title FROM "+table+" ORDER BY title")
		out := []Option{}
		for _, r := range rows {
			out = append(out, Option{anyToString(r["handle"]), anyToString(r["title"])})
		}
		return out
	}
	return map[string]any{"assetContainers": titled("asset_containers"), "collections": titled("collections"), "taxonomies": titled("taxonomies"), "fieldsets": handles(a, "fieldsets", "handle")}
}

func lookupsFor(lk map[string]any) Lookups {
	vals := func(k string) []string {
		opts, _ := lk[k].([]Option)
		return values(opts)
	}
	fs, _ := lk["fieldsets"].([]string)
	return Lookups{AssetContainers: vals("assetContainers"), Collections: vals("collections"), Taxonomies: vals("taxonomies"), Fieldsets: fs}
}

// collectionFlags = schemaTabVisible()/seoTabVisible() untuk tiap collection, supaya Alpine bisa menampilkan tab "Schema"/"SEO" saat namespace berubah.
func collectionFlags(a *web.App) map[string]map[string]bool {
	rows, _ := web.Rows(a.DB, `SELECT handle,settings FROM collections`)
	out := map[string]map[string]bool{}
	for _, r := range rows {
		var s map[string]any
		json.Unmarshal([]byte(anyToString(r["settings"])), &s)
		schema, _ := s["schema_enabled"].(bool)
		seo, _ := s["seo_enabled"].(bool)
		out["collections."+anyToString(r["handle"])] = map[string]bool{"schema": schema, "seo": seo}
	}
	return out
}

func editorData() map[string]any {
	return map[string]any{"fieldtypes": Fieldtypes, "conditionOperators": ConditionOperators, "bardToolbarButtons": BardToolbarButtons}
}
