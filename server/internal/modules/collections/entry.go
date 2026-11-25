package collections

// = Models\Entry + EntryUrlResolver + SitemapSyncer + entries:publish-scheduled.

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"draftcms-lite/internal/modules/sitesettings"
	"draftcms-lite/internal/web"
)

type entry struct {
	ID, Site, Collection, Status string
	Slug, URI, Date, Blueprint   string
	Origin                       string // "" = bukan terjemahan
	UpdatedAt                    string
	Published                    bool
	Order                        *int
	Data, Seo                    map[string]any
	Schema                       []sitesettings.Block
	hasSchema, hasSeo            bool
}

const entryCols = "id,site,origin_id,published,status,slug,uri,`date`,`order`,collection,blueprint,data,`schema`,seo,updated_at"

type scanner interface{ Scan(...any) error }

func scanEntry(r scanner) (*entry, error) {
	e := &entry{}
	var origin, slug, uri, date, bp, upd sql.NullString
	var order sql.NullInt64
	var data, schema, seo []byte
	if err := r.Scan(&e.ID, &e.Site, &origin, &e.Published, &e.Status, &slug, &uri, &date, &order, &e.Collection, &bp, &data, &schema, &seo, &upd); err != nil {
		return nil, err
	}
	e.Origin, e.Slug, e.URI, e.Date, e.Blueprint, e.UpdatedAt = origin.String, slug.String, uri.String, date.String, bp.String, upd.String
	if order.Valid {
		o := int(order.Int64)
		e.Order = &o
	}
	json.Unmarshal(data, &e.Data)
	if e.Data == nil {
		e.Data = map[string]any{}
	}
	if len(schema) > 0 && string(schema) != "null" {
		e.hasSchema = true
		json.Unmarshal(schema, &e.Schema)
	}
	if len(seo) > 0 && string(seo) != "null" {
		e.hasSeo = true
		json.Unmarshal(seo, &e.Seo)
	}
	return e, nil
}

func (m *mod) entry(id, collection string) (*entry, error) {
	return scanEntry(m.a.DB.QueryRow(`SELECT `+entryCols+` FROM entries WHERE id=? AND collection=?`, id, collection))
}

func (m *mod) entryList(q string, args ...any) []*entry {
	rows, err := m.a.DB.Query(`SELECT `+entryCols+` FROM entries `+q, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []*entry
	for rows.Next() {
		if e, err := scanEntry(rows); err == nil {
			out = append(out, e)
		}
	}
	return out
}

func (e *entry) title() string { s, _ := e.Data["title"].(string); return s }

// label = Entry::activityLabel().
func (e *entry) label() string {
	t := e.title()
	if t == "" {
		t = e.Slug
	}
	if t == "" {
		t = e.ID
	}
	return "entry '" + t + "' (" + e.Collection + ")"
}

// sv = (string) ala PHP untuk nilai JSON.
func sv(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "1"
		}
		return ""
	}
	return fmt.Sprint(v)
}

/* ---------- SEO ---------- */

func defaultSeo() map[string]any {
	return map[string]any{
		"meta_title": "", "meta_description": "", "meta_canonical": "", "robots": []any{"noindex", "nofollow"},
		"meta_featured_image": []any{}, "meta_author": "", "sitemap_priority": "0.8", "sitemap_changefreq": "weekly",
	}
}

// seoFields = Entry::seoFields(): default digabung dengan yang tersimpan.
func (e *entry) seoFields() map[string]any {
	out := defaultSeo()
	for k, v := range e.Seo {
		out[k] = v
	}
	return out
}

// seoLocalizable = EntryForm::SEO_LOCALIZABLE.
var seoLocalizable = map[string]bool{"meta_title": true, "meta_description": true, "meta_canonical": true, "robots": true,
	"meta_featured_image": false, "meta_author": false, "sitemap_priority": true, "sitemap_changefreq": true}

var (
	seoRobots      = []string{"index", "follow", "noindex", "nofollow", "noarchive", "noimageindex", "nosnippet"}
	seoRobotLabels = map[string]string{"index": "Index", "follow": "Follow", "noindex": "No Index", "nofollow": "No Follow", "noarchive": "No Archive", "noimageindex": "No Image Index", "nosnippet": "No Snippet"}
	seoPriorities  = []string{"0.0", "0.1", "0.2", "0.3", "0.4", "0.5", "0.6", "0.7", "0.8", "0.9", "1.0"}
	seoChangefreqs = []string{"always", "hourly", "daily", "weekly", "monthly", "yearly", "never"}
)

