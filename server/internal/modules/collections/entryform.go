package collections

// = EntryForm: muat (GET) dan simpan (POST) satu entry. Field blueprint memakai mesin yang sama dengan Navigation/Globals/Taxonomies
// (navigation.Fill/Validate/...); yang khas entry: status/tanggal/order/parent, tab SEO dan Schema, serta satu baris per bahasa (origin_id).

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"draftcms-lite/internal/modules/navigation"
	"draftcms-lite/internal/modules/sitesettings"
	"draftcms-lite/internal/web"
)

type ectx struct {
	c        *col
	u        *web.User
	e        *entry // nil = entry baru
	ss       []navigation.Site
	site     string
	defSite  string
	isOrigin bool
	bpHandle string
	b        *navigation.BP
	originID string
	avail    []string
}

// resolveEntry = mount(): collection, entry (atau site/origin/blueprint untuk entry baru), izin, blueprint dan bahasa aktif.
func (m *mod) resolveEntry(r *http.Request, id, siteQ, originQ, bpQ string) (*ectx, error) {
	c, err := m.col(r.PathValue("handle"))
	if err != nil {
		return nil, err
	}
	ec := &ectx{c: c, u: web.CurrentUser(r)}
	perm := "create"
	if id != "" {
		perm = "edit"
	}
	if !ec.u.Can(perm + " " + c.Handle + " entries") {
		return nil, navigation.ErrForbidden
	}
	ec.site, ec.originID, ec.bpHandle = siteQ, originQ, bpQ
	if id != "" {
		if ec.e, err = m.entry(id, c.Handle); err != nil {
			return nil, err
		}
		if !ec.e.canEdit(ec.u) {
			return nil, navigation.ErrForbidden
		}
		ec.site, ec.originID, ec.bpHandle = ec.e.Site, ec.e.Origin, ec.e.Blueprint
		if ec.originID == "" {
			ec.originID = ec.e.ID
		}
	}
	if ec.bpHandle == "" {
		ec.bpHandle = m.defaultBlueprint(c)
	}
	ec.ss = m.sitesOf(c)
	if len(ec.ss) > 0 {
		ec.defSite = ec.ss[0].Handle
		if id == "" && !slices.ContainsFunc(ec.ss, func(s navigation.Site) bool { return s.Handle == ec.site }) {
			return nil, navigation.APIErr("The selected site is invalid.")
		}
	}
	ec.isOrigin = ec.defSite == "" || ec.site == ec.defSite
	ec.avail = navigation.BlueprintHandles(m.a, "collections."+c.Handle)
	if ec.b = navigation.BlueprintIn(m.a, "collections."+c.Handle, ec.bpHandle); ec.b == nil {
		ec.b = &navigation.BP{Tabs: []any{map[string]any{"handle": "main", "display": "Main"}}, Fields: []map[string]any{}}
	}
	return ec, nil
}

// sibling = siblingEntryForSite(): entry di site tsb dengan origin yang sama (atau origin-nya sendiri).
func (m *mod) sibling(handle, originID, site string) *entry {
	if originID == "" {
		return nil
	}
	l := m.entryList(`WHERE collection=? AND site=? AND (id=? OR origin_id=?) LIMIT 1`, handle, site, originID, originID)
	if len(l) == 0 {
		return nil
	}
	return l[0]
}

// originEntry = originEntry(): entry ini sendiri di bahasa default, kalau tidak saudaranya di bahasa default.
func (m *mod) originEntry(ec *ectx) *entry {
	if ec.isOrigin {
		return ec.e
	}
	return m.sibling(ec.c.Handle, ec.originID, ec.defSite)
}

// overlay = overlayNonLocalizableData(): field non-localizable (kecuali replicator) selalu mengikuti bahasa default.
func overlay(fields []map[string]any, data, origin map[string]any) {
	skip := navigation.NonLocalizable(fields)
	for _, f := range fields {
		if h := sv(f["handle"]); skip[h] && f["type"] != "replicator" {
			if v, ok := origin[h]; ok && v != nil {
				data[h] = v
			}
		}
	}
}

