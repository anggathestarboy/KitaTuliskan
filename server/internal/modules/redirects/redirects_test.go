package redirects

import (
	"net/http/httptest"
	"strings"
	"testing"

	"draftcms-lite/internal/web"
)

// Satu cek: template redirect-manager terparse, JSON baris masuk ke atribut Edit dalam bentuk ter-escape.
func TestIndexTemplateRenders(t *testing.T) {
	a := web.New(nil, "admin-panel", "../../../templates")
	rec := httptest.NewRecorder()
	row := map[string]any{"id": int64(5), "source": `/old"path`, "target": "/new", "status_code": int64(301), "match_type": "exact", "is_active": int64(1), "hits": int64(3), "start_at": "2026-01-01 10:00:00", "end_at": nil}
	a.Render(rec, httptest.NewRequest("GET", "/admin-panel/redirect-manager", nil), "redirects/index.html", map[string]any{"Rows": []map[string]any{row}, "Base": "/admin-panel/redirect-manager"})
	out := rec.Body.String()
	for _, want := range []string{`$dispatch('edit', {&#34;end_at&#34;:null,`, `&#34;source&#34;:&#34;/old\&#34;path&#34;`, `label:&#34;/old\&#34;path&#34;})`, `x-model="f.status_code"`, `:value="f.is_active ? 1 : 0"`, `New redirect`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}
