// Package collections = app/Modules/Collections: CollectionManager (daftar + modal), EntryManager (daftar/tree entry),
// EntryForm (form entry), CollectionTreeService, EntryUrlResolver, entries:publish-scheduled, dan SitemapSyncer.
// UI = React island (resources/ts/collections); data lewat JSON API di /collections/api (lihat API.md).
// Live preview: state form disimpan di preview.go, dirender Astro (site/src/pages/preview). PageController publik juga di Astro.
package collections

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"draftcms-lite/internal/modules/activitylog"
	"draftcms-lite/internal/modules/navigation"
	"draftcms-lite/internal/web"
)

var handleRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// EntryForm::STATUSES (urutan tampil).
var statusOrder = []string{"published", "scheduled", "need_review", "draft"}
var statusLabels = map[string]string{"published": "Published", "scheduled": "Scheduled", "need_review": "Need Review", "draft": "Draft"}

type mod struct{ a *web.App }

// settings = kolom collections.settings (Collection::settings).
type settings struct {
	RouteMode     string            `json:"route_mode"`
	Route         *string           `json:"route"`
	Routes        map[string]string `json:"routes"`
	Sites         []string          `json:"sites"`
	Structured    bool              `json:"structured"`
	Orderable     bool              `json:"orderable"`
	VisibleOrder  *bool             `json:"visible_order"`
	LivePreview   bool              `json:"live_preview"`
	SchemaEnabled bool              `json:"schema_enabled"`
	SeoEnabled    bool              `json:"seo_enabled"`
	MaxDepth      *int              `json:"max_depth"`
	ExpectsRoot   bool              `json:"expects_root"`
	SaveAction    string            `json:"save_action"`
}

type col struct {
	ID            int64
	Handle, Title string
	settings
}

func (c *col) visibleOrder() bool { return c.VisibleOrder == nil || *c.VisibleOrder }
func (c *col) saveAction() string {
	if c.SaveAction == "" {
		return "listing"
	}
	return c.SaveAction
}
func (c *col) routeMode() string {
	if c.RouteMode == "" {
		return "single"
	}
	return c.RouteMode
}

// routeForSite = Collection::routeForSite(): "" = tidak punya route publik di site itu.
func (c *col) routeForSite(site string) string {
	if c.routeMode() == "per_site" {
		return c.Routes[site]
	}
	if c.Route != nil {
		return *c.Route
	}
	return ""
}

const colCols = `SELECT id,handle,title,settings FROM collections`

func scanCol(r interface{ Scan(...any) error }) (*col, error) {
	c := &col{}
	var raw []byte
	if err := r.Scan(&c.ID, &c.Handle, &c.Title, &raw); err != nil {
		return nil, err
	}
	json.Unmarshal(raw, &c.settings)
	return c, nil
}

func (m *mod) col(handle string) (*col, error) {
	return scanCol(m.a.DB.QueryRow(colCols+` WHERE handle=?`, handle))
}

// sitesOf = site milik collection (kosong = semua site); site pertama = default/origin language.
func (m *mod) sitesOf(c *col) (out []navigation.Site) {
	for _, s := range navigation.Sites(m.a) {
		if len(c.Sites) == 0 || slices.Contains(c.Sites, s.Handle) {
			out = append(out, s)
		}
	}
	return
}

func (m *mod) effectiveBlueprint(c *col) string {
	if b := navigation.BlueprintFor(m.a, "collections", c.Handle, ""); b != nil {
		return b.Handle
	}
	return ""
}

// defaultBlueprint = defaultBlueprintHandle(): blueprint efektif, kalau tak ada yang pertama menurut handle.
func (m *mod) defaultBlueprint(c *col) string {
	if h := m.effectiveBlueprint(c); h != "" {
		return h
	}
	if all := navigation.BlueprintHandles(m.a, "collections."+c.Handle); len(all) > 0 {
		return all[0]
	}
	return ""
}

// log = LogsActivity: "{user} {action} {label}" (no-op bila activity log mati).
func (m *mod) log(r *http.Request, action, subject, label string) {
	if u := web.CurrentUser(r); u != nil {
		activitylog.Record(m.a, u.ID, action, subject, "", u.Name+" "+action+" "+label, nil)
	}
}

