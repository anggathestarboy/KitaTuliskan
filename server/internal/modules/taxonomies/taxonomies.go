// Package taxonomies = app/Modules/Taxonomies: TaxonomyManager (daftar + modal), TermManager (daftar term) dan TermForm.
// UI = React island (resources/ts/taxonomies); data lewat JSON API di /taxonomies/api. Form term memakai mesin yang sama
// dengan Navigation/Globals (navigation.Fill/Validate/...), tetapi tiap bahasa = satu baris taxonomy_terms (origin_id).
package taxonomies

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"draftcms-lite/internal/modules/navigation"
	"draftcms-lite/internal/web"
)

var handleRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type settings struct {
	Blueprint  *string `json:"blueprint"`
	SaveAction string  `json:"save_action"`
}

type row struct {
	ID         int64    `json:"id"`
	Handle     string   `json:"handle"`
	Title      string   `json:"title"`
	Blueprint  string   `json:"blueprint"`
	Sites      []string `json:"sites"`
	SaveAction string   `json:"saveAction"`
	explicit   string   // settings.blueprint (pilihan eksplisit)
}

type term struct {
	ID        int64
	Site      string
	Origin    sql.NullInt64
	Slug      string
	Blueprint sql.NullString
	Data      map[string]any
}

func Register(mux *http.ServeMux, a *web.App) {
	// scan: satu baris + settings-nya. Blueprint efektif = pilihan eksplisit, kalau kosong "taxonomies.{handle}".
	scan := func(r interface{ Scan(...any) error }) (g row, err error) {
		var sites, raw []byte
		if err = r.Scan(&g.ID, &g.Handle, &g.Title, &sites, &raw); err != nil {
			return
		}
		var s settings
		json.Unmarshal(raw, &s)
		json.Unmarshal(sites, &g.Sites)
		if s.Blueprint != nil {
			g.explicit = *s.Blueprint
		}
		if b := navigation.BlueprintFor(a, "taxonomies", g.Handle, g.explicit); b != nil {
			g.Blueprint = b.Handle
		}
		g.SaveAction = s.SaveAction
		if g.Sites == nil {
			g.Sites = []string{}
		}
		if g.SaveAction == "" {
			g.SaveAction = "listing"
		}
		return
	}
	const cols = `SELECT id,handle,title,sites,settings FROM taxonomies`
	byHandle := func(h string) (row, error) { return scan(a.DB.QueryRow(cols+` WHERE handle=?`, h)) }
	// sitesOf = site milik taxonomy (kosong = semua site); site pertama = default/origin language.
	sitesOf := func(g row) (out []navigation.Site) {
		for _, s := range navigation.Sites(a) {
			if len(g.Sites) == 0 || slices.Contains(g.Sites, s.Handle) {
				out = append(out, s)
			}
		}
		return
	}
	forbid := func(w http.ResponseWriter) { navigation.Reply(w, nil, navigation.ErrForbidden) }

	/* ---------- TaxonomyManager ---------- */

	a.Page(mux, "/taxonomies", "taxonomies/index.html", nil)

	a.Admin(mux, "GET /taxonomies/api/list", func(w http.ResponseWriter, r *http.Request) {
		rows, err := a.DB.Query(cols + ` ORDER BY title`)
		if err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		defer rows.Close()
		list := []row{}
		for rows.Next() {
			g, _ := scan(rows)
			list = append(list, g)
		}
		navigation.Reply(w, map[string]any{"taxonomies": list, "canConfigure": web.CurrentUser(r).Can("configure taxonomies"), "blueprints": navigation.BlueprintOptions(a), "sites": navigation.Sites(a)}, rows.Err())
	})

	a.Admin(mux, "POST /taxonomies/api/save", func(w http.ResponseWriter, r *http.Request) {
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
		if !web.CurrentUser(r).Can("configure taxonomies") {
			forbid(w)
			return
		}
		var old row
		if in.ID != 0 {
			var err error
			if old, err = scan(a.DB.QueryRow(cols+` WHERE id=?`, in.ID)); err != nil {
				navigation.Reply(w, nil, err)
				return
			}
		}
		title, handle := strings.TrimSpace(in.Title), strings.TrimSpace(in.Handle)
		fail := func(m string) { navigation.Reply(w, nil, navigation.APIErr(m)) }
		var taken, bpCount int
		a.DB.QueryRow(`SELECT COUNT(*) FROM taxonomies WHERE handle=? AND id<>?`, handle, in.ID).Scan(&taken)
		create := in.CreateBlueprint && in.ID == 0
		if in.Blueprint != "" && !create {
			a.DB.QueryRow(`SELECT COUNT(*) FROM blueprints WHERE handle=?`, in.Blueprint).Scan(&bpCount)
		}
		// Default site selalu ikut (origin language), urut sesuai urutan site.
		all := navigation.Sites(a)
		sites := []string{}
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
		case in.ID != 0 && handle != old.Handle: // taxonomy_terms.taxonomy = FK tanpa ON UPDATE CASCADE
			fail("The handle cannot be changed after the taxonomy is created.")
		case in.Blueprint != "" && !create && bpCount == 0:
			fail("The selected blueprint is invalid.")
		case !slices.Contains([]string{"listing", "editing", "create_another"}, in.SaveAction):
			fail("The selected save action is invalid.")
		default:
			var picked *string
			if in.Blueprint != "" && !create {
				picked = &in.Blueprint
			}
			rawSites, _ := json.Marshal(sites)
			raw, _ := json.Marshal(settings{Blueprint: picked, SaveAction: in.SaveAction})
			var err error
			if in.ID == 0 {
				_, err = a.DB.Exec(`INSERT INTO taxonomies(handle,title,sites,settings,created_at,updated_at) VALUES(?,?,?,?,?,?)`, handle, title, string(rawSites), string(raw), web.Now(), web.Now())
			} else {
				_, err = a.DB.Exec(`UPDATE taxonomies SET title=?, sites=?, settings=?, updated_at=? WHERE id=?`, title, string(rawSites), string(raw), web.Now(), in.ID)
			}
			created := false
			if err == nil && create {
				if created, err = navigation.CreateBlueprint(a, "taxonomies", handle); created {
					web.Flash(w, "Taxonomy saved — now add fields to its blueprint below.")
				}
			}
			navigation.Reply(w, map[string]any{"ok": true, "blueprintCreated": created}, err)
		}
	})

	a.Admin(mux, "POST /taxonomies/api/delete", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ ID int64 }
		if err := navigation.Decode(w, r, &in); err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		if !web.CurrentUser(r).Can("configure taxonomies") {
			forbid(w)
			return
		}
		g, err := scan(a.DB.QueryRow(cols+` WHERE id=?`, in.ID))
		if err == nil {
			var n int
			if a.DB.QueryRow(`SELECT COUNT(*) FROM taxonomy_terms WHERE taxonomy=?`, g.Handle).Scan(&n); n > 0 {
				err = navigation.APIErr("Cannot delete \"" + g.Title + "\" — it still has " + strconv.Itoa(n) + " terms. Delete those first.")
			} else {
				_, err = a.DB.Exec(`DELETE FROM taxonomies WHERE id=?`, g.ID)
			}
		}
		navigation.Reply(w, nil, err)
	})

	/* ---------- TermManager ---------- */

	// page = halaman React untuk satu taxonomy; izin dicek seperti mount() Livewire.
	page := func(tpl, perm string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			g, err := byHandle(r.PathValue("handle"))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			if !web.CurrentUser(r).Can(perm + " " + g.Handle + " terms") {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			a.Render(w, r, tpl, map[string]any{"Handle": g.Handle, "TaxTitle": g.Title, "ID": r.PathValue("id"), "Site": r.PathValue("site")})
		}
	}
	a.Admin(mux, "GET /taxonomies/{handle}", page("taxonomies/terms.html", "view"))
	a.Admin(mux, "GET /taxonomies/{handle}/terms/create/{site}", page("taxonomies/term.html", "create"))
	a.Admin(mux, "GET /taxonomies/{handle}/terms/{id}", page("taxonomies/term.html", "edit"))

	a.Admin(mux, "GET /taxonomies/api/term-list/{handle}", func(w http.ResponseWriter, r *http.Request) {
		g, err := byHandle(r.PathValue("handle"))
		u := web.CurrentUser(r)
		if err == nil && !u.Can("view "+g.Handle+" terms") {
			err = navigation.ErrForbidden
		}
		if err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		rows, err := a.DB.Query(`SELECT id,site,slug,data,updated_at FROM taxonomy_terms WHERE taxonomy=? ORDER BY updated_at DESC, id DESC`, g.Handle)
		if err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		defer rows.Close()
		list := []map[string]any{}
		for rows.Next() {
			var id int64
			var site, slug string
			var raw []byte
			var upd sql.NullString
			rows.Scan(&id, &site, &slug, &raw, &upd)
			var d struct{ Title string }
			json.Unmarshal(raw, &d)
			at := ""
			if upd.Valid { // UTC "2006-01-02 15:04:05" → ISO
				at = strings.Replace(upd.String, " ", "T", 1) + "Z"
			}
			list = append(list, map[string]any{"id": id, "site": site, "title": d.Title, "slug": slug, "updatedAt": at})
		}
		ss := sitesOf(g)
		create := ""
		if len(ss) > 0 {
			create = ss[0].Handle
		}
		navigation.Reply(w, map[string]any{
			"taxonomy": map[string]any{"handle": g.Handle, "title": g.Title}, "terms": list, "sites": ss, "createSite": create,
			"hasBlueprint": g.Blueprint != "", "canCreate": u.Can("create " + g.Handle + " terms"),
			"canEdit": u.Can("edit " + g.Handle + " terms"), "canDelete": u.Can("delete " + g.Handle + " terms"),
		}, rows.Err())
	})

	a.Admin(mux, "POST /taxonomies/api/term-delete/{handle}", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ ID int64 }
		if err := navigation.Decode(w, r, &in); err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		h := r.PathValue("handle")
		if !web.CurrentUser(r).Can("delete " + h + " terms") {
			forbid(w)
			return
		}
		// origin_id = FK tanpa ON DELETE: terjemahan ikut terlepas dari origin-nya (Laravel nullOnDelete).
		a.DB.Exec(`UPDATE taxonomy_terms SET origin_id=NULL WHERE origin_id=? AND taxonomy=?`, in.ID, h)
		res, err := a.DB.Exec(`DELETE FROM taxonomy_terms WHERE id=? AND taxonomy=?`, in.ID, h)
		if err == nil {
			if n, _ := res.RowsAffected(); n == 0 {
				err = sql.ErrNoRows
			}
		}
		navigation.Reply(w, nil, err)
	})

	/* ---------- TermForm ---------- */

	load := func(id int64, h string) (t term, err error) {
		var raw []byte
		if err = a.DB.QueryRow(`SELECT id,site,origin_id,slug,blueprint,data FROM taxonomy_terms WHERE id=? AND taxonomy=?`, id, h).Scan(&t.ID, &t.Site, &t.Origin, &t.Slug, &t.Blueprint, &raw); err == nil {
			json.Unmarshal(raw, &t.Data)
		}
		if t.Data == nil {
			t.Data = map[string]any{}
		}
		return
	}
	// sibling = siblingTermForSite: term (atau origin-nya) di site tsb dengan origin yang sama; 0 bila belum ada.
	sibling := func(h string, originID int64, site string) (id int64) {
		if originID != 0 {
			a.DB.QueryRow(`SELECT id FROM taxonomy_terms WHERE taxonomy=? AND site=? AND (id=? OR origin_id=?) LIMIT 1`, h, site, originID, originID).Scan(&id)
		}
		return
	}
	// resolve: taxonomy + blueprint + site aktif/origin + term (id 0 = baru) + izin; dipakai GET dan POST.
	type ctx struct {
		g              row
		b              *navigation.BP
		ss             []navigation.Site
		active, origin string
		isOrigin       bool
		t              term
		originID       int64 // id term origin (0 = belum ada)
		originData     map[string]any
	}
	resolve := func(r *http.Request, id int64, site string, originQ int64) (c ctx, err error) {
		u := web.CurrentUser(r)
		if c.g, err = byHandle(r.PathValue("handle")); err != nil {
			return
		}
		perm := "edit"
		if id == 0 {
			perm = "create"
		}
		if !u.Can(perm + " " + c.g.Handle + " terms") {
			return c, navigation.ErrForbidden
		}
		if id != 0 {
			if c.t, err = load(id, c.g.Handle); err != nil {
				return
			}
			site, originQ = c.t.Site, c.t.Origin.Int64
		}
		c.ss = sitesOf(c.g)
		if !slices.ContainsFunc(c.ss, func(s navigation.Site) bool { return s.Handle == site }) && len(c.ss) > 0 {
			return c, navigation.APIErr("The selected site is invalid.")
		}
		c.active, c.origin, c.isOrigin = navigation.ResolveSite(c.ss, site)
		if b := navigation.BlueprintFor(a, "taxonomies", c.g.Handle, c.g.explicit); b != nil {
			c.b = b
		} else {
			c.b = &navigation.BP{Tabs: []any{map[string]any{"handle": "main", "display": "Main"}}, Fields: []map[string]any{}}
		}
		c.originID = originQ
		if id != 0 && originQ == 0 {
			c.originID = id
		}
		c.originData = map[string]any{}
		if !c.isOrigin {
			if oid := sibling(c.g.Handle, c.originID, c.origin); oid != 0 {
				o, _ := load(oid, c.g.Handle)
				c.originData = o.Data
			}
		}
		return
	}
	// overlay = overlayNonLocalizableData: field non-localizable (kecuali replicator) selalu mengikuti bahasa default.
	overlay := func(fields []map[string]any, data, origin map[string]any) {
		skip := navigation.NonLocalizable(fields)
		for _, f := range fields {
			if h, _ := f["handle"].(string); skip[h] && f["type"] != "replicator" {
				if v, ok := origin[h]; ok && v != nil {
					data[h] = v
				}
			}
		}
	}
	termID := func(r *http.Request) int64 { n, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64); return n }

	a.Admin(mux, "GET /taxonomies/api/term/{handle}", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		origin, _ := strconv.ParseInt(q.Get("origin"), 10, 64)
		c, err := resolve(r, termID(r), q.Get("site"), origin)
		if err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		fields := c.b.Fields
		data := navigation.Fill(fields, c.t.Data)
		if !c.isOrigin {
			od := navigation.Fill(fields, c.originData)
			overlay(fields, data, od)
			if c.t.ID == 0 { // terjemahan baru: field localizable dimulai dari nilai bahasa default
				skip := navigation.NonLocalizable(fields)
				for _, f := range fields {
					if h, _ := f["handle"].(string); !skip[h] && f["type"] != "replicator" {
						data[h] = od[h]
					}
				}
			}
			navigation.LocalizeLinks(a, data, fields, c.active)
		}
		// Site switcher: hanya bila >1 site; href ke term saudara di site itu, atau form create untuk site itu.
		links := []map[string]any{}
		if len(c.ss) > 1 {
			for _, s := range c.ss {
				href := ""
				if s.Handle != c.active {
					if sid := sibling(c.g.Handle, c.originID, s.Handle); sid != 0 {
						href = a.CP("/taxonomies/" + c.g.Handle + "/terms/" + strconv.FormatInt(sid, 10))
					} else {
						href = a.CP("/taxonomies/" + c.g.Handle + "/terms/create/" + s.Handle)
						if c.originID != 0 {
							href += "?origin=" + strconv.FormatInt(c.originID, 10)
						}
					}
				}
				links = append(links, map[string]any{"handle": s.Handle, "name": s.Name, "href": href})
			}
		}
		title, _ := c.t.Data["title"].(string)
		out := map[string]any{
			"item": map[string]any{"handle": c.g.Handle, "title": c.g.Title}, "blueprint": c.b, "sites": links, "activeSite": c.active, "isOrigin": c.isOrigin,
			"data": data, "canEdit": true, "saveAction": c.g.SaveAction,
			"term": map[string]any{"id": c.t.ID, "title": title, "slug": c.t.Slug, "origin": c.originID},
		}
		for k, v := range navigation.Lookups(a) {
			out[k] = v
		}
		navigation.Reply(w, out, nil)
	})

	a.Admin(mux, "POST /taxonomies/api/term/{handle}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID, Origin  int64
			Site        string
			Title, Slug string
			Data        map[string]any
		}
		if err := navigation.Decode(w, r, &in); err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		c, err := resolve(r, in.ID, in.Site, in.Origin)
		if err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		title, slug := strings.TrimSpace(in.Title), strings.TrimSpace(in.Slug)
		var taken int
		a.DB.QueryRow(`SELECT COUNT(*) FROM taxonomy_terms WHERE slug=? AND taxonomy=? AND site=? AND id<>?`, slug, c.g.Handle, c.active, in.ID).Scan(&taken)
		switch {
		case title == "":
			err = navigation.APIErr("The title field is required.")
		case len([]rune(title)) > 300:
			err = navigation.APIErr("The title field must not be greater than 300 characters.")
		case slug == "":
			err = navigation.APIErr("The slug field is required.")
		case len([]rune(slug)) > 255:
			err = navigation.APIErr("The slug field must not be greater than 255 characters.")
		case taken > 0:
			err = navigation.APIErr("The slug has already been taken.")
		}
		var data map[string]any
		if err == nil {
			data, err = navigation.Validate(a, c.active, c.b.Fields, in.Data)
		}
		if err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		data["title"] = title
		if !c.isOrigin {
			overlay(c.b.Fields, data, navigation.Fill(c.b.Fields, c.originData))
		}
		raw, _ := json.Marshal(data)
		var origin any
		if c.originID != 0 && c.originID != in.ID {
			origin = c.originID
		}
		id := in.ID
		if id == 0 {
			var res sql.Result
			if res, err = a.DB.Exec(`INSERT INTO taxonomy_terms(site,origin_id,slug,taxonomy,blueprint,data,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`,
				c.active, origin, slug, c.g.Handle, c.b.Handle, string(raw), web.Now(), web.Now()); err == nil {
				id, _ = res.LastInsertId()
			}
		} else {
			_, err = a.DB.Exec(`UPDATE taxonomy_terms SET slug=?, blueprint=?, data=?, updated_at=? WHERE id=?`, slug, c.b.Handle, string(raw), web.Now(), id)
		}
		if err == nil {
			web.Flash(w, "Term saved.")
		}
		navigation.Reply(w, map[string]any{"ok": true, "id": id, "site": c.active, "saveAction": c.g.SaveAction}, err)
	})
}
