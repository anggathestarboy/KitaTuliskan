package blueprints

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"draftcms-lite/internal/web"
)

// Semua halaman modul terparse & terender tanpa ZgotmplZ (parse error template hanya di-log oleh web.App).
func TestTemplatesRender(t *testing.T) {
	a := web.New(nil, "admin-panel", "../../../templates")
	st := State{ID: 7, Handle: "post", Namespace: "collections.blog", Tabs: []Tab{{Handle: "main", Display: "Main", Sections: []Section{{Handle: "hero", Display: "Hero"}}}},
		Fields: []Field{FieldDefaults()}, Errors: map[string]string{"fields.0.handle": "bad"}}
	pages := map[string]map[string]any{
		"blueprints/form.html":          {"State": st, "Namespaces": []Option{{"collections.blog", "Blog"}}, "Templates": []string{"pages.index"}, "Lookups": lookupsRaw(), "CollectionFlags": map[string]map[string]bool{}, "Editor": editorData()},
		"blueprints/fieldset-form.html": {"State": st, "Lookups": lookupsRaw(), "Editor": editorData()},
		"blueprints/index.html":         {"Groups": map[string][]map[string]any{"collections": {{"id": int64(1), "namespace": "collections.blog", "handle": "post", "fields": 2}}}, "Empty": false},
		"blueprints/fieldsets.html":     {"Rows": []map[string]any{{"id": int64(1), "handle": "seo", "fields": 3}}},
	}
	for name, data := range pages {
		rec := httptest.NewRecorder()
		a.Render(rec, httptest.NewRequest("GET", "/", nil), name, data)
		out := rec.Body.String()
		if rec.Code != 200 || strings.Contains(out, "ZgotmplZ") || strings.Contains(out, "template not found") {
			t.Fatalf("%s: code %d, ZgotmplZ=%v", name, rec.Code, strings.Contains(out, "ZgotmplZ"))
		}
		if strings.HasSuffix(name, "form.html") && !strings.Contains(out, "replicator_fields.indexOf(f5)") {
			t.Fatalf("%s: field-editor recursion missing", name)
		}
		if strings.HasSuffix(name, "form.html") && !strings.Contains(out, "Please fix the following before saving:") {
			t.Fatalf("%s: ringkasan error simpan hilang", name)
		}
		// state JSON di x-data harus di-escape tepat sekali (&#34;), bukan &amp;#34; — kalau dobel, Alpine gagal init dan semua method "is not defined".
		if strings.HasSuffix(name, "form.html") && (!strings.Contains(out, `x-data="fieldBuilder({&#34;id&#34;`) || strings.Contains(out, "&amp;#34;")) {
			t.Fatalf("%s: x-data JSON escaped wrong", name)
		}
	}
}

func lookupsRaw() map[string]any {
	return map[string]any{"assetContainers": []Option{{"images", "Images"}}, "collections": []Option{{"blog", "Blog"}}, "taxonomies": []Option{}, "fieldsets": []string{"seo"}}
}

// Storage ↔ editor: ToStorage lalu FromStorage mengembalikan nilai yang sama; ValidateTree menangkap kasus utama.
func TestStorageRoundTripAndValidation(t *testing.T) {
	f := FieldDefaults()
	f.Handle, f.Display, f.Type, f.Options, f.Default, f.MinItems = "status", "Status", "checkbox", "draft, published", "draft", "1"
	sub := FieldDefaults()
	sub.Handle, sub.Display, sub.Type, sub.Default = "on", "On", "toggle", "true"
	rep := FieldDefaults()
	rep.Handle, rep.Display, rep.Type, rep.ReplicatorFields = "rows", "Rows", "replicator", []Field{sub}
	raw, _ := json.Marshal([]any{ToStorage(f, true), ToStorage(rep, true)})
	var stored []any
	json.Unmarshal(raw, &stored)
	back := FromStorage(stored)
	if back[0].Options != "draft, published" || back[0].Default != "draft" || back[0].MinItems != "1" || back[1].ReplicatorFields[0].Default != "true" {
		t.Fatalf("round trip mismatch: %+v", back)
	}
	lk := Lookups{Fieldsets: []string{"seo"}}
	bad := FieldDefaults()
	bad.Handle, bad.Display, bad.Validation, bad.ConditionMode = "title", "Title", "required|nope", "show"
	fs := FieldDefaults()
	fs.Type, fs.Options = "fieldset", "seo"
	fs2 := fs
	errs := ValidateTree([]Field{bad, fs, fs2}, "fields", []string{"main"}, nil, lk, []Field{bad, fs, fs2})
	for _, k := range []string{"fields.0.handle", "fields.0.validation", "fields.0.condition_field", "fields.1.options"} {
		if errs[k] == "" {
			t.Fatalf("expected error %s, got %v", k, errs)
		}
	}
	if len(ValidateTree([]Field{f, rep}, "fields", nil, nil, lk, nil)) != 0 {
		t.Fatalf("valid tree has errors: %v", ValidateTree([]Field{f, rep}, "fields", nil, nil, lk, nil))
	}
	if Headline("hero_title") != "Hero Title" {
		t.Fatal("headline")
	}
}
