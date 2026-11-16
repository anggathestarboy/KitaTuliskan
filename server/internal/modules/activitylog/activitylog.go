// Package activitylog = app/Modules/ActivityLog. Aktif hanya kalau ACTIVITY_LOG_ENABLED=true (config/activity-log.php).
package activitylog

import (
	"encoding/json"
	"net/http"
	"os"

	"draftcms-lite/internal/web"
)

func Enabled() bool { return os.Getenv("ACTIVITY_LOG_ENABLED") == "true" }

func Register(mux *http.ServeMux, a *web.App) {
	a.Page(mux, "/activity-log", "activitylog/index.html", func(r *http.Request) map[string]any {
		rows, _ := web.Rows(a.DB, `SELECT l.*, u.name AS user_name FROM activity_logs l LEFT JOIN users u ON u.id=l.user_id ORDER BY l.id DESC LIMIT 200`)
		return map[string]any{"Rows": rows}
	})
}

// Record = ActivityLogger::log(). No-op kalau tidak enabled.
func Record(a *web.App, userID, action, subjectType, subjectID, description string, changes any) {
	if !Enabled() {
		return
	}
	b, _ := json.Marshal(changes)
	a.DB.Exec(`INSERT INTO activity_logs(user_id,action,subject_type,subject_id,description,changes,created_at) VALUES(?,?,?,?,?,?,?)`,
		userID, action, subjectType, subjectID, description, string(b), web.Now())
}
