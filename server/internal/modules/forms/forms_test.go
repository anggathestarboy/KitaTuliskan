package forms

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"draftcms-lite/internal/web"
)

func TestTemplates(t *testing.T) {
	a := web.New(nil, "admin-panel", "../../../templates")
	form := Form{ID: 1, Handle: "contact", Title: "Contact"}
	pages := map[string]map[string]any{
		"forms/index.html":       {"Rows": []map[string]any{{"id": int64(1), "handle": "contact", "title": "Contact", "blueprint": "contact", "blueprintPick": "contact", "storeSubmissions": true, "redirectUrl": "", "previewFields": []string{}, "submissions": 2}}, "Blueprints": []map[string]any{{"handle": "contact", "label": "forms.contact.contact"}}, "FieldsByBlueprint": map[string][]map[string]string{"contact": {{"handle": "email", "display": "Email"}}}, "Errors": map[string]string{"handle": "bad"}, "Form": map[string]any{"id": int64(0), "handle": "x y", "title": "T", "createBlueprint": true, "previewFields": []string{}}},
		"forms/submissions.html": {"Form": form, "Cols": []map[string]any{{"handle": "email", "display": "Email"}}, "Rows": []map[string]any{{"id": "1700000000.1234", "cells": []any{"a@b.c"}, "created": "2026-01-01 | 10:00"}}},
		"forms/submission.html":  {"Form": form, "ID": "1700000000.1234", "Created": "2026-01-01 10:00", "Values": []map[string]any{{"label": "Email", "html": "a@b.c"}}},
	}
	for name, data := range pages {
		rec := httptest.NewRecorder()
		a.Render(rec, httptest.NewRequest("GET", "/", nil), name, data)
		if out := rec.Body.String(); rec.Code != 200 || strings.Contains(out, "ZgotmplZ") || strings.Contains(out, "template not found") || strings.Contains(out, "&amp;#34;") {
			t.Fatalf("%s: code %d", name, rec.Code)
		}
	}
}

func TestSanitizeAndValidate(t *testing.T) {
	out := Sanitize(`<p onclick="x">Hi <a href="javascript:alert(1)">bad</a> <a href="https://x.y">ok</a><script>evil()</script><span>un</span><img src="/a.png" alt="a"></p>`)
	for _, bad := range []string{"onclick", "javascript:", "<script", "<span"} {
		if strings.Contains(out, bad) {
			t.Fatalf("unsafe: %s", out)
		}
	}
	if !strings.Contains(out, `rel="noopener noreferrer"`) || !strings.Contains(out, "un") || !strings.Contains(out, `<img src="/a.png" alt="a"`) {
		t.Fatalf("lost content: %s", out)
	}
	if ToText("<p>a</p><p>b &amp; c</p>") != "a b & c" {
		t.Fatal(ToText("<p>a</p><p>b &amp; c</p>"))
	}
	fields := []map[string]any{
		{"handle": "email", "display": "Email", "type": "text", "required": true},
		{"handle": "plan", "type": "select", "options": []any{"a", "b"}},
		{"handle": "why", "type": "text", "required": true, "conditions": map[string]any{"mode": "show", "field": "plan", "operator": "equals", "value": "b"}},
	}
	_, errs := validateSubmission(fields, url.Values{"data[plan]": {"a"}})
	if errs["email"] == "" || errs["why"] != "" {
		t.Fatalf("errs %v", errs)
	}
	data, errs := validateSubmission(fields, url.Values{"email": {"x"}, "data[plan]": {"b"}, "data[why]": {"because"}})
	if len(errs) != 0 || data["why"] != "because" || data["plan"] != "b" {
		t.Fatalf("data %v errs %v", data, errs)
	}
	if _, errs := validateSubmission(fields, url.Values{"email": {"x"}, "data[plan]": {"zzz"}}); errs["plan"] == "" {
		t.Fatal("invalid option accepted")
	}
}
