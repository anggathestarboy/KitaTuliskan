package globals

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"draftcms-lite/internal/db"
	"draftcms-lite/internal/web"
)

// Butuh MySQL lokal; dilewati kalau tidak ada. Alur: buat set (+blueprint) → nilai per bahasa → hapus.
func TestGlobalSetLifecycle(t *testing.T) {
	d, err := db.Open("127.0.0.1", "3306", "root", "", "draftcms_lite_test_globals", true)
	if err != nil {
		t.Skip("MySQL tidak tersedia:", err)
	}
	t.Cleanup(func() { d.Exec("DROP DATABASE draftcms_lite_test_globals") })
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

	if c, o := call("POST", "/globals/api/save", `{"title":"Site","handle":"Bad Handle","createBlueprint":true}`); c != 422 {
		t.Fatalf("handle tidak valid harus ditolak: %d %v", c, o)
	}
	if c, o := call("POST", "/globals/api/save", `{"title":"Site","handle":"site","createBlueprint":true,"sites":["id"],"saveAction":"editing"}`); c != 200 || o["blueprintCreated"] != true {
		t.Fatalf("buat set: %d %v", c, o)
	}
	var n int
	d.QueryRow(`SELECT COUNT(*) FROM global_set_variables WHERE handle='site' AND locale IN ('en','id')`).Scan(&n)
	if n != 2 { // default site (en) ikut otomatis walau tidak dicentang
		t.Fatalf("baris nilai per bahasa = %d, mau 2", n)
	}
	if c, _ := call("POST", "/globals/api/save", `{"id":1,"title":"Site","handle":"renamed","saveAction":"listing"}`); c != 422 {
		t.Fatal("ganti handle harus ditolak (FK ke global_set_variables)")
	}
	d.Exec(`UPDATE blueprints SET data='{"tabs":[{"handle":"main","display":"Main"}],"fields":[{"handle":"name","type":"text","tab":"main","localizable":true}]}' WHERE namespace='globals.site'`)
	if c, o := call("POST", "/globals/api/content/site?site=id", `{"data":{"name":"Situs"}}`); c != 200 {
		t.Fatalf("simpan nilai: %d %v", c, o)
	}
	c, o := call("GET", "/globals/api/content/site?site=id", "")
	if c != 200 || o["data"].(map[string]any)["name"] != "Situs" || o["activeSite"] != "id" || o["saveAction"] != "editing" {
		t.Fatalf("muat nilai: %d %v", c, o)
	}
	if c, _ := call("POST", "/globals/api/delete", `{"id":1}`); c != 200 {
		t.Fatal("hapus")
	}
	d.QueryRow(`SELECT COUNT(*) FROM global_set_variables`).Scan(&n)
	if n != 0 {
		t.Fatal("nilai harus ikut terhapus")
	}
}