/* ---------- izin (Entry::canBeEditedBy / canBeDeletedBy / ownedBy) ---------- */

func (e *entry) ownedBy(u *web.User) bool {
	a := sv(e.seoFields()["meta_author"])
	return a == "" || (u != nil && a == u.ID)
}

func (e *entry) canEdit(u *web.User) bool {
	return u.Can("edit "+e.Collection+" entries") && (e.ownedBy(u) || u.Can("edit other authors "+e.Collection+" entries"))
}

func (e *entry) canDelete(u *web.User) bool {
	return u.Can("delete "+e.Collection+" entries") && (e.ownedBy(u) || u.Can("delete other authors "+e.Collection+" entries"))
}

// canPublish = canEditStatusOf(): mengubah status butuh "publish {h} entries" (+ "publish other authors" untuk entry orang lain).
func canPublish(u *web.User, handle string, e *entry) bool {
	return u.Can("publish "+handle+" entries") && (e == nil || e.ownedBy(u) || u.Can("publish other authors "+handle+" entries"))
}

/* ---------- tanggal ---------- */

// shortcut: tanggal entry dianggap UTC (app.timezone default), upgrade bila app punya zona waktu sendiri.
var dateLayouts = []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02"}

func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func isFuture(s string) bool { t, ok := parseDate(s); return ok && t.After(time.Now().UTC()) }

/* ---------- URL (EntryUrlResolver) ---------- */

// path = path publik entry tanpa domain; ok=false bila collection tak punya route di site itu. Entry order=1 = homepage ("").
func (m *mod) path(c *col, e *entry) (string, bool) { return m.pathDepth(c, e, 0) }

func (m *mod) pathDepth(c *col, e *entry, depth int) (string, bool) {
	tpl := c.routeForSite(e.Site)
	if tpl == "" {
		return "", false
	}
	if e.Order != nil && *e.Order == 1 {
		return "", true
	}
	var y, mo, d string
	if t, ok := parseDate(e.Date); ok {
		y, mo, d = t.Format("2006"), t.Format("01"), t.Format("02")
	}
	parent := ""
	if depth < 50 { // tree tidak boleh siklik; batas ini hanya jaring pengaman
		if pid := m.parentMap(c.Handle, e.Site)[e.ID]; pid != "" {
			if pe, err := m.entry(pid, c.Handle); err == nil {
				parent, _ = m.pathDepth(c, pe, depth+1)
			}
		}
	}
	p := strings.NewReplacer("{slug}", e.Slug, "{year}", y, "{month}", mo, "{day}", d, "{parent_uri}", parent).Replace(tpl)
	return strings.Trim(p, "/"), true
}

// url = URL penuh (URL site + path); "" bila tak ada route atau site tidak ada.
func (m *mod) url(c *col, e *entry) string {
	p, ok := m.path(c, e)
	if !ok {
		return ""
	}
	var siteURL string
	if m.a.DB.QueryRow(`SELECT url FROM sites WHERE handle=?`, e.Site).Scan(&siteURL) != nil {
		return ""
	}
	base := strings.TrimRight(siteURL, "/")
	if !strings.HasPrefix(strings.ToLower(siteURL), "http://") && !strings.HasPrefix(strings.ToLower(siteURL), "https://") {
		base = strings.TrimRight(os.Getenv("APP_URL"), "/") + base
	}
	if p == "" {
		if base == "" {
			return "/"
		}
		return base
	}
	return base + "/" + p
}

/* ---------- simpan + efek samping ---------- */