type colOut struct {
	ID            int64             `json:"id"`
	Handle        string            `json:"handle"`
	Title         string            `json:"title"`
	Blueprint     string            `json:"blueprint"`
	Sites         []string          `json:"sites"`
	RouteMode     string            `json:"routeMode"`
	Route         string            `json:"route"`
	Routes        map[string]string `json:"routes"`
	Structured    bool              `json:"structured"`
	Orderable     bool              `json:"orderable"`
	VisibleOrder  bool              `json:"visibleOrder"`
	LivePreview   bool              `json:"livePreview"`
	SchemaEnabled bool              `json:"schemaEnabled"`
	SeoEnabled    bool              `json:"seoEnabled"`
	MaxDepth      string            `json:"maxDepth"`
	ExpectsRoot   bool              `json:"expectsRoot"`
	SaveAction    string            `json:"saveAction"`
}

func (m *mod) out(c *col) colOut {
	o := colOut{ID: c.ID, Handle: c.Handle, Title: c.Title, Blueprint: m.effectiveBlueprint(c), RouteMode: c.routeMode(), Routes: c.Routes,
		Structured: c.Structured, Orderable: c.Orderable, VisibleOrder: c.visibleOrder(), LivePreview: c.LivePreview, SchemaEnabled: c.SchemaEnabled,
		SeoEnabled: c.SeoEnabled, ExpectsRoot: c.ExpectsRoot, SaveAction: c.saveAction(), Sites: c.Sites}
	if c.Route != nil {
		o.Route = *c.Route
	}
	if o.Routes == nil {
		o.Routes = map[string]string{}
	}
	if len(o.Sites) == 0 { // edit(): tanpa pembatasan = semua site tercentang
		o.Sites = []string{}
		for _, s := range navigation.Sites(m.a) {
			o.Sites = append(o.Sites, s.Handle)
		}
	}
	if c.MaxDepth != nil {
		o.MaxDepth = strconv.Itoa(*c.MaxDepth)
	}
	return o
}

func Register(mux *http.ServeMux, a *web.App) {
	m := &mod{a}
	m.registerManager(mux)
	m.registerEntries(mux)
	m.registerForm(mux)
	m.registerPreview(mux)
	m.startScheduler()
}

