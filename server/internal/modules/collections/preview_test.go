package collections

import "testing"

func TestPreviewVersionBumps(t *testing.T) {
	p := func(title string) map[string]any {
		return previewPayload("blog", "post", "id", "1", "", "draft", "s", "2026-01-02T03:04", "", title, map[string]any{"a": 1})
	}
	if v := putPreview("t", p("x")); v != 1 {
		t.Fatalf("versi awal %d", v)
	}
	if v := putPreview("t", p("y")); v != 2 {
		t.Fatalf("versi kedua %d", v)
	}
	got := getPreview("t")["attributes"].(map[string]any)
	if got["data"].(map[string]any)["title"] != "y" || got["date"] != "2026-01-02 03:04" || got["order"] != nil {
		t.Fatalf("payload salah: %v", got)
	}
	if getPreview("none") != nil {
		t.Fatal("token tak dikenal harus nil")
	}
}
