package collections

// = EntryManager: daftar entry (flat atau tree untuk collection structured), hapus, duplikat, aksi massal, reorder.

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"draftcms-lite/internal/modules/navigation"
	"draftcms-lite/internal/web"
)

type listEntry struct {
	ID           string       `json:"id"`
	Site         string       `json:"site"`
	Title        string       `json:"title"`
	Slug         string       `json:"slug"`
	Blueprint    string       `json:"blueprint"`
	Status       string       `json:"status"`
	Order        *int         `json:"order"`
	UpdatedAt    string       `json:"updatedAt"`
	CanEdit      bool         `json:"canEdit"`
	CanDelete    bool         `json:"canDelete"`
	CanDeleteAll bool         `json:"canDeleteAll"`
	CanPublish   bool         `json:"canPublish"`
	SiblingCount int          `json:"siblingCount"`
	Children     []*listEntry `json:"children,omitempty"`
	e            *entry
}

// iso = "2006-01-02 15:04:05" (UTC) → ISO 8601.
func iso(s string) string {
	if s == "" {
		return ""
	}
	return strings.Replace(s, " ", "T", 1) + "Z"
}

// family = origin-family tiap entry collection: kunci origin (origin_id ?? id) → anggota (id + penulis).
type member struct{ id, author string }

func (m *mod) family(handle string) (map[string]string, map[string][]member) {
	keyOf := map[string]string{}
	fam := map[string][]member{}
	rows, err := m.a.DB.Query("SELECT id,origin_id,JSON_UNQUOTE(JSON_EXTRACT(seo,'$.meta_author')) FROM entries WHERE collection=?", handle)
	if err != nil {
		return keyOf, fam
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var origin, author *string
		rows.Scan(&id, &origin, &author)
		k := id
		if origin != nil && *origin != "" {
			k = *origin
		}
		a := ""
		if author != nil && *author != "null" {
			a = *author
		}
		keyOf[id] = k
		fam[k] = append(fam[k], member{id, a})
	}
	return keyOf, fam
}

// siblings = localeSiblings(): versi bahasa lain dari entry ini.
func (m *mod) siblings(e *entry) []*entry {
	k := e.Origin
	if k == "" {
		k = e.ID
	}
	return m.entryList(`WHERE collection=? AND id<>? AND (id=? OR origin_id=?)`, e.Collection, e.ID, k, k)
}