func (m *mod) registerManager(mux *http.ServeMux) {
	a := m.a
	a.Page(mux, "/collections", "collections/index.html", nil)

	a.Admin(mux, "GET /collections/api/list", func(w http.ResponseWriter, r *http.Request) {
		rows, err := a.DB.Query(colCols + ` ORDER BY title`)
		if err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		defer rows.Close()
		list := []colOut{}
		for rows.Next() {
			if c, e := scanCol(rows); e == nil {
				list = append(list, m.out(c))
			}
		}
		navigation.Reply(w, map[string]any{"collections": list, "canConfigure": web.CurrentUser(r).Can("configure collections"), "sites": navigation.Sites(a)}, rows.Err())
	})

	a.Admin(mux, "POST /collections/api/save", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID                                                                          int64
			Handle, Title, RouteMode, Route                                             string
			Routes                                                                      map[string]string
			Sites                                                                       []string
			Structured, Orderable, VisibleOrder, LivePreview, SchemaEnabled, SeoEnabled bool
			MaxDepth                                                                    any
			ExpectsRoot                                                                 bool
			SaveAction                                                                  string
		}
		if err := navigation.Decode(w, r, &in); err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		if !web.CurrentUser(r).Can("configure collections") {
			navigation.Reply(w, nil, navigation.ErrForbidden)
			return
		}
		var old *col
		if in.ID != 0 {
			var err error
			if old, err = scanCol(a.DB.QueryRow(colCols+` WHERE id=?`, in.ID)); err != nil {
				navigation.Reply(w, nil, err)
				return
			}
		}
		title, handle := strings.TrimSpace(in.Title), strings.TrimSpace(in.Handle)
		fail := func(msg string) { navigation.Reply(w, nil, navigation.APIErr(msg)) }
		var taken int
		a.DB.QueryRow(`SELECT COUNT(*) FROM collections WHERE handle=? AND id<>?`, handle, in.ID).Scan(&taken)
		all := navigation.Sites(a)
		for _, s := range in.Sites {
			if !slices.ContainsFunc(all, func(x navigation.Site) bool { return x.Handle == s }) {
				fail("The selected sites.* is invalid.")
				return
			}
		}
		depth, depthMsg := parseDepth(in.MaxDepth)
		switch {
		case handle == "":
			fail("The handle field is required.")
		case title == "":
			fail("The title field is required.")
		case len(title) > 255 || len(handle) > 255:
			fail("The title and handle may not be greater than 255 characters.")
		case !handleRe.MatchString(handle):
			fail("The handle format is invalid.")
		case taken > 0:
			fail("The handle has already been taken.")
		case old != nil && handle != old.Handle: // entries.collection = FK tanpa ON UPDATE CASCADE
			fail("The handle cannot be changed after the collection is created.")
		case in.RouteMode != "single" && in.RouteMode != "per_site":
			fail("The selected route mode is invalid.")
		case len(in.Route) > 255 || slices.ContainsFunc(valuesOf(in.Routes), func(s string) bool { return len(s) > 255 }):
			fail("The route may not be greater than 255 characters.")
		case depthMsg != "":
			fail(depthMsg)
		case !slices.Contains([]string{"listing", "editing", "create_another"}, in.SaveAction):
			fail("The selected save action is invalid.")
		default:
			// Semua site tercentang = tanpa pembatasan; per-site hanya masuk akal untuk >1 site.
			sites := in.Sites
			if len(sites) == len(all) || sites == nil {
				sites = []string{}
			}
			s := settings{RouteMode: in.RouteMode, Sites: sites, Structured: in.Structured, Orderable: in.Orderable, VisibleOrder: &in.VisibleOrder,
				LivePreview: in.LivePreview, SchemaEnabled: in.SchemaEnabled, SeoEnabled: in.SeoEnabled, SaveAction: in.SaveAction, Routes: map[string]string{}}
			if len(in.Sites) <= 1 {
				s.RouteMode = "single"
			}
			if s.RouteMode == "per_site" {
				for site, route := range in.Routes {
					if route != "" && slices.Contains(in.Sites, site) {
						s.Routes[site] = route
					}
				}
			} else if in.Route != "" {
				s.Route = &in.Route
			}
			if in.Structured {
				s.MaxDepth, s.ExpectsRoot = depth, in.ExpectsRoot
			}
			raw, _ := json.Marshal(s)
			var err error
			action := "updated"
			if in.ID == 0 {
				action = "created"
				_, err = a.DB.Exec(`INSERT INTO collections(handle,title,settings,created_at,updated_at) VALUES(?,?,?,?,?)`, handle, title, string(raw), web.Now(), web.Now())
			} else {
				_, err = a.DB.Exec(`UPDATE collections SET title=?, settings=?, updated_at=? WHERE id=?`, title, string(raw), web.Now(), in.ID)
			}
			if err == nil && old != nil { // route bisa berubah
				var c *col
				if c, err = m.col(handle); err == nil {
					m.resyncURIs(c)
				}
			}
			if err == nil {
				m.log(r, action, "collection", "collection '"+title+"'")
			}
			navigation.Reply(w, nil, err)
		}
	})

	a.Admin(mux, "POST /collections/api/delete", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ ID int64 }
		if err := navigation.Decode(w, r, &in); err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		if !web.CurrentUser(r).Can("configure collections") {
			navigation.Reply(w, nil, navigation.ErrForbidden)
			return
		}
		c, err := scanCol(a.DB.QueryRow(colCols+` WHERE id=?`, in.ID))
		if err == nil {
			var n int
			if a.DB.QueryRow(`SELECT COUNT(*) FROM entries WHERE collection=?`, c.Handle).Scan(&n); n > 0 {
				err = navigation.APIErr(fmt.Sprintf("Cannot delete \"%s\" — it still has %d entries. Delete those first.", c.Title, n))
			} else if _, err = a.DB.Exec(`DELETE FROM collections WHERE id=?`, c.ID); err == nil {
				m.log(r, "deleted", "collection", "collection '"+c.Title+"'")
			}
		}
		navigation.Reply(w, nil, err)
	})
}

func valuesOf(m map[string]string) (out []string) {
	for _, v := range m {
		out = append(out, v)
	}
	return
}

// parseDepth = 'maxDepth' => nullable|integer|min:1; "" / nil = tanpa batas.
func parseDepth(v any) (*int, string) {
	var s string
	switch t := v.(type) {
	case nil:
		return nil, ""
	case string:
		s = strings.TrimSpace(t)
	case float64:
		s = strconv.FormatFloat(t, 'f', -1, 64)
	default:
		s = fmt.Sprint(t)
	}
	if s == "" {
		return nil, ""
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil, "The max depth field must be an integer."
	}
	if n < 1 {
		return nil, "The max depth field must be at least 1."
	}
	return &n, ""
}
