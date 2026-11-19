package users

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"draftcms-lite/internal/db"
	"draftcms-lite/internal/web"
)

// Butuh MySQL lokal; dilewati kalau tidak ada. Alur: role/group → user (izin bertingkat) → self-protection → registry dinamis.
func TestUsersLifecycle(t *testing.T) {
	d, err := db.Open("127.0.0.1", "3306", "root", "", "draftcms_lite_test_users", true)
	if err != nil {
		t.Skip("MySQL tidak tersedia:", err)
	}
	t.Cleanup(func() { d.Exec("DROP DATABASE draftcms_lite_test_users") })
	a := web.New(d, "admin-panel", "../../../templates")
	mux := http.NewServeMux()
	Register(mux, a)

	d.Exec(`INSERT INTO users(id,name,email,email_verified_at,password) VALUES('su','Root','r@x.test',NOW(),'x'),('ed','Ed','e@x.test',NOW(),'x')`)
	EnsureSuperAdminRole(a)
	d.Exec(`INSERT INTO roles(handle,title,permissions) VALUES('manager','Manager','["access cp","view users","edit users","create users"]')`)
	d.Exec(`INSERT INTO role_user SELECT id,'su' FROM roles WHERE handle='super_admin'`)
	d.Exec(`INSERT INTO role_user SELECT id,'ed' FROM roles WHERE handle='manager'`)
	d.Exec("INSERT INTO collections(handle,title,settings) VALUES('blog','Blog','{\"orderable\":true}')")

	as := func(id string) *http.Cookie {
		rec := httptest.NewRecorder()
		a.Login(rec, httptest.NewRequest("GET", "/", nil), id)
		return rec.Result().Cookies()[0]
	}
	su, ed := as("su"), as("ed")
	call := func(c *http.Cookie, method, path, body string) (int, map[string]any) {
		r := httptest.NewRequest(method, "/admin-panel"+path, strings.NewReader(body))
		r.AddCookie(c)
		w := httptest.NewRecorder()
		a.LoadUser(mux).ServeHTTP(w, r)
		var out map[string]any
		json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}

	// Registry: grup per-collection dibangun dari DB, orderable menambah "reorder".
	var found bool
	for _, g := range Groups(a) {
		if g.Group == "Collections" {
			found = slicesHas(flatten(g.Permissions), "reorder blog entries") && slicesHas(flatten(g.Permissions), "view blog entries")
		}
	}
	if !found {
		t.Fatal("registry harus memuat izin per-collection")
	}

	// Role: handle kembar ditolak; permission tak dikenal dibuang; super menyimpan ['super'].
	if c, o := call(su, "POST", "/roles/api/save", `{"title":"Editor","handle":"manager"}`); c != 422 || o["errors"] == nil {
		t.Fatalf("handle role kembar harus ditolak: %d %v", c, o)
	}
	if c, _ := call(su, "POST", "/roles/api/save", `{"title":"Editor","handle":"editor","permissions":["access cp","bogus perm","edit blog entries"]}`); c != 200 {
		t.Fatal("buat role")
	}
	var raw string
	d.QueryRow(`SELECT permissions FROM roles WHERE handle='editor'`).Scan(&raw)
	if raw != `["access cp", "edit blog entries"]` && raw != `["access cp","edit blog entries"]` {
		t.Fatalf("permission harus dipangkas ke registry: %s", raw)
	}
	if c, _ := call(ed, "GET", "/roles/api/list", ""); c != 403 {
		t.Fatal("role manager butuh 'edit roles'")
	}

	// User: manager boleh membuat tapi tidak boleh mengubah role user lain tanpa 'assign user roles'.
	if c, o := call(ed, "POST", "/users/api/save", `{"name":"Zed","email":"zed@x.test","password":"short","passwordMode":"admin"}`); c != 422 || o["errors"].(map[string]any)["password"] == nil {
		t.Fatalf("password min 8: %d %v", c, o)
	}
	if c, _ := call(ed, "POST", "/users/api/save", `{"name":"Zed","email":"zed@x.test","password":"password123","passwordMode":"admin","roleHandles":["editor"]}`); c != 200 {
		t.Fatal("buat user")
	}
	var zid string
	d.QueryRow(`SELECT id FROM users WHERE email='zed@x.test'`).Scan(&zid)
	if c, _ := call(ed, "POST", "/users/api/save", `{"id":"`+zid+`","name":"Zed","email":"zed@x.test","roleHandles":[]}`); c != 403 {
		t.Fatal("ubah role butuh 'assign user roles'")
	}
	if c, _ := call(ed, "POST", "/users/api/save", `{"id":"`+zid+`","name":"Zed 2","email":"zed@x.test","password":"newpassword1","roleHandles":["editor"]}`); c != 403 {
		t.Fatal("ubah password butuh 'change passwords'")
	}
	if c, _ := call(ed, "POST", "/users/api/ban", `{"id":"`+zid+`","ban":true}`); c != 403 {
		t.Fatal("ban butuh 'ban users'")
	}

	// Role yang masih dipakai tidak boleh dihapus.
	var rid string
	d.QueryRow(`SELECT id FROM roles WHERE handle='editor'`).Scan(&rid)
	if c, o := call(su, "POST", "/roles/api/delete", `{"id":`+rid+`}`); c != 422 || !strings.Contains(o["error"].(string), "1 user") {
		t.Fatalf("role terpakai: %d %v", c, o)
	}

	// Self-protection + ban/unban + hapus.
	if c, _ := call(su, "POST", "/users/api/delete", `{"id":"su"}`); c != 422 {
		t.Fatal("tidak boleh menghapus akun sendiri")
	}
	if c, _ := call(su, "POST", "/users/api/ban", `{"id":"`+zid+`","ban":true}`); c != 200 {
		t.Fatal("ban")
	}
	if c, _ := call(su, "POST", "/users/api/bulk", `{"action":"delete","ids":["su","`+zid+`"]}`); c != 200 {
		t.Fatal("bulk delete")
	}
	var n int
	d.QueryRow(`SELECT COUNT(*) FROM users WHERE id IN ('su',?)`, zid).Scan(&n)
	if n != 1 {
		t.Fatalf("bulk delete harus melewati akun sendiri, tersisa %d", n)
	}

	// Group: simpan + role ter-sync + hapus.
	if c, _ := call(su, "POST", "/user-groups/api/save", `{"title":"Marketing","handle":"marketing","roleHandles":["editor"]}`); c != 200 {
		t.Fatal("buat group")
	}
	d.QueryRow(`SELECT COUNT(*) FROM role_user_group`).Scan(&n)
	if n != 1 {
		t.Fatal("role group harus tersimpan")
	}
	if c, _ := call(su, "POST", "/user-groups/api/delete", `{"id":1}`); c != 200 {
		t.Fatal("hapus group")
	}
}

func slicesHas(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
