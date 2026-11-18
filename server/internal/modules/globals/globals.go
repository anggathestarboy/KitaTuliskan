// Package globals = app/Modules/Globals: GlobalSetManager (daftar + modal) dan GlobalSetForm (nilai per bahasa).
// UI = React island (resources/ts/globals); data lewat JSON API di /globals/api. Form nilai memakai mesin yang sama dengan
// Navigation (navigation.RegisterContent), hanya penyimpanannya di global_set_variables.
package globals

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"draftcms-lite/internal/modules/navigation"
	"draftcms-lite/internal/web"
)

var handleRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type settings struct {
	Blueprint  *string  `json:"blueprint"`
	Sites      []string `json:"sites"`
	SaveAction string   `json:"save_action"`
}

type row struct {
	ID         int64    `json:"id"`
	Handle     string   `json:"handle"`
	Title      string   `json:"title"`
	Blueprint  string   `json:"blueprint"`
	Sites      []string `json:"sites"`
	SaveAction string   `json:"saveAction"`
	CanEdit    bool     `json:"canEdit"`
	CanDelete  bool     `json:"canDelete"`
	explicit   string   // settings.blueprint (pilihan eksplisit)
}

// Izin = GlobalSetManager::canEdit/canDelete: "configure globals" berlaku untuk semua set, atau izin per-set.
func canEdit(u *web.User, h string) bool { return u.Can("configure globals") || u.Can("edit "+h+" globals") }
func canDelete(u *web.User, h string) bool {
	return u.Can("configure globals") || u.Can("delete "+h+" globals")
}
func canView(u *web.User, h string) bool { return u.Can("view "+h+" globals") || canEdit(u, h) }

