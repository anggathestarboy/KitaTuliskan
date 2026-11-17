package sites

import (
	"net/http/httptest"
	"strings"
	"testing"

	"draftcms-lite/internal/web"
)

// Satu cek: template sites terparse; origin dapat badge dan tanpa menu Delete; baris lain punya Delete.
func TestIndexTemplateRenders(t *testing.T) {
	a := web.New(nil, "admin-panel", "../../../templates")
	rec := httptest.NewRecorder()
	rows := []map[string]any{
		{"id": int64(1), "name": "Indonesia", "handle": "indonesia", "url": "/", "locale": "id", "lang": "id", "attrs": []map[string]any{{"key": "native_name", "value": "Bahasa"}}},
		{"id": int64(2), "name": "English", "handle": "english", "url": "/en", "locale": "en", "lang": "en", "attrs": []map[string]any{}},
	}
	a.Render(rec, httptest.NewRequest("GET", "/", nil), "sites/index.html", map[string]any{"Rows": rows, "Base": "/admin-panel/sites", "Origin": int64(1), "Languages": Languages, "CanConfigure": true})
	out := rec.Body.String()
	for _, want := range []string{">Origin</span>", `label:&#34;English&#34;})`, `<option value="">Select a language</option>`, `:readonly="f.id == 1"`, `&#34;attrs&#34;:[{&#34;key&#34;:&#34;native_name&#34;`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if strings.Count(out, "Delete</button>") != 2 { // 1 baris non-origin + tombol di dialog konfirmasi
		t.Fatalf("origin must not have Delete; got %d", strings.Count(out, "Delete</button>"))
	}
	if strings.Contains(out, "ZgotmplZ") {
		t.Fatal("ZgotmplZ in output")
	}
}
