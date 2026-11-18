package sitesettings

import (
	"net/http/httptest"
	"strings"
	"testing"

	"draftcms-lite/internal/web"
)

func TestRenderAndBuild(t *testing.T) {
	a := web.New(nil, "admin-panel", "../../../templates")
	st := State{LlmsCollections: []string{}, LlmsTaxonomies: []string{}, LlmsLimit: "1000", SchemaBlocks: []Block{{Type: "Organization", Name: "Acme", SameAs: []string{}}}, Errors: map[string]string{}}
	rec := httptest.NewRecorder()
	a.Render(rec, httptest.NewRequest("GET", "/?q=schema", nil), "sitesettings/index.html", map[string]any{"State": st, "Tab": "schema", "Collections": []map[string]string{{"handle": "pages", "title": "Pages"}}, "Taxonomies": []map[string]string{},
		"Opts": map[string]any{"appName": "X", "appUrl": "http://x", "assetContainer": "images", "schemaTypes": SchemaTypes}})
	out := rec.Body.String()
	if rec.Code != 200 || strings.Contains(out, "ZgotmplZ") || strings.Contains(out, "&amp;#34;") || !strings.Contains(out, `x-data="siteSettings({&#34;robotsTxt&#34;`) {
		t.Fatalf("render: %d %s", rec.Code, out[:200])
	}
	b := Block{Type: "Organization", Name: "Acme", Telephone: "1", SameAs: []string{"", "https://x"}}
	m := Build(b)
	if m["@type"] != "Organization" || m["contactPoint"] == nil || m["address"] != nil || len(m["sameAs"].([]string)) != 1 || Build(Block{}) != nil {
		t.Fatalf("build: %v", m)
	}
	errs := validate(State{LlmsLimit: "0", GaEnabled: true, SchemaBlocks: []Block{{Type: "Nope", Email: "bad"}}})
	for _, k := range []string{"llmsLimit", "gaMeasurementId", "schemaBlocks.0.type", "schemaBlocks.0.email"} {
		if errs[k] == "" {
			t.Fatalf("missing %s in %v", k, errs)
		}
	}
}