func Register(mux *http.ServeMux, a *web.App) {
	// load: satu baris + settings-nya. Blueprint efektif = pilihan eksplisit, kalau kosong "globals.{handle}".
	scan := func(r interface{ Scan(...any) error }) (g row, err error) {
		var raw []byte
		if err = r.Scan(&g.ID, &g.Handle, &g.Title, &raw); err != nil {
			return
		}
		var s settings
		json.Unmarshal(raw, &s)
		if s.Blueprint != nil {
			g.explicit = *s.Blueprint
		}
		if b := navigation.BlueprintFor(a, "globals", g.Handle, g.explicit); b != nil {
			g.Blueprint = b.Handle
		}
		g.Sites, g.SaveAction = s.Sites, s.SaveAction
		if g.Sites == nil {
			g.Sites = []string{}
		}
		if g.SaveAction == "" {
			g.SaveAction = "listing"
		}
		return
	}
	byHandle := func(h string) (row, error) {
		return scan(a.DB.QueryRow(`SELECT id,handle,title,settings FROM global_sets WHERE handle=?`, h))
	}
	// ensureVar = GlobalSetVariable::firstOrCreate: satu baris nilai per bahasa (locale NULL bila tanpa site).
	ensureVar := func(handle, locale string) error {
		var id int64
		var loc any
		if locale != "" {
			loc = locale
		}
		err := a.DB.QueryRow(`SELECT id FROM global_set_variables WHERE handle=? AND locale<=>? LIMIT 1`, handle, loc).Scan(&id)
		if err == sql.ErrNoRows {
			_, err = a.DB.Exec(`INSERT INTO global_set_variables(handle,locale,data,created_at,updated_at) VALUES(?,?,'{}',?,?)`, handle, loc, web.Now(), web.Now())
		}
		return err
	}

	a.Page(mux, "/globals", "globals/index.html", nil)
	a.Admin(mux, "GET /globals/{handle}", func(w http.ResponseWriter, r *http.Request) {
		g, err := byHandle(r.PathValue("handle"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if !canView(web.CurrentUser(r), g.Handle) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		a.Render(w, r, "globals/edit.html", map[string]any{"Handle": g.Handle, "SetTitle": g.Title})
	})

	a.Admin(mux, "GET /globals/api/list", func(w http.ResponseWriter, r *http.Request) {
		u := web.CurrentUser(r)
		rows, err := a.DB.Query(`SELECT id,handle,title,settings FROM global_sets ORDER BY title`)
		if err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		defer rows.Close()
		list := []row{}
		for rows.Next() {
			g, _ := scan(rows)
			g.CanEdit, g.CanDelete = canEdit(u, g.Handle), canDelete(u, g.Handle)
			list = append(list, g)
		}
		navigation.Reply(w, map[string]any{"sets": list, "canCreate": u.Can("configure globals"), "blueprints": navigation.BlueprintOptions(a), "sites": navigation.Sites(a)}, rows.Err())
	})

	a.Admin(mux, "POST /globals/api/save", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID                       int64
			Handle, Title, Blueprint string
			CreateBlueprint          bool
			Sites                    []string
			SaveAction               string
		}
		if err := navigation.Decode(w, r, &in); err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		u := web.CurrentUser(r)
		var old row
		if in.ID != 0 {
			var err error
			if old, err = scan(a.DB.QueryRow(`SELECT id,handle,title,settings FROM global_sets WHERE id=?`, in.ID)); err != nil {
				navigation.Reply(w, nil, err)
				return
			}
			if !canEdit(u, old.Handle) {
				navigation.Reply(w, nil, navigation.ErrForbidden)
				return
			}
		} else if !u.Can("configure globals") {
			navigation.Reply(w, nil, navigation.ErrForbidden)
			return
		}
		title, handle := strings.TrimSpace(in.Title), strings.TrimSpace(in.Handle)
		fail := func(m string) { navigation.Reply(w, nil, navigation.APIErr(m)) }
		var taken, bpCount int
		a.DB.QueryRow(`SELECT COUNT(*) FROM global_sets WHERE handle=? AND id<>?`, handle, in.ID).Scan(&taken)
		create := in.CreateBlueprint && in.ID == 0
		if in.Blueprint != "" && !create {
			a.DB.QueryRow(`SELECT COUNT(*) FROM blueprints WHERE handle=?`, in.Blueprint).Scan(&bpCount)
		}
		// Default site selalu ikut (origin language), urut sesuai urutan site.
		all := navigation.Sites(a)
		var sites []string
		for i, s := range all {
			if i == 0 || slices.Contains(in.Sites, s.Handle) {
				sites = append(sites, s.Handle)
			}
		}
		for _, s := range in.Sites {
			if !slices.ContainsFunc(all, func(x navigation.Site) bool { return x.Handle == s }) {
				fail("The selected sites.* is invalid.")
				return
			}
		}
		switch {
		case title == "":
			fail("The title field is required.")
		case handle == "":
			fail("The handle field is required.")
		case len(title) > 255 || len(handle) > 255:
			fail("The title and handle may not be greater than 255 characters.")
		case !handleRe.MatchString(handle):
			fail("The handle format is invalid.")
		case taken > 0:
			fail("The handle has already been taken.")
		case in.ID != 0 && handle != old.Handle: // global_set_variables.handle = FK tanpa ON UPDATE CASCADE
			fail("The handle cannot be changed after the global set is created.")
		case in.Blueprint != "" && !create && bpCount == 0:
			fail("The selected blueprint is invalid.")
		case in.SaveAction != "listing" && in.SaveAction != "editing":
			fail("The selected save action is invalid.")
		default:
			save(w, a, in.ID, handle, title, in.Blueprint, in.SaveAction, sites, create, ensureVar)
		}
	})

	a.Admin(mux, "POST /globals/api/delete", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ ID int64 }
		if err := navigation.Decode(w, r, &in); err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		g, err := scan(a.DB.QueryRow(`SELECT id,handle,title,settings FROM global_sets WHERE id=?`, in.ID))
		if err == nil && !canDelete(web.CurrentUser(r), g.Handle) {
			err = navigation.ErrForbidden
		}
		if err == nil {
			if _, err = a.DB.Exec(`DELETE FROM global_set_variables WHERE handle=?`, g.Handle); err == nil {
				_, err = a.DB.Exec(`DELETE FROM global_sets WHERE id=?`, g.ID)
			}
		}
		navigation.Reply(w, nil, err)
	})

	navigation.RegisterContent(mux, a, "/globals", navigation.Source{
		Find: func(h string) (string, *navigation.BP, error) {
			g, err := byHandle(h)
			if err != nil {
				return "", nil, err
			}
			return g.Title, navigation.BlueprintFor(a, "globals", h, g.explicit), nil
		},
		CanView: canView, CanEdit: canEdit, EmptyOK: true,
		Sites: func(h string) (out []navigation.Site) {
			g, _ := byHandle(h)
			for _, s := range navigation.Sites(a) {
				if slices.Contains(g.Sites, s.Handle) {
					out = append(out, s)
				}
			}
			return
		},
		Load: func(h, loc string) map[string]any {
			var raw sql.NullString
			var l any
			if loc != "" {
				l = loc
			}
			a.DB.QueryRow(`SELECT data FROM global_set_variables WHERE handle=? AND locale<=>? LIMIT 1`, h, l).Scan(&raw)
			out := map[string]any{}
			json.Unmarshal([]byte(raw.String), &out)
			return out
		},
		Save: func(h, loc string, d map[string]any) error {
			if err := ensureVar(h, loc); err != nil {
				return err
			}
			b, _ := json.Marshal(d)
			var l any
			if loc != "" {
				l = loc
			}
			_, err := a.DB.Exec(`UPDATE global_set_variables SET data=?, updated_at=? WHERE handle=? AND locale<=>?`, string(b), web.Now(), h, l)
			return err
		},
		Extra: func(h string) map[string]any { g, _ := byHandle(h); return map[string]any{"saveAction": g.SaveAction} },
		FlashOnSave: func(h string) string {
			if g, _ := byHandle(h); g.SaveAction == "listing" {
				return "Global set saved."
			}
			return ""
		},
	})
}

// save = GlobalSetManager::save() setelah validasi: simpan set, siapkan baris nilai tiap bahasa, buat blueprint bila diminta.
func save(w http.ResponseWriter, a *web.App, id int64, handle, title, blueprint, action string, sites []string, create bool, ensureVar func(h, loc string) error) {
	var picked *string
	if blueprint != "" && !create {
		picked = &blueprint
	}
	if sites == nil {
		sites = []string{}
	}
	raw, _ := json.Marshal(settings{Blueprint: picked, Sites: sites, SaveAction: action})
	var err error
	if id == 0 {
		_, err = a.DB.Exec(`INSERT INTO global_sets(handle,title,settings,created_at,updated_at) VALUES(?,?,?,?,?)`, handle, title, string(raw), web.Now(), web.Now())
	} else {
		_, err = a.DB.Exec(`UPDATE global_sets SET title=?, settings=?, updated_at=? WHERE id=?`, title, string(raw), web.Now(), id)
	}
	locales := sites
	if len(sites) == 0 {
		locales = []string{""} // tanpa site: satu baris locale NULL
	}
	for _, loc := range locales {
		if err == nil {
			err = ensureVar(handle, loc)
		}
	}
	created := false
	if err == nil && create {
		if created, err = navigation.CreateBlueprint(a, "globals", handle); created {
			web.Flash(w, "Global set saved — now add fields to its blueprint below.")
		}
	}
	navigation.Reply(w, map[string]any{"ok": true, "blueprintCreated": created}, err)
}
