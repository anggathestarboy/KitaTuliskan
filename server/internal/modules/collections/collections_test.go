package collections

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"draftcms-lite/internal/db"
	"draftcms-lite/internal/web"
)

// Butuh MySQL lokal; dilewati kalau tidak ada. Alur: collection → entry (SEO/schema) → terjemahan → tree/max depth → reorder/homepage
// → duplikat → hapus → collection flat (bulk status, scheduled) → sitemap.
func TestCollectionsLifecycle(t *testing.T) {
	d, err := db.Open("127.0.0.1", "3306", "root", "", "draftcms_lite_test_collections", true)
	if err != nil {
		t.Skip("MySQL tidak tersedia:", err)
	}
	t.Cleanup(func() { d.Exec("DROP DATABASE draftcms_lite_test_collections") })
	a := web.New(d, "admin-panel", "../../../templates")
	mux := http.NewServeMux()
	m := &mod{a}
	m.registerManager(mux)
	m.registerEntries(mux)
	m.registerForm(mux)

	d.Exec(`INSERT INTO users(id,name,email,email_verified_at,password) VALUES('u1','Root','r@x.test',NOW(),'x')`)
	d.Exec(`INSERT INTO roles(id,handle,title,permissions) VALUES(1,'super','Super','["super"]')`)
	d.Exec(`INSERT INTO role_user VALUES(1,'u1')`)
	d.Exec("INSERT INTO sites(handle,name,url,locale,lang,attributes,`order`) VALUES('en','English','/','en_US','en','{}',1),('id','Indonesia','/id','id_ID','id','{}',2)")
	d.Exec(`INSERT INTO asset_containers(handle,title,disk) VALUES('assets','Assets','local')`)
	d.Exec(`INSERT INTO assets_meta(container,folder,basename,filename,extension,path) VALUES('assets','/','a.png','a','png','a.png')`)
	rec := httptest.NewRecorder()
	a.Login(rec, httptest.NewRequest("GET", "/", nil), "u1")
	cookie := rec.Result().Cookies()[0]

	call := func(method, path string, body any) (int, map[string]any) {
		var rd *strings.Reader
		if s, ok := body.(string); ok {
			rd = strings.NewReader(s)
		} else {
			b, _ := json.Marshal(body)
			rd = strings.NewReader(string(b))
		}
		r := httptest.NewRequest(method, "/admin-panel"+path, rd)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		a.LoadUser(mux).ServeHTTP(w, r)
		var out map[string]any
		json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	type J = map[string]any
	// Halaman HTML (template valid + custom element terpasang) untuk keempat route.
	d.Exec(`INSERT INTO collections(id,handle,title,settings,created_at,updated_at) VALUES(900,'pg','Pg','{}',NOW(),NOW())`)
	for path, tag := range map[string]string{"/collections": "<collections-manager", "/collections/pg": "<collections-entries", "/collections/pg/entries/create/en": "<collections-entry", "/collections/pg/entries/x": "<collections-entry"} {
		r := httptest.NewRequest("GET", "/admin-panel"+path, nil)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		a.LoadUser(mux).ServeHTTP(w, r)
		if w.Code != 200 || !strings.Contains(w.Body.String(), tag) {
			t.Fatalf("%s: %d tanpa %s", path, w.Code, tag)
		}
	}
	d.Exec(`DELETE FROM collections WHERE id=900`)
	d.Exec(`ALTER TABLE collections AUTO_INCREMENT=1`) // id test lain mengandalkan urutan mulai dari 1
	seo := J{"meta_title": "T", "meta_description": "D", "meta_canonical": "", "robots": []string{"index", "follow"}, "meta_featured_image": []int{1}, "meta_author": "u1", "sitemap_priority": "0.8", "sitemap_changefreq": "weekly"}
	save := func(h string, body J) (int, J) { return call("POST", "/collections/api/entry/"+h, body) }
	str := func(q string, args ...any) (s string) { d.QueryRow(q, args...).Scan(&s); return }
	num := func(q string, args ...any) (n int) { d.QueryRow(q, args...).Scan(&n); return }

	// ---- collection ----
	col := J{"title": "Pages", "handle": "pages", "routeMode": "single", "route": "{parent_uri}/{slug}", "sites": []string{"en", "id"}, "structured": true, "orderable": true,
		"visibleOrder": true, "schemaEnabled": true, "seoEnabled": true, "maxDepth": "2", "saveAction": "editing"}
	bad := J{"title": "X", "handle": "Bad Handle", "routeMode": "single", "saveAction": "listing"}
	if c, _ := call("POST", "/collections/api/save", bad); c != 422 {
		t.Fatal("handle tidak valid harus ditolak")
	}
	if c, _ := call("POST", "/collections/api/save", J{"title": "X", "handle": "x", "routeMode": "single", "maxDepth": "0", "saveAction": "listing"}); c != 422 {
		t.Fatal("max depth < 1 harus ditolak")
	}
	if c, o := call("POST", "/collections/api/save", col); c != 200 {
		t.Fatalf("buat collection: %d %v", c, o)
	}
	_, o := call("GET", "/collections/api/list", "")
	cs := o["collections"].([]any)[0].(J)
	if cs["maxDepth"] != "2" || cs["structured"] != true || len(cs["sites"].([]any)) != 2 || cs["saveAction"] != "editing" {
		t.Fatalf("daftar collection: %v", cs)
	}
	if c, _ := call("POST", "/collections/api/save", J{"id": 1, "title": "Pages", "handle": "renamed", "routeMode": "single", "saveAction": "listing"}); c != 422 {
		t.Fatal("ganti handle harus ditolak")
	}
	d.Exec(`INSERT INTO blueprints(namespace,handle,data) VALUES('collections.pages','pages','{"tabs":[{"handle":"main","display":"Main"}],"fields":[{"handle":"color","type":"text","tab":"main"},{"handle":"blurb","type":"text","tab":"main","localizable":true}]}')`)

	// ---- entry: validasi ----
	base := J{"site": "en", "blueprint": "pages", "title": "Home", "slug": "home", "status": "published", "date": "2020-01-01T10:00", "data": J{"color": "blue", "blurb": "Hi"}, "seo": seo, "schemaBlocks": []J{}}
	with := func(kv J) J {
		o := J{}
		for k, v := range base {
			o[k] = v
		}
		for k, v := range kv {
			o[k] = v
		}
		return o
	}
	if c, o := save("pages", with(J{"title": ""})); c != 422 || o["errors"].(J)["title"] == nil {
		t.Fatalf("judul wajib: %d %v", c, o)
	}
	if c, o := save("pages", with(J{"seo": J{}})); c != 422 || o["errors"].(J)["seo.meta_title"] == nil || o["errors"].(J)["seo.meta_author"] == nil {
		t.Fatalf("SEO wajib: %d %v", c, o)
	}
	if c, o := save("pages", with(J{"schemaBlocks": []J{{"type": "Nope"}}})); c != 422 || o["errors"].(J)["schemaBlocks.0.type"] == nil {
		t.Fatalf("tipe schema: %d %v", c, o)
	}

	// ---- entry: root pertama = homepage (order 1), uri kosong, masuk sitemap ----
	c, o := save("pages", base)
	if c != 200 || o["saveAction"] != "editing" {
		t.Fatalf("buat entry: %d %v", c, o)
	}
	home := o["id"].(string)
	if num("SELECT `order` FROM entries WHERE id=?", home) != 1 || num("SELECT COUNT(*) FROM entries WHERE id=? AND uri=''", home) != 1 {
		t.Fatal("entry root pertama harus order 1 dengan uri kosong")
	}
	if str("SELECT url FROM sitemaps WHERE origin_id=?", home) != "/" {
		t.Fatal("homepage harus masuk sitemap dengan url /")
	}

	// ---- tree + max depth (2) ----
	c, o = save("pages", with(J{"title": "About", "slug": "about", "parentId": home, "order": ""}))
	if c != 200 {
		t.Fatalf("anak: %d %v", c, o)
	}
	about := o["id"].(string)
	if str("SELECT uri FROM entries WHERE id=?", about) != "about" || str("SELECT url FROM sitemaps WHERE origin_id=?", about) != "/about" {
		t.Fatal("uri anak = {parent_uri}/{slug} tanpa slash di ujung")
	}
	if c, o := save("pages", with(J{"title": "Deep", "slug": "deep", "parentId": about})); c != 422 || o["errors"].(J)["parentId"] == nil || num("SELECT COUNT(*) FROM entries WHERE slug='deep'") != 0 {
		t.Fatalf("max depth harus ditolak dan entry baru dibatalkan: %d %v", c, o)
	}

	// ---- slug induk berubah → uri anak ikut (homepage selalu "", jadi dites lewat anak biasa) ----
	d.Exec("UPDATE entries SET `order`=3 WHERE id=?", home)
	if c, o := save("pages", with(J{"id": home, "slug": "start", "order": 3})); c != 200 || str("SELECT uri FROM entries WHERE id=?", about) != "start/about" {
		t.Fatalf("uri anak harus ikut slug induk: %d %v", c, o)
	}
	if c, o := save("pages", with(J{"id": home, "order": 1})); c != 200 || str("SELECT uri FROM entries WHERE id=?", about) != "about" {
		t.Fatalf("kembali jadi homepage: %d %v", c, o)
	}

	// ---- terjemahan: field non-localizable mengikuti origin, localizable boleh beda ----
	c, o = call("GET", "/collections/api/entry/pages?site=id&origin="+home+"&blueprint=pages", "")
	if c != 200 || o["isOrigin"] != false || o["data"].(J)["blurb"] != "Hi" || o["seo"].(J)["meta_author"] != "u1" {
		t.Fatalf("muat terjemahan baru: %d %v", c, o)
	}
	c, o = save("pages", with(J{"site": "id", "origin": home, "title": "Beranda", "slug": "beranda", "data": J{"color": "red", "blurb": "Halo"}}))
	if c != 200 {
		t.Fatalf("simpan terjemahan: %d %v", c, o)
	}
	idEntry := o["id"].(string)
	_, o = call("GET", "/collections/api/entry/pages?id="+idEntry, "")
	if o["data"].(J)["color"] != "blue" || o["data"].(J)["blurb"] != "Halo" || o["entry"].(J)["origin"] != home {
		t.Fatalf("color harus ikut origin: %v", o)
	}
	if l := o["sites"].([]any); l[0].(J)["href"] != "/admin-panel/collections/pages/entries/"+home {
		t.Fatalf("link site: %v", l)
	}
	// "published" dengan date masa depan tetap published (tidak otomatis scheduled)
	c, o = save("pages", with(J{"title": "Future", "slug": "future", "date": "2999-01-01T10:00"}))
	if c != 200 || str("SELECT status FROM entries WHERE id=?", o["id"]) != "published" {
		t.Fatal("tanggal depan harus tetap 'published'")
	}
	future := o["id"].(string)

	// ---- daftar tree ----
	_, o = call("GET", "/collections/api/entry-list/pages?site=en", "")
	roots := o["entries"].([]any)
	if len(roots) < 2 || roots[0].(J)["id"] != home || len(roots[0].(J)["children"].([]any)) != 1 {
		t.Fatalf("tree: %v", o["entries"])
	}

	// ---- reorder: c ke posisi 0 jadi homepage ----
	if c, _ := call("POST", "/collections/api/entry-reorder/pages", J{"id": future, "position": 0, "parentId": nil, "site": "en"}); c != 200 {
		t.Fatal("reorder")
	}
	if num("SELECT `order` FROM entries WHERE id=?", future) != 1 || num("SELECT `order` FROM entries WHERE id=?", home) != 2 {
		t.Fatal("urutan root setelah reorder")
	}
	if c, _ := call("POST", "/collections/api/entry-reorder/pages", J{"id": about, "position": 0, "parentId": home, "site": "en"}); c != 200 || num("SELECT `order` FROM entries WHERE id=?", about) != 2 {
		t.Fatal("anak mulai dari order 2")
	}

	// ---- duplikat (entry + terjemahannya) ----
	before := num("SELECT COUNT(*) FROM entries")
	if c, o := call("POST", "/collections/api/entry-duplicate/pages", J{"id": home}); c != 200 {
		t.Fatalf("duplikat: %d %v", c, o)
	}
	if num("SELECT COUNT(*) FROM entries") != before+2 || num("SELECT COUNT(*) FROM entries WHERE slug='home-copy' AND status='draft'") != 1 {
		t.Fatal("duplikat harus membuat salinan draft untuk tiap bahasa")
	}
	if num("SELECT COUNT(*) FROM entries WHERE slug='beranda-copy' AND origin_id IS NOT NULL") != 1 {
		t.Fatal("salinan terjemahan harus menunjuk origin salinan")
	}

	// ---- hapus beserta terjemahan ----
	if c, _ := call("POST", "/collections/api/entry-delete/pages", J{"id": home, "deleteLocales": true}); c != 200 {
		t.Fatal("hapus")
	}
	if num("SELECT COUNT(*) FROM entries WHERE id IN (?,?)", home, idEntry) != 0 || num("SELECT COUNT(*) FROM sitemaps WHERE origin_id=?", home) != 0 {
		t.Fatal("entry, terjemahan, dan baris sitemap harus ikut terhapus")
	}
	if c, o := call("POST", "/collections/api/delete", J{"id": 1}); c != 422 || !strings.Contains(o["error"].(string), "it still has") {
		t.Fatalf("collection berisi entry tidak boleh dihapus: %d %v", c, o)
	}

	// ---- collection flat: bulk status + scheduled ----
	if c, o := call("POST", "/collections/api/save", J{"title": "Posts", "handle": "posts", "routeMode": "single", "route": "blog/{slug}", "sites": []string{"en", "id"}, "visibleOrder": true, "saveAction": "listing"}); c != 200 {
		t.Fatalf("collection flat: %d %v", c, o)
	}
	d.Exec(`INSERT INTO blueprints(namespace,handle,data) VALUES('collections.posts','posts','{"tabs":[{"handle":"main","display":"Main"}],"fields":[]}')`)
	var ids []string
	for _, s := range []string{"a", "b"} {
		c, o := save("posts", J{"site": "en", "blueprint": "posts", "title": s, "slug": s, "status": "draft", "date": "2020-01-01T10:00", "data": J{}})
		if c != 200 {
			t.Fatalf("post %s: %d %v", s, c, o)
		}
		ids = append(ids, o["id"].(string))
	}
	if c, o := call("POST", "/collections/api/entry-bulk/posts", J{"action": "status", "status": "published", "ids": ids}); c != 200 || o["count"] != float64(2) {
		t.Fatalf("bulk status: %d %v", c, o)
	}
	if num("SELECT COUNT(*) FROM entries WHERE collection='posts' AND status='published' AND published=1") != 2 || str("SELECT url FROM sitemaps WHERE origin_id=?", ids[0]) != "/blog/a" {
		t.Fatal("bulk publish harus mengisi sitemap")
	}
	if c, o := call("POST", "/collections/api/entry-bulk/posts", J{"action": "status", "status": "published", "ids": ids}); c != 200 || o["count"] != float64(2) {
		t.Fatalf("bulk ulang: %d %v", c, o)
	}
	_, o = call("GET", "/collections/api/entry-list/posts", "")
	if o["bulkAvailable"] != true || len(o["entries"].([]any)) != 2 {
		t.Fatalf("daftar flat: %v", o)
	}
	// route diubah / dikosongkan setelah entry ada → uri ikut
	flat := J{"id": 2, "title": "Posts", "handle": "posts", "routeMode": "single", "route": "news/{slug}", "sites": []string{"en", "id"}, "visibleOrder": true, "saveAction": "listing"}
	if c, o := call("POST", "/collections/api/save", flat); c != 200 || str("SELECT uri FROM entries WHERE id=?", ids[0]) != "news/a" {
		t.Fatalf("ganti route harus menghitung ulang uri: %d %v", c, o)
	}
	flat["route"] = ""
	if c, _ := call("POST", "/collections/api/save", flat); c != 200 || num("SELECT COUNT(*) FROM entries WHERE collection='posts' AND uri IS NULL") != 2 {
		t.Fatal("tanpa route uri harus NULL")
	}
	d.Exec(`UPDATE entries SET status='scheduled',published=0,`+"`date`"+`='2000-01-01 00:00' WHERE id=?`, ids[0])
	if n := PublishScheduled(a); n != 1 || str("SELECT status FROM entries WHERE id=?", ids[0]) != "published" {
		t.Fatalf("scheduled harus terbit: %d", n)
	}
	if c, _ := call("POST", "/collections/api/entry-bulk/posts", J{"action": "delete", "ids": ids}); c != 200 || num("SELECT COUNT(*) FROM entries WHERE collection='posts'") != 0 {
		t.Fatal("bulk delete")
	}
	if c, _ := call("POST", "/collections/api/delete", J{"id": 2}); c != 200 {
		t.Fatal("hapus collection kosong")
	}
	_ = fmt.Sprint
}