func (m *mod) registerEntries(mux *http.ServeMux) {
	a := m.a
	// page = halaman React; izin dicek seperti mount() Livewire.
	page := func(tpl, perm string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			c, err := m.col(r.PathValue("handle"))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			if !web.CurrentUser(r).Can(perm + " " + c.Handle + " entries") {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			a.Render(w, r, tpl, map[string]any{"Handle": c.Handle, "CollTitle": c.Title, "ID": r.PathValue("id"), "Site": r.PathValue("site")})
		}
	}
	a.Admin(mux, "GET /collections/{handle}", page("collections/entries.html", "view"))
	a.Admin(mux, "GET /collections/{handle}/entries/create/{site}", page("collections/entry.html", "create"))
	a.Admin(mux, "GET /collections/{handle}/entries/{id}", page("collections/entry.html", "edit"))

	a.Admin(mux, "GET /collections/api/entry-list/{handle}", func(w http.ResponseWriter, r *http.Request) {
		c, err := m.col(r.PathValue("handle"))
		u := web.CurrentUser(r)
		if err == nil && !u.Can("view "+c.Handle+" entries") {
			err = navigation.ErrForbidden
		}
		if err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		q, args := `WHERE collection=?`, []any{c.Handle}
		if s := r.URL.Query().Get("search"); s != "" {
			q += " AND (slug LIKE ? OR JSON_UNQUOTE(JSON_EXTRACT(data,'$.title')) LIKE ?)"
			args = append(args, "%"+s+"%", "%"+s+"%")
		}
		if s := r.URL.Query().Get("site"); s != "" {
			q, args = q+" AND site=?", append(args, s)
		}
		if !c.Structured {
			q += " ORDER BY (`order` != 1), updated_at DESC"
		}
		entries := m.entryList(q, args...)
		keyOf, fam := m.family(c.Handle)
		items := make([]*listEntry, 0, len(entries))
		for _, e := range entries {
			le := &listEntry{ID: e.ID, Site: e.Site, Title: e.title(), Slug: e.Slug, Blueprint: e.Blueprint, Status: e.Status, Order: e.Order,
				UpdatedAt: iso(e.UpdatedAt), CanEdit: e.canEdit(u), CanDelete: e.canDelete(u), CanPublish: canPublish(u, c.Handle, e), e: e}
			le.CanDeleteAll = le.CanDelete
			for _, s := range fam[keyOf[e.ID]] {
				if s.id == e.ID {
					continue
				}
				le.SiblingCount++
				if s.author != "" && s.author != u.ID && !u.Can("delete other authors "+c.Handle+" entries") {
					le.CanDeleteAll = false
				}
			}
			items = append(items, le)
		}
		var out any = items
		if c.Structured {
			out = buildTree(items, m.parents(c.Handle, items), "")
		}
		sites := navigation.Sites(a)
		createSite := ""
		if len(sites) > 0 {
			createSite = sites[0].Handle
		}
		bps := navigation.BlueprintHandles(a, "collections."+c.Handle)
		navigation.Reply(w, map[string]any{
			"collection": map[string]any{"handle": c.Handle, "title": c.Title, "structured": c.Structured, "orderable": c.Orderable},
			"entries":    out, "sites": sites, "createSite": createSite, "blueprints": bps, "defaultBlueprint": m.effectiveBlueprint(c),
			"canCreate": u.Can("create "+c.Handle+" entries") && len(bps) > 0 && createSite != "", "statuses": statusMap(), "statusOrder": statusOrder,
			"bulkAvailable": !c.Structured && !c.Orderable,
			"perms":         map[string]bool{"edit": u.Can("edit " + c.Handle + " entries"), "publish": u.Can("publish " + c.Handle + " entries"), "delete": u.Can("delete " + c.Handle + " entries"), "reorder": u.Can("reorder " + c.Handle + " entries")},
		}, nil)
	})

	a.Admin(mux, "POST /collections/api/entry-delete/{handle}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID            string
			DeleteLocales *bool
		}
		c, u, err := m.postCtx(w, r, &in, "delete")
		if err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		e, err := m.entry(in.ID, c.Handle)
		if err == nil && !e.canDelete(u) {
			err = navigation.ErrForbidden
		}
		if err == nil {
			sibs := m.siblings(e)
			if in.DeleteLocales == nil || *in.DeleteLocales {
				for _, s := range sibs {
					if !s.canDelete(u) {
						err = navigation.ErrForbidden
					}
				}
				for _, s := range sibs {
					if err != nil {
						break
					}
					if c.Structured {
						m.removeFromTree(c.Handle, s.Site, s.ID)
					}
					err = m.remove(s)
				}
			}
			if err == nil {
				if c.Structured {
					m.removeFromTree(c.Handle, e.Site, e.ID)
				}
				if err = m.remove(e); err == nil {
					m.log(r, "deleted", "entry", e.label())
				}
			}
		}
		navigation.Reply(w, nil, err)
	})

	a.Admin(mux, "POST /collections/api/entry-duplicate/{handle}", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ ID string }
		c, u, err := m.postCtx(w, r, &in, "edit")
		if err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		e, err := m.entry(in.ID, c.Handle)
		if err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		versions := append([]*entry{e}, m.siblings(e)...)
		for _, v := range versions {
			if !v.canEdit(u) {
				navigation.Reply(w, nil, navigation.ErrForbidden)
				return
			}
		}
		origin := e.Origin
		if origin == "" {
			origin = e.ID
		}
		sort.SliceStable(versions, func(i, j int) bool { return versions[i].ID == origin && versions[j].ID != origin })
		newOrigin := ""
		for _, v := range versions {
			slug := truncate(v.Slug+"-copy", 240)
			base := slug
			for n := 2; m.exists(v.Collection, v.Site, slug); n++ {
				slug = base + "-" + strconv.Itoa(n)
			}
			data := map[string]any{}
			for k, x := range v.Data {
				data[k] = x
			}
			t := v.title()
			if t == "" {
				t = v.Slug
			}
			data["title"] = t + " (Copy)"
			cp := &entry{ID: web.UUID(), Site: v.Site, Collection: v.Collection, Blueprint: v.Blueprint, Schema: v.Schema, Seo: v.Seo, hasSchema: v.hasSchema, hasSeo: v.hasSeo,
				Slug: slug, Status: "draft", Date: v.Date, Data: data}
			if v.ID != origin {
				cp.Origin = newOrigin
			}
			if err = m.insert(cp); err != nil {
				break
			}
			if newOrigin == "" {
				newOrigin = cp.ID
			}
			if c.Structured {
				var parent *string
				if pid := m.parentMap(c.Handle, v.Site)[v.ID]; pid != "" {
					parent = &pid
				}
				if err = m.setParent(c.Handle, cp.Site, cp.ID, parent, c.MaxDepth); err != nil {
					err = navigation.APIErr(err.Error())
					break
				}
			}
			m.setURI(c, cp)
			m.syncSitemap(cp)
			m.log(r, "created", "entry", cp.label())
		}
		navigation.Reply(w, nil, err)
	})

	a.Admin(mux, "POST /collections/api/entry-bulk/{handle}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Action, Status string
			IDs            []string
		}
		if err := navigation.Decode(w, r, &in); err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		m.bulk(w, r, in.Action, in.Status, in.IDs)
	})

	a.Admin(mux, "POST /collections/api/entry-reorder/{handle}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID       string
			Position int
			ParentID *string
			Site     string
		}
		c, _, err := m.postCtx(w, r, &in, "reorder")
		if err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		if !c.Orderable || in.Site == "" {
			navigation.Reply(w, nil, nil)
			return
		}
		all := m.entryList(`WHERE collection=? AND site=?`, c.Handle, in.Site)
		var moved *entry
		for _, e := range all {
			if e.ID == in.ID {
				moved = e
			}
		}
		if moved == nil {
			navigation.Reply(w, nil, nil)
			return
		}
		parent := ""
		if in.ParentID != nil {
			parent = *in.ParentID
		}
		sibs := []*entry{}
		pm := map[string]string{}
		if c.Structured {
			pm = m.parentMap(c.Handle, in.Site)
		}
		for _, e := range all {
			if e.ID != in.ID && (!c.Structured || pm[e.ID] == parent) {
				sibs = append(sibs, e)
			}
		}
		sort.SliceStable(sibs, func(i, j int) bool { return orderKey(sibs[i]) < orderKey(sibs[j]) })
		pos := max(0, min(in.Position, len(sibs)))
		sibs = append(sibs[:pos], append([]*entry{moved}, sibs[pos:]...)...)
		base := 1
		if parent != "" {
			base = 2
		}
		for i, e := range sibs {
			o := i + base
			e.Order = &o
			if err = m.update(e); err != nil {
				break
			}
		}
		if err == nil { // order 1 = homepage (uri ""): uri semua entry harus dihitung ulang, kalau tidak Astro masih menyajikan homepage lama
			for _, e := range all {
				m.setURI(c, e)
			}
		}
		navigation.Reply(w, nil, err)
	})
}