// dropDeletedSeoImages: id gambar yang sudah dihapus dari media library dibuang.
func (m *mod) dropDeletedSeoImages(seo map[string]any) {
	ids := []any{}
	raw, _ := seo["meta_featured_image"].([]any)
	for _, x := range raw {
		var n int
		m.a.DB.QueryRow(`SELECT COUNT(*) FROM assets_meta WHERE id=?`, sv(x)).Scan(&n)
		if n > 0 {
			ids = append(ids, x)
		}
	}
	seo["meta_featured_image"] = ids
}

func blocksOrEmpty(b []sitesettings.Block) []sitesettings.Block {
	out := make([]sitesettings.Block, len(b))
	for i, x := range b {
		if x.SameAs == nil {
			x.SameAs = []string{}
		}
		if x.Type == "" {
			x.Type = "Organization"
		}
		out[i] = x
	}
	return out
}

func (m *mod) registerForm(mux *http.ServeMux) {
	a := m.a

	a.Admin(mux, "GET /collections/api/entry/{handle}", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		ec, err := m.resolveEntry(r, q.Get("id"), q.Get("site"), q.Get("origin"), q.Get("blueprint"))
		if err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		c, e, fields := ec.c, ec.e, ec.b.Fields
		var own map[string]any
		if e != nil {
			own = e.Data
		}
		data := navigation.Fill(fields, own)
		oe := m.originEntry(ec)
		var od map[string]any
		if oe != nil {
			od = oe.Data
		}
		if !ec.isOrigin {
			full := navigation.Fill(fields, od)
			overlay(fields, data, full)
			if e == nil { // terjemahan baru: field localizable dimulai dari nilai bahasa default
				skip := navigation.NonLocalizable(fields)
				for _, f := range fields {
					if h := sv(f["handle"]); !skip[h] && f["type"] != "replicator" {
						data[h] = full[h]
					}
				}
			}
			navigation.LocalizeLinks(a, data, fields, ec.site)
		}

		seo := map[string]any{}
		if c.SeoEnabled {
			if e != nil {
				seo = e.seoFields()
			} else {
				seo = defaultSeo()
			}
			m.dropDeletedSeoImages(seo)
			if !ec.isOrigin {
				os := defaultSeo()
				if oe != nil {
					os = oe.seoFields()
				}
				for k, loc := range seoLocalizable {
					if !loc {
						seo[k] = os[k]
					}
				}
				m.dropDeletedSeoImages(seo)
			}
		}
		var blocks []sitesettings.Block
		if c.SchemaEnabled && e != nil {
			blocks = e.Schema
		}

		// siteLinks: hanya bila >1 site; href ke entry saudara di site itu, atau form create untuk site itu.
		links := []map[string]any{}
		if len(ec.ss) > 1 {
			for _, s := range ec.ss {
				href := ""
				if s.Handle != ec.site {
					if sib := m.sibling(c.Handle, ec.originID, s.Handle); sib != nil {
						href = a.CP("/collections/" + c.Handle + "/entries/" + sib.ID)
					} else {
						href = a.CP("/collections/"+c.Handle+"/entries/create/"+s.Handle) + "?blueprint=" + ec.bpHandle
						if ec.originID != "" {
							href += "&origin=" + ec.originID
						}
					}
				}
				links = append(links, map[string]any{"handle": s.Handle, "name": s.Name, "href": href})
			}
		}

		ent := map[string]any{"id": "", "title": "", "slug": "", "status": "draft", "date": "", "order": "", "parentId": nil, "origin": nil, "blueprint": ec.bpHandle, "url": ""}
		if ec.originID != "" && (e == nil || e.Origin != "") {
			ent["origin"] = ec.originID
		}
		if e != nil {
			ent["id"], ent["title"], ent["slug"], ent["url"] = e.ID, e.title(), e.Slug, m.url(c, e)
			if e.Status != "" {
				ent["status"] = e.Status
			}
			if t, ok := parseDate(e.Date); ok { // menit saja: input datetime-local menolak detik
				ent["date"] = t.Format("2006-01-02T15:04")
			}
			if e.Order != nil {
				ent["order"] = strconv.Itoa(*e.Order)
			}
			if c.Structured {
				if pid := m.parentMap(c.Handle, e.Site)[e.ID]; pid != "" {
					ent["parentId"] = pid
				}
			}
		}
		parents := []map[string]string{}
		if c.Structured {
			pq, pargs := `WHERE collection=? AND site=?`, []any{c.Handle, ec.site}
			if e != nil {
				pq, pargs = pq+` AND id<>?`, append(pargs, e.ID)
			}
			for _, p := range m.entryList(pq+` ORDER BY slug`, pargs...) {
				t := p.title()
				if t == "" {
					t = p.Slug
				}
				if t == "" {
					t = p.ID
				}
				parents = append(parents, map[string]string{"id": p.ID, "title": t})
			}
		}

		if c.LivePreview { // = mount(): token baru per muat halaman + payload awal dari state tersimpan
			tok := newPreviewToken()
			ent["previewUrl"] = previewURL(tok)
			str := func(k string) string { s, _ := ent[k].(string); return s }
			putPreview(tok, previewPayload(c.Handle, ec.bpHandle, ec.site, str("id"), str("origin"), str("status"), str("slug"), str("date"), ent["order"], str("title"), data))
		}

		out := map[string]any{
			"item": map[string]any{"handle": c.Handle, "title": c.Title}, "blueprint": ec.b, "sites": links, "activeSite": ec.site, "isOrigin": ec.isOrigin,
			"data": data, "canEdit": true,
			"collection": map[string]any{"handle": c.Handle, "title": c.Title, "structured": c.Structured, "visibleOrder": c.visibleOrder(), "livePreview": c.LivePreview,
				"schemaEnabled": c.SchemaEnabled, "seoEnabled": c.SeoEnabled, "saveAction": c.saveAction()},
			"entry": ent, "statuses": statusLabels, "statusOrder": statusOrder, "canEditStatus": canPublish(ec.u, c.Handle, e), "parents": parents,
			"seo": seo, "schemaBlocks": blocksOrEmpty(blocks), "schemaTypes": sitesettings.SchemaTypes,
			"seoOptions": map[string]any{"robots": seoRobotLabels, "robotsOrder": seoRobots, "priorities": seoPriorities, "changefreqs": seoChangefreqs},
		}
		for k, v := range navigation.Lookups(a) {
			out[k] = v
		}
		navigation.Reply(w, out, nil)
	})

	a.Admin(mux, "POST /collections/api/entry/{handle}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID, Site, Origin, Blueprint, Title, Slug, Status, Date string
			Order                                                  any
			ParentID                                               string
			Data, Seo                                              map[string]any
			SchemaBlocks                                           []sitesettings.Block
		}
		if err := navigation.Decode(w, r, &in); err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		ec, err := m.resolveEntry(r, in.ID, in.Site, in.Origin, in.Blueprint)
		if err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		m.save(w, r, ec, in.Title, in.Slug, in.Status, in.Date, in.Order, in.ParentID, in.Data, in.Seo, in.SchemaBlocks)
	})
}

