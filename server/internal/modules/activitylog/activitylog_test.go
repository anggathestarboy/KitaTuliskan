package activitylog

import (
	"testing"
	"time"
)

func TestHelpers(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	for in, want := range map[string]string{"2026-01-01 11:59:30": "30 seconds ago", "2026-01-01 11:59:00": "1 minute ago", "2026-01-01 09:00:00": "3 hours ago", "2025-12-30 12:00:00": "2 days ago"} {
		if got := ago(in, now); got != want {
			t.Errorf("ago(%s)=%q want %q", in, got, want)
		}
	}
	if color("user_deleted") != "red" || color("banned") != "amber" || color("created") != "green" || color("updated") != "zinc" {
		t.Error("color")
	}
	if prettyChanges("[]") != "" || prettyChanges(nil) != "" || prettyChanges(`{"a":1}`) == "" {
		t.Error("prettyChanges")
	}
}
