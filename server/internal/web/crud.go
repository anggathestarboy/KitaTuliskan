package web

import (
	"database/sql"
	"net/http"
	"regexp"
	"strings"
)

// Rows: SELECT bebas → []map untuk template/JSON.
func Rows(db *sql.DB, q string, args ...any) ([]map[string]any, error) {
	rs, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	cols, _ := rs.Columns()
	var out []map[string]any
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		rs.Scan(ptrs...)
		m := map[string]any{}
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok { // mysql driver: teks datang sebagai []byte
				m[c] = string(b)
			} else {
				m[c] = vals[i]
			}
		}
		out = append(out, m)
	}
	return out, nil
}

var handleRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

func ValidHandle(h string) bool { return handleRe.MatchString(h) }

type Field struct{ Name, Label, Type string }

// Resource: CRUD list generik untuk satu tabel di dalam admin panel.
// Menghasilkan GET <Path>, POST <Path>, POST <Path>/{id}/delete dengan template layouts/resource.html.
// Modul yang butuh halaman edit/anak (entries, terms, submissions) menambah route sendiri di sampingnya.
type Resource struct {
	Path     string            // "/collections" atau "/collections/{parent}" (child resource)
	Table    string            // tabel
	Title    string            // judul halaman
	One      string            // bentuk tunggal untuk tombol "New …" (default: Title)
	Cols     []string          // kolom tabel yang ditampilkan
	Fields   []Field           // field form create (nama = kolom)
	Perm     string            // permission untuk create/delete; kosong = cukup "access cp"
	Parent   string            // untuk child resource: kolom yang diisi dari {parent}, mis. "collection"
	Order    string            // ORDER BY, default "id DESC"
	Extra    map[string]string // kolom tambahan saat INSERT dengan nilai literal SQL, mis. {"data": "'{}'"}
	Link     string            // kalau diisi, kolom pertama jadi link ke Path+"/"+row[Link]
	NoCreate bool              // modul mendaftarkan POST <Path> sendiri (form create khusus)
}

func (a *App) Resource(mux *http.ServeMux, res Resource) {
	if res.Order == "" {
		res.Order = "id DESC"
	}
	guard := func(w http.ResponseWriter, r *http.Request) bool {
		if res.Perm != "" && !CurrentUser(r).Can(res.Perm) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return false
		}
		return true
	}
	where, args := "", func(r *http.Request) []any { return nil }
	if res.Parent != "" {
		where = " WHERE " + res.Parent + "=?"
		args = func(r *http.Request) []any { return []any{r.PathValue("parent")} }
	}
	base := func(r *http.Request) string {
		return a.CP(strings.Replace(res.Path, "{parent}", r.PathValue("parent"), 1))
	}

	a.Admin(mux, "GET "+res.Path, func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) { // = $user->authorizePermission() di mount() Livewire
			return
		}
		rows, err := Rows(a.DB, "SELECT * FROM "+res.Table+where+" ORDER BY "+res.Order, args(r)...)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		a.Render(w, r, "resource/index.html", map[string]any{"Res": res, "Rows": rows, "Base": base(r), "ParentValue": r.PathValue("parent")})
	})
	if !res.NoCreate {
		a.Admin(mux, "POST "+res.Path, a.resourceCreate(res, guard, base))
	}
	a.Admin(mux, "POST "+res.Path+"/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		if _, err := a.DB.Exec("DELETE FROM "+res.Table+" WHERE id=?", r.PathValue("id")); err != nil {
			Flash(w, err.Error())
		}
		http.Redirect(w, r, base(r), http.StatusFound)
	})
}

func (a *App) resourceCreate(res Resource, guard func(http.ResponseWriter, *http.Request) bool, base func(*http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		cols, marks, vals := []string{}, []string{}, []any{}
		for _, f := range res.Fields {
			v := strings.TrimSpace(r.FormValue(f.Name))
			if f.Name == "handle" && !ValidHandle(v) {
				Flash(w, "Handle must be lowercase letters, numbers, - or _.")
				http.Redirect(w, r, base(r), http.StatusFound)
				return
			}
			cols, marks, vals = append(cols, "`"+f.Name+"`"), append(marks, "?"), append(vals, v)
		}
		if res.Parent != "" {
			cols, marks, vals = append(cols, res.Parent), append(marks, "?"), append(vals, r.PathValue("parent"))
		}
		for k, v := range res.Extra {
			cols, marks = append(cols, "`"+k+"`"), append(marks, v)
		}
		cols, marks, vals = append(cols, "created_at", "updated_at"), append(marks, "?", "?"), append(vals, Now(), Now())
		if _, err := a.DB.Exec("INSERT INTO "+res.Table+"("+strings.Join(cols, ",")+") VALUES("+strings.Join(marks, ",")+")", vals...); err != nil {
			Flash(w, err.Error())
		} else {
			Flash(w, res.Title+" created.")
		}
		http.Redirect(w, r, base(r), http.StatusFound)
	}
}

// Page: halaman admin statis/sederhana tanpa tabel (About, 404 Detector, dsb).
func (a *App) Page(mux *http.ServeMux, path, tpl string, data func(r *http.Request) map[string]any) {
	a.Admin(mux, "GET "+path, func(w http.ResponseWriter, r *http.Request) {
		var d map[string]any
		if data != nil {
			d = data(r)
		}
		a.Render(w, r, tpl, d)
	})
}

// CleanSegment: satu segmen path tanpa / atau .. (untuk nama container/folder dari URL).
func CleanSegment(s string) string {
	if strings.ContainsAny(s, `/\`) || s == ".." || s == "." {
		return "_"
	}
	return s
}

// F: shorthand Field.
func F(name, label, typ string) Field { return Field{Name: name, Label: label, Type: typ} }
