package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"draftcms-lite/internal/web"
)

func TestSmoke(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(p, []byte("# c\nFOO_TEST=\"bar\"\n"), 0o600)
	loadEnv(p)
	if os.Getenv("FOO_TEST") != "bar" {
		t.Fatal("loadEnv")
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(web.UUID()) {
		t.Fatal("uuid v4")
	}
	a := &web.App{Prefix: "/admin-panel"}
	rr := httptest.NewRecorder()
	a.Auth(func(http.ResponseWriter, *http.Request) { t.Fatal("must not run") })(rr, httptest.NewRequest("GET", "/admin-panel/dashboard", nil))
	if rr.Code != 302 || rr.Header().Get("Location") != "/admin-panel/login" {
		t.Fatal("Auth guard", rr.Code, rr.Header().Get("Location"))
	}
	u := web.User{Permissions: []string{"access cp"}}
	if u.Can("edit roles") || !u.Can("access cp") || !(web.User{Permissions: []string{"super"}}).Can("anything") {
		t.Fatal("permissions")
	}
	if web.ValidHandle("Blog") || !web.ValidHandle("blog-2") {
		t.Fatal("handle")
	}
}