func jsonOrNil(has bool, v any) any {
	if !has {
		return nil
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// insert / update menulis semua kolom, lalu menyinkronkan sitemap (Entry::booted saved).
func (m *mod) insert(e *entry) error {
	data, _ := json.Marshal(e.Data)
	var origin, order any
	if e.Origin != "" {
		origin = e.Origin
	}
	if e.Order != nil {
		order = *e.Order
	}
	_, err := m.a.DB.Exec(`INSERT INTO entries(`+entryCols[:len(entryCols)-len(",updated_at")]+`,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.ID, e.Site, origin, e.Published, e.Status, nullStr(e.Slug), nullStr(e.URI), nullStr(e.Date), order, e.Collection, nullStr(e.Blueprint), string(data),
		jsonOrNil(e.hasSchema, nonNilBlocks(e.Schema)), jsonOrNil(e.hasSeo, e.Seo), web.Now(), web.Now())
	if err == nil {
		e.UpdatedAt = web.Now()
		m.syncSitemap(e)
	}
	return err
}

func (m *mod) update(e *entry) error {
	data, _ := json.Marshal(e.Data)
	var order any
	if e.Order != nil {
		order = *e.Order
	}
	_, err := m.a.DB.Exec("UPDATE entries SET published=?,status=?,slug=?,uri=?,`date`=?,`order`=?,blueprint=?,data=?,`schema`=?,seo=?,updated_at=? WHERE id=?",
		e.Published, e.Status, nullStr(e.Slug), nullStr(e.URI), nullStr(e.Date), order, nullStr(e.Blueprint), string(data),
		jsonOrNil(e.hasSchema, nonNilBlocks(e.Schema)), jsonOrNil(e.hasSeo, e.Seo), web.Now(), e.ID)
	if err == nil {
		e.UpdatedAt = web.Now()
		m.syncSitemap(e)
	}
	return err
}

// remove = Entry::delete + deleted event (hapus baris sitemap).
func (m *mod) remove(e *entry) error {
	_, err := m.a.DB.Exec(`DELETE FROM entries WHERE id=?`, e.ID)
	if err == nil {
		m.removeSitemap(e)
	}
	return err
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nonNilBlocks(b []sitesettings.Block) []sitesettings.Block {
	if b == nil {
		return []sitesettings.Block{}
	}
	return b
}

// setURI = $entry->update(['uri' => path]) tanpa memicu sync kedua.
func (m *mod) setURI(c *col, e *entry) {
	p, ok := m.path(c, e)
	e.URI = ""
	if ok {
		e.URI = p
	}
	// "" dan NULL berbeda: homepage punya uri "" (bukan NULL)
	var v any
	if ok {
		v = p
	}
	m.a.DB.Exec(`UPDATE entries SET uri=? WHERE id=?`, v, e.ID)
}

// resyncURIs menghitung ulang uri semua entry collection. Pengganti fallback findEntry di source: Astro hanya mencocokkan kolom uri.
// shortcut: O(n) query per simpan, ubah ke turunan entry yang berubah saja bila entry sudah ribuan.
func (m *mod) resyncURIs(c *col) {
	for _, e := range m.entryList("WHERE collection=?", c.Handle) {
		m.setURI(c, e)
	}
}

/* ---------- SitemapSyncer ---------- */

func (m *mod) syncSitemap(e *entry) {
	c, err := m.col(e.Collection)
	url := ""
	if err == nil {
		url = m.url(c, e)
	}
	seoOn := err == nil && c.SeoEnabled
	robots, _ := e.seoFields()["robots"].([]any)
	noindexed := seoOn && slices.ContainsFunc(robots, func(x any) bool { return sv(x) == "noindex" })
	if err != nil || url == "" || e.Status != "published" || noindexed {
		m.removeSitemap(e)
		return
	}
	priority, changefreq := 0.5, "weekly"
	if seoOn {
		s := e.seoFields()
		if f, err := strconv.ParseFloat(sv(s["sitemap_priority"]), 64); err == nil {
			priority = f
		}
		if cf := sv(s["sitemap_changefreq"]); cf != "" {
			changefreq = cf
		}
	}
	m.a.DB.Exec(`INSERT INTO sitemaps(site,collection,origin_id,url,changefreq,priority,lastmod,is_active,created_at,updated_at) VALUES(?,?,?,?,?,?,?,1,?,?)
		ON DUPLICATE KEY UPDATE url=VALUES(url),changefreq=VALUES(changefreq),priority=VALUES(priority),lastmod=VALUES(lastmod),is_active=1,updated_at=VALUES(updated_at)`,
		e.Site, e.Collection, e.ID, url, changefreq, math.Round(priority*10)/10, web.Now(), web.Now(), web.Now())
}

func (m *mod) removeSitemap(e *entry) {
	m.a.DB.Exec(`DELETE FROM sitemaps WHERE site=? AND collection=? AND origin_id=?`, e.Site, e.Collection, e.ID)
}

/* ---------- entries:publish-scheduled ---------- */

// PublishScheduled membalik "scheduled" yang tanggalnya sudah lewat jadi "published"; mengembalikan jumlahnya.
func PublishScheduled(a *web.App) int {
	m := &mod{a}
	n := 0
	for _, e := range m.entryList(`WHERE status='scheduled'`) {
		if e.Date != "" && !isFuture(e.Date) {
			e.Status, e.Published = "published", true
			if m.update(e) == nil {
				n++
			}
		}
	}
	return n
}

// startScheduler = scheduler Laravel (tiap menit). Goroutine berhenti bersama proses.
func (m *mod) startScheduler() {
	go func() {
		for range time.Tick(time.Minute) {
			PublishScheduled(m.a)
		}
	}()
}
