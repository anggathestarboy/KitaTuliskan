package navigation

import (
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
	d, err := db.Open("127.0.0.1", "3306", "root", "", "draftcms_lite_test", true)
	if err != nil {
		t.Skip("MySQL tidak tersedia:", err)
	}
	t.Cleanup(func() { d.Exec("DROP DATABASE draftcms_lite_test") })
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
