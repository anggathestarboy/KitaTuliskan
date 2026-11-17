package navigation

import (
	"encoding/json"
	"strings"
	"testing"

	"draftcms-lite/internal/db"
	"draftcms-lite/internal/modules/structure"
	"draftcms-lite/internal/web"
)

func ctx(roles ...string) *sctx {
	c := &sctx{roles: map[string]bool{}, seen: map[string]bool{}}
	for _, r := range roles {
		c.roles[r] = true
	}
	return c
}

func TestSanitize(t *testing.T) {
	in := []node{
		{ID: "a", Type: "internal", Title: " Home ", URL: "/", Roles: []string{"editor", "ghost", "editor"}, Children: []node{
			{ID: "a", Type: "external", Title: "Dup id", URL: "https://example.com", Target: "_blank"},
		}},
		{Type: "page", Title: "About", PageID: "p1", URL: "/ignored"},
	}
	out, err := sanitize(in, 1, ctx("editor"))
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Title != "Home" || len(out[0].Roles) != 1 || out[0].Roles[0] != "editor" {
		t.Fatalf("title/roles: %+v", out[0])
	}
	if out[0].Children[0].ID == "a" || out[0].Children[0].ID == "" || out[0].Children[0].Target != "_blank" {
		t.Fatalf("id kembar harus diganti: %+v", out[0].Children[0])
	}
	if out[1].ID == "" || out[1].URL != "" || out[1].PageID != "p1" || out[1].Target != "_self" {
		t.Fatalf("page: %+v", out[1])
	}

	bad := map[string][]node{
		"javascript": {{Type: "external", URL: "javascript:alert(1)"}},
		"data":       {{Type: "internal", URL: "DATA:text/html,x"}},
		"type":       {{Type: "weird"}},
		"depth":      {{Type: "internal", Children: []node{{Type: "internal", Children: []node{{Type: "internal"}}}}}},
		"long":       {{Type: "internal", Title: strings.Repeat("x", 256)}},
	}
	for name, tree := range bad {
		if _, err := sanitize(tree, 1, ctx()); err == nil {
			t.Fatalf("%s harus ditolak", name)
		}
	}
	for _, u := range []string{"", "/x", "#a", "mailto:a@b.c", "tel:123", "HTTPS://x.y"} {
		if !validURL(u) {
			t.Fatalf("%q harus lolos", u)
		}
	}
}

func TestProtected(t *testing.T) {
	if !isProtected("sidebar-admin") || isProtected("main") {
		t.Fatal("protected")
	}
	u := &web.User{Permissions: []string{"edit main navigation"}}
	if !canEdit(u, "main") || canEdit(u, "other") || canDelete(u, "main") || !canView(u, "main") {
		t.Fatal("izin per-navigation")
	}
	if !canDelete(&web.User{Permissions: []string{"configure navigation"}}, "x") {
		t.Fatal("configure navigation berlaku untuk semua")
	}
}

// Butuh MySQL lokal; dilewati kalau tidak ada.
func TestStoreSaveIsUpsert(t *testing.T) {
	d, err := db.Open("127.0.0.1", "3306", "root", "", "draftcms_lite_test_nav", true)
	if err != nil {
		t.Skip("MySQL tidak tersedia:", err)
	}
	t.Cleanup(func() { d.Exec("DROP DATABASE draftcms_lite_test_nav") })
	for i, title := range []string{"one", "two", "three"} {
		if err := structure.Save(d, "m", "navigation", []structure.Item{{ID: "i", Type: "internal", Title: title}}); err != nil {
			t.Fatal(i, err)
		}
	}
	var n int
	d.QueryRow(`SELECT COUNT(*) FROM trees WHERE handle='m'`).Scan(&n)
	got := structure.Load(d, "m", "navigation")
	if n != 1 || len(got) != 1 || got[0].Title != "three" {
		t.Fatalf("Save harus meng-update satu baris (locale NULL): rows=%d tree=%+v", n, got)
	}
	structure.Rename(d, "m", "m2", "navigation")
	if len(structure.Load(d, "m2", "navigation")) != 1 || len(structure.Load(d, "m", "navigation")) != 0 {
		t.Fatal("rename")
	}
	a := &web.App{DB: d, Prefix: "/admin-panel"}
	if !SeedAdminSidebar(a, false) || SeedAdminSidebar(a, false) {
		t.Fatal("seed hanya sekali tanpa force")
	}
	structure.Save(d, SidebarHandle, "navigation", nil)
	if !SeedAdminSidebar(a, true) || len(structure.Load(d, SidebarHandle, "navigation")) != 20 {
		t.Fatal("force harus mereset ke 20 item default")
	}
}

func TestContentCheck(t *testing.T) {
	var fields []map[string]any
	json.Unmarshal([]byte(`[
	 {"handle":"title","display":"Halo","type":"text","required":true,"validation":"max:5"},
	 {"handle":"kind","type":"select","options":["a","b"]},
	 {"handle":"extra","type":"text","required":true,"conditions":{"mode":"show","field":"kind","operator":"equals","value":"b"}},
	 {"handle":"links","display":"Links","type":"replicator","replicator_fields":[{"handle":"label","display":"Label","type":"text","required":true},{"handle":"url","type":"link","validation":"url"}]}
	]`), &fields)
	run := func(in string) (map[string]any, map[string]string) {
		var d map[string]any
		json.Unmarshal([]byte(in), &d)
		c := &chk{errs: map[string]string{}}
		return c.check(fields, d, ""), c.errs
	}
	if _, e := run(`{"title":"","kind":"a","links":[]}`); e["title"] != "The Halo field is required." || e["extra"] != "" {
		t.Fatalf("required + kondisi tersembunyi: %v", e)
	}
	if _, e := run(`{"title":"toolong","kind":"b","links":[]}`); e["title"] == "" || e["extra"] == "" {
		t.Fatalf("max + extra wajib saat kind=b: %v", e)
	}
	if _, e := run(`{"title":"ok","kind":"zzz","links":[{"label":"","url":"nope"},{"label":"x","url":"entry::5"}]}`); e["kind"] == "" || e["links.0.label"] == "" || e["links.0.url"] == "" || e["links.1.url"] != "" {
		t.Fatalf("select/replicator/link: %v", e)
	}
	out, e := run(`{"title":"ok","kind":"a","stray":1,"links":[{"label":"x","_uid":"u","url":""}]}`)
	if len(e) != 0 || out["stray"] != nil || out["links"].([]any)[0].(map[string]any)["_uid"] != nil {
		t.Fatalf("field asing/_uid harus dibuang: %v %v", e, out)
	}
	if !eq(map[string]any{"a": []any{1.0}}, map[string]any{"a": []any{"1"}}) || eq([]any{"x"}, []any{"y"}) {
		t.Fatal("eq")
	}
	if !condMatches(map[string]any{"operator": "contains_any", "value": "a, c"}, []any{"c"}) || condMatches(map[string]any{"operator": "strict", "value": "1"}, 1.0) {
		t.Fatal("operator kondisi")
	}
}
