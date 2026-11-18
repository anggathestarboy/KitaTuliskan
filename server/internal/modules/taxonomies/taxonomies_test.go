package taxonomies

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"draftcms-lite/internal/db"
	"draftcms-lite/internal/web"
)

// Butuh MySQL lokal; dilewati kalau tidak ada. Alur: buat taxonomy (+blueprint) → term origin → terjemahan → hapus.
func TestTaxonomyLifecycle(t *testing.T) {
	d, err := db.Open("127.0.0.1", "3306", "root", "", "draftcms_lite_test_taxonomies", true)
	if err != nil {
		t.Skip("MySQL tidak tersedia:", err)
	}
	t.Cleanup(func() { d.Exec("DROP DATABASE draftcms_lite_test_taxonomies") })
	a := web.New(d, "admin-panel", "../../../templates")
	mux := http.NewServeMux()
	Register(mux, a)

	d.Exec(`INSERT INTO users(id,name,email,email_verified_at,password) VALUES('u1','Root','r@x.test',NOW(),'x')`)
	d.Exec(`INSERT INTO roles(id,handle,title,permissions) VALUES(1,'super','Super','["super"]')`)
	d.Exec(`INSERT INTO role_user VALUES(1,'u1')`)
	d.Exec("INSERT INTO sites(handle,name,url,locale,lang,attributes,`order`) VALUES('en','English','/','en_US','en','{}',1),('id','Indonesia','/id','id_ID','id','{}',2)")
	rec := httptest.NewRecorder()
	a.Login(rec, httptest.NewRequest("GET", "/", nil), "u1")
	cookie := rec.Result().Cookies()[0]

	call := func(method, path, body string) (int, map[string]any) {
		r := httptest.NewRequest(method, "/admin-panel"+path, strings.NewReader(body))
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		a.LoadUser(mux).ServeHTTP(w, r)
		var out map[string]any
		json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}

	if c, _ := call("POST", "/taxonomies/api/save", `{"title":"Tags","handle":"Bad Handle","createBlueprint":true,"saveAction":"listing"}`); c != 422 {
		t.Fatal("handle tidak valid harus ditolak")
	}
	if c, o := call("POST", "/taxonomies/api/save", `{"title":"Tags","handle":"tags","createBlueprint":true,"sites":["id"],"saveAction":"create_another"}`); c != 200 || o["blueprintCreated"] != true {
		t.Fatalf("buat taxonomy: %d %v", c, o)
	}
	if c, _ := call("POST", "/taxonomies/api/save", `{"id":1,"title":"Tags","handle":"renamed","saveAction":"listing"}`); c != 422 {
		t.Fatal("ganti handle harus ditolak (FK ke taxonomy_terms)")
	}
	d.Exec(`UPDATE blueprints SET data='{"tabs":[{"handle":"main","display":"Main"}],"fields":[{"handle":"color","type":"text","tab":"main"},{"handle":"blurb","type":"text","tab":"main","localizable":true}]}' WHERE namespace='taxonomies.tags'`)

	if c, _ := call("POST", "/taxonomies/api/term/tags", `{"site":"en","slug":"go","data":{}}`); c != 422 {
		t.Fatal("judul wajib")
	}
	c, o := call("POST", "/taxonomies/api/term/tags", `{"site":"en","title":"Go","slug":"go","data":{"color":"blue","blurb":"Lang"}}`)
	if c != 200 || o["saveAction"] != "create_another" {
		t.Fatalf("buat term origin: %d %v", c, o)
	}
	if c, _ := call("POST", "/taxonomies/api/term/tags", `{"site":"en","title":"Go 2","slug":"go","data":{}}`); c != 422 {
		t.Fatal("slug kembar di site yang sama harus ditolak")
	}
	// Terjemahan baru: field localizable terisi dari origin; non-localizable ikut origin walau dikirim lain.
	c, o = call("GET", "/taxonomies/api/term/tags?site=id&origin=1", "")
	if c != 200 || o["data"].(map[string]any)["blurb"] != "Lang" || o["isOrigin"] != false {
		t.Fatalf("muat terjemahan baru: %d %v", c, o)
	}
	if c, o := call("POST", "/taxonomies/api/term/tags", `{"site":"id","origin":1,"title":"Bahasa Go","slug":"go","data":{"color":"red","blurb":"Bahasa"}}`); c != 200 {
		t.Fatalf("buat terjemahan: %d %v", c, o)
	}
	c, o = call("GET", "/taxonomies/api/term/tags?id=2", "")
	if c != 200 || o["data"].(map[string]any)["color"] != "blue" || o["data"].(map[string]any)["blurb"] != "Bahasa" || o["term"].(map[string]any)["title"] != "Bahasa Go" {
		t.Fatalf("color non-localizable harus mengikuti origin: %d %v", c, o)
	}
	if sites := o["sites"].([]any); sites[0].(map[string]any)["href"] != "/admin-panel/taxonomies/tags/terms/1" {
		t.Fatalf("link site origin: %v", sites)
	}
	if c, o := call("POST", "/taxonomies/api/delete", `{"id":1}`); c != 422 || !strings.Contains(o["error"].(string), "2 terms") {
		t.Fatalf("taxonomy berisi term tidak boleh dihapus: %d %v", c, o)
	}
	call("POST", "/taxonomies/api/term-delete/tags", `{"id":1}`)
	call("POST", "/taxonomies/api/term-delete/tags", `{"id":2}`)
	if c, _ := call("POST", "/taxonomies/api/delete", `{"id":1}`); c != 200 {
		t.Fatal("hapus taxonomy")
	}
}