const noOrder = 1 << 30

func orderKey(e *entry) int {
	if e.Order == nil {
		return noOrder
	}
	return *e.Order
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func (m *mod) exists(collection, site, slug string) bool {
	var n int
	m.a.DB.QueryRow(`SELECT COUNT(*) FROM entries WHERE collection=? AND site=? AND slug=?`, collection, site, slug).Scan(&n)
	return n > 0
}

func statusMap() map[string]string { return statusLabels }

// postCtx = decode body + collection + izin dasar "{perm} {handle} entries".
func (m *mod) postCtx(w http.ResponseWriter, r *http.Request, in any, perm string) (*col, *web.User, error) {
	if err := navigation.Decode(w, r, in); err != nil {
		return nil, nil, err
	}
	c, err := m.col(r.PathValue("handle"))
	if err != nil {
		return nil, nil, err
	}
	u := web.CurrentUser(r)
	if perm != "" && !u.Can(perm+" "+c.Handle+" entries") {
		return nil, nil, navigation.ErrForbidden
	}
	return c, u, nil
}

// parents = parentIdOf() untuk daftar entry (per site): id → id induk.
func (m *mod) parents(handle string, items []*listEntry) map[string]string {
	bySite := map[string]map[string]string{}
	out := map[string]string{}
	for _, it := range items {
		if bySite[it.Site] == nil {
			bySite[it.Site] = m.parentMap(handle, it.Site)
		}
		out[it.ID] = bySite[it.Site][it.ID]
	}
	return out
}

// buildTree = buildTree()/sortedSiblings(): saudara diurutkan menurut order (kosong terakhir) lalu updated_at terbaru.
// Entry yang induknya tidak ikut daftar (mis. terfilter) tidak muncul, sama seperti versi lama.
func buildTree(items []*listEntry, parents map[string]string, parent string) []*listEntry {
	sib := []*listEntry{}
	for _, it := range items {
		if parents[it.ID] == parent {
			sib = append(sib, it)
		}
	}
	sort.SliceStable(sib, func(i, j int) bool {
		if oi, oj := orderKey(sib[i].e), orderKey(sib[j].e); oi != oj {
			return oi < oj
		}
		return sib[i].UpdatedAt > sib[j].UpdatedAt
	})
	for _, it := range sib {
		it.Children = buildTree(items, parents, it.ID)
		if it.Children == nil {
			it.Children = []*listEntry{}
		}
	}
	return sib
}

/* ---------- aksi massal ---------- */

func (m *mod) bulk(w http.ResponseWriter, r *http.Request, action, status string, ids []string) {
	c, err := m.col(r.PathValue("handle"))
	u := web.CurrentUser(r)
	if err != nil {
		navigation.Reply(w, nil, err)
		return
	}
	if c.Structured || c.Orderable { // bulkActionsAvailable(): hanya collection flat
		navigation.Reply(w, nil, nil)
		return
	}
	count := 0
	switch action {
	case "delete":
		if !u.Can("delete " + c.Handle + " entries") {
			navigation.Reply(w, nil, navigation.ErrForbidden)
			return
		}
		for _, id := range ids {
			e, err := m.entry(id, c.Handle)
			if err != nil {
				continue
			}
			sibs := m.siblings(e)
			ok := e.canDelete(u)
			for _, s := range sibs {
				ok = ok && s.canDelete(u)
			}
			if !ok {
				navigation.Reply(w, nil, navigation.ErrForbidden)
				return
			}
			for _, s := range sibs {
				m.remove(s)
			}
			m.remove(e)
			m.log(r, "deleted", "entry", e.label())
			count++
		}
	case "status":
		if !u.Can("edit " + c.Handle + " entries") {
			navigation.Reply(w, nil, navigation.ErrForbidden)
			return
		}
		if _, ok := statusLabels[status]; !ok {
			navigation.Reply(w, nil, nil)
			return
		}
		list := []*entry{}
		for _, id := range ids {
			if e, err := m.entry(id, c.Handle); err == nil {
				list = append(list, e)
			}
		}
		if status == "published" || status == "scheduled" {
			if !u.Can("publish " + c.Handle + " entries") {
				navigation.Reply(w, nil, navigation.ErrForbidden)
				return
			}
			for _, e := range list {
				if !e.ownedBy(u) && !u.Can("publish other authors "+c.Handle+" entries") {
					navigation.Reply(w, nil, navigation.ErrForbidden)
					return
				}
			}
		}
		for _, e := range list {
			resolved := status
			old := e.Status
			e.Status, e.Published = resolved, resolved == "published"
			if err = m.update(e); err != nil {
				break
			}
			if old != resolved && (resolved == "published" || resolved == "scheduled") {
				m.log(r, resolved, "entry", e.label())
			} else {
				m.log(r, "updated", "entry", e.label())
			}
			count++
		}
	default:
		navigation.Reply(w, nil, navigation.APIErr("The selected action is invalid."))
		return
	}
	navigation.Reply(w, map[string]any{"ok": true, "count": count}, err)
}