// errs mengumpulkan galat validasi per kunci; pesan pertama jadi pesan utama (seperti urutan rules Laravel).
type errs struct {
	all   map[string]string
	first string
}

func (e *errs) add(key, msg string) {
	if _, dup := e.all[key]; dup {
		return
	}
	e.all[key] = msg
	if e.first == "" {
		e.first = msg
	}
}

func isIntLike(v any) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), t == float64(int(t))
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		return n, err == nil
	}
	return 0, false
}

func (m *mod) seoValidate(v *errs, seo map[string]any) map[string]any {
	clean := map[string]any{}
	str := func(key, label string, max int, required bool) {
		s := sv(seo[key])
		switch {
		case required && strings.TrimSpace(s) == "":
			v.add("seo."+key, "The "+label+" field is required.")
		case len([]rune(s)) > max:
			v.add("seo."+key, "The "+label+" field must not be greater than "+strconv.Itoa(max)+" characters.")
		}
		clean[key] = s
	}
	str("meta_title", "Meta Title", 60, true)
	str("meta_description", "Meta Description", 160, true)
	str("meta_canonical", "Meta Canonical", 2048, false)
	robots, _ := seo["robots"].([]any)
	if len(robots) == 0 {
		v.add("seo.robots", "The Robots field is required.")
	}
	for _, x := range robots {
		if !slices.Contains(seoRobots, sv(x)) {
			v.add("seo.robots", "The selected Robots is invalid.")
		}
	}
	clean["robots"] = robots
	imgs, _ := seo["meta_featured_image"].([]any)
	switch {
	case len(imgs) == 0:
		v.add("seo.meta_featured_image", "The Meta Featured Image field is required.")
	case len(imgs) > 1:
		v.add("seo.meta_featured_image", "The Meta Featured Image field must not have more than 1 items.")
	}
	ids := []any{}
	for _, x := range imgs {
		n, ok := isIntLike(x)
		var c int
		if ok {
			m.a.DB.QueryRow(`SELECT COUNT(*) FROM assets_meta WHERE id=?`, n).Scan(&c)
		}
		if c == 0 {
			v.add("seo.meta_featured_image", "The selected Meta Featured Image is invalid.")
		}
		ids = append(ids, n)
	}
	clean["meta_featured_image"] = ids
	author := sv(seo["meta_author"])
	var c int
	m.a.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE id=?`, author).Scan(&c)
	switch {
	case author == "":
		v.add("seo.meta_author", "The Meta Author field is required.")
	case c == 0:
		v.add("seo.meta_author", "The selected Meta Author is invalid.")
	}
	clean["meta_author"] = author
	prio, cf := sv(seo["sitemap_priority"]), sv(seo["sitemap_changefreq"])
	if !slices.Contains(seoPriorities, prio) {
		v.add("seo.sitemap_priority", "The selected Sitemap Priority is invalid.")
	}
	if !slices.Contains(seoChangefreqs, cf) {
		v.add("seo.sitemap_changefreq", "The selected Sitemap Changefreq is invalid.")
	}
	clean["sitemap_priority"], clean["sitemap_changefreq"] = prio, cf
	return clean
}

// save = EntryForm::save() setelah resolve.
func (m *mod) save(w http.ResponseWriter, r *http.Request, ec *ectx, title, slug, status, date string, orderIn any, parentID string, in, seoIn map[string]any, blocks []sitesettings.Block) {
	c, e, u := ec.c, ec.e, ec.u
	v := &errs{all: map[string]string{}}
	title, slug = strings.TrimSpace(title), strings.TrimSpace(slug)
	switch {
	case title == "":
		v.add("title", "The title field is required.")
	case len([]rune(title)) > 300:
		v.add("title", "The title field must not be greater than 300 characters.")
	}
	switch {
	case slug == "":
		v.add("slug", "The slug field is required.")
	case len([]rune(slug)) > 255:
		v.add("slug", "The slug field must not be greater than 255 characters.")
	}
	if status == "" {
		v.add("status", "The status field is required.")
	} else if _, ok := statusLabels[status]; !ok {
		v.add("status", "The selected status is invalid.")
	}
	if strings.TrimSpace(date) == "" {
		v.add("date", "The date field is required.")
	} else if _, ok := parseDate(date); !ok {
		v.add("date", "The date field must be a valid date.")
	}
	var order *int
	if s := sv(orderIn); strings.TrimSpace(s) != "" {
		if n, ok := isIntLike(orderIn); ok {
			order = &n
		} else {
			v.add("order", "The order field must be an integer.")
		}
	}
	if !slices.Contains(ec.avail, ec.bpHandle) {
		v.add("blueprint", "The selected blueprint is invalid.")
	}
	clean, err := navigation.Validate(m.a, ec.site, ec.b.Fields, in)
	if first, all, ok := navigation.FieldErrsOf(err); ok {
		for k, msg := range all {
			v.add(k, msg)
		}
		_ = first
	} else if err != nil {
		navigation.Reply(w, nil, err)
		return
	}
	if c.SchemaEnabled {
		for k, msg := range sitesettings.ValidateBlocks(blocks) {
			v.add(k, msg)
		}
	}
	var seo map[string]any
	if c.SeoEnabled {
		seo = m.seoValidate(v, seoIn)
	}
	if len(v.all) > 0 {
		navigation.Reply(w, nil, navigation.FieldErrs(v.first, v.all))
		return
	}

	// "published" tetap published walau tanggalnya di masa depan; "scheduled" hanya bila dipilih eksplisit.
	cur := "draft"
	if e != nil && e.Status != "" {
		cur = e.Status
	}
	if status != cur && !canPublish(u, c.Handle, e) {
		navigation.Reply(w, nil, navigation.ErrForbidden)
		return
	}

	clean["title"] = title
	if !ec.isOrigin {
		var od map[string]any
		oe := m.originEntry(ec)
		if oe != nil {
			od = oe.Data
		}
		overlay(ec.b.Fields, clean, navigation.Fill(ec.b.Fields, od))
		if c.SeoEnabled {
			os := defaultSeo()
			if oe != nil {
				os = oe.seoFields()
			}
			for k, loc := range seoLocalizable {
				if !loc {
					seo[k] = os[k]
				}
			}
		}
	}

	// Entry root pertama sebuah site (structured + orderable) jadi homepage lewat order = 1.
	if order == nil && e == nil && c.Structured && c.Orderable && parentID == "" {
		var n int
		m.a.DB.QueryRow("SELECT COUNT(*) FROM entries WHERE collection=? AND site=? AND `order`=1", c.Handle, ec.site).Scan(&n)
		if n == 0 {
			one := 1
			order = &one
		}
	}

	created := e == nil
	prevStatus := ""
	if created {
		e = &entry{ID: web.UUID(), Site: ec.site, Collection: c.Handle}
		if o := ec.originID; o != "" {
			if _, err := m.entry(o, c.Handle); err == nil {
				e.Origin = o
			}
		}
	} else {
		prevStatus = e.Status
	}
	e.Blueprint, e.Slug, e.Status, e.Published = ec.bpHandle, slug, status, status == "published"
	e.Date, e.Order, e.Data = strings.ReplaceAll(strings.TrimSpace(date), "T", " "), order, clean
	if c.SchemaEnabled {
		e.Schema, e.hasSchema = sitesettings.CleanBlocks(blocks), true
	}
	if c.SeoEnabled {
		e.Seo, e.hasSeo = seo, true
	}
	if created {
		err = m.insert(e)
	} else {
		err = m.update(e)
	}
	if err != nil {
		navigation.Reply(w, nil, err)
		return
	}
	if c.Structured {
		var parent *string
		if parentID != "" {
			parent = &parentID
		}
		if err := m.setParent(c.Handle, ec.site, e.ID, parent, c.MaxDepth); err != nil {
			if created { // beda dari versi lama: entry baru yang gagal masuk tree tidak dibiarkan menggantung (simpan ulang membuat duplikat)
				m.remove(e)
			}
			navigation.Reply(w, nil, navigation.FieldErrs(err.Error(), map[string]string{"parentId": err.Error()}))
			return
		}
	}
	m.setURI(c, e)
	m.syncSitemap(e)

	action := "updated"
	if created {
		action = "created"
	}
	if !created && prevStatus != status && (status == "published" || status == "scheduled") {
		action = status
	}
	m.log(r, action, "entry", e.label())
	web.Flash(w, "Entry saved.")
	navigation.Reply(w, map[string]any{"ok": true, "id": e.ID, "site": e.Site, "blueprint": e.Blueprint, "saveAction": c.saveAction()}, nil)
}
