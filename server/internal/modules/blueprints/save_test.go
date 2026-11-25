package blueprints

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"draftcms-lite/internal/db"
	"draftcms-lite/internal/web"
)

// Butuh MySQL lokal; dilewati kalau tidak ada. Simpan gagal harus 422 (200 dibuang Turbo, jadi user tidak melihat apa pun).
func TestFailedSaveIs422(t *testing.T) {
	d, err := db.Open("127.0.0.1", "3306", "root", "", "draftcms_lite_test_blueprints", true)
	if err != nil {
		t.Skip("MySQL tidak tersedia:", err)
	}
	t.Cleanup(func() { d.Exec("DROP DATABASE draftcms_lite_test_blueprints") })
	a := web.New(d, "admin-panel", "../../../templates")
	mux := http.NewServeMux()
	Register(mux, a)
	d.Exec(`INSERT INTO users(id,name,email,email_verified_at,password) VALUES('u1','Root','r@x.test',NOW(),'x')`)
	d.Exec(`INSERT INTO roles(id,handle,title,permissions) VALUES(1,'super','Super','["super"]')`)
	d.Exec(`INSERT INTO role_user VALUES(1,'u1')`)
	rec := httptest.NewRecorder()
	a.Login(rec, httptest.NewRequest("GET", "/", nil), "u1")
	cookie := rec.Result().Cookies()[0]
	for _, path := range []string{"/blueprints/save", "/fieldsets/save"} {
		r := httptest.NewRequest("POST", "/admin-panel"+path, strings.NewReader(url.Values{"state": {`{"handle":"","fields":[]}`}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		a.LoadUser(mux).ServeHTTP(w, r)
		res := w.Result()
		if res.StatusCode != 422 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") || !strings.Contains(w.Body.String(), "Add at least one field.") {
			t.Fatalf("%s: %d %s", path, res.StatusCode, res.Header.Get("Content-Type"))
		}
	}
}
