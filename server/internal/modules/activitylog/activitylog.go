// Package activitylog = app/Modules/ActivityLog. Aktif hanya kalau ACTIVITY_LOG_ENABLED=true (config/activity-log.php);
// nyalakan lewat `make activity_log` (= php artisan draft:activity_log).
package activitylog

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"draftcms-lite/internal/mail"
	"draftcms-lite/internal/web"
)

const (
	perPage = 25
	layout  = "2006-01-02 15:04:05"
)

func Enabled() bool { return os.Getenv("ACTIVITY_LOG_ENABLED") == "true" }

var envLine = regexp.MustCompile(`(?m)^ACTIVITY_LOG_ENABLED=.*$`)

// Enable = DraftActivityLogCommand: set ACTIVITY_LOG_ENABLED=true di .env. Tabelnya sudah dibuat db.Open (schema.sql).
func Enable(envPath string) error {
	b, err := os.ReadFile(envPath)
	if err != nil {
		return errors.New("no .env file found — create one first (cp .env.example .env)")
	}
	s := string(b)
	if envLine.MatchString(s) {
		s = envLine.ReplaceAllString(s, "ACTIVITY_LOG_ENABLED=true")
	} else {
		s = strings.TrimRight(s, " \r\n") + "\nACTIVITY_LOG_ENABLED=true\n"
	}
	return os.WriteFile(envPath, []byte(s), 0o644)
}

// Record = ActivityLogger::log(). No-op kalau tidak enabled. userID/subject kosong → NULL (aksi sistem / tanpa subjek).
func Record(a *web.App, userID, action, subjectType, subjectID, description string, changes any) {
	RecordProps(a, userID, action, subjectType, subjectID, description, changes, nil)
}

// RecordProps = Record + kolom properties (IP, email, dst).
func RecordProps(a *web.App, userID, action, subjectType, subjectID, description string, changes, properties any) {
	if !Enabled() {
		return
	}
	nz := func(s string) any {
		if s == "" {
			return nil
		}
		return s
	}
	js := func(v any) any {
		if v == nil {
			return nil
		}
		b, _ := json.Marshal(v)
		return string(b)
	}
	a.DB.Exec(`INSERT INTO activity_logs(user_id,action,subject_type,subject_id,description,changes,properties,created_at) VALUES(?,?,?,?,?,?,?,?)`,
		nz(userID), action, nz(subjectType), nz(subjectID), description, js(changes), js(properties), web.Now())
}

// IP = request()->ip() untuk properties login/logout.
func IP(r *http.Request) string {
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return h
}

func Register(mux *http.ServeMux, a *web.App) {
	need := func(perm string, h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !web.CurrentUser(r).Can(perm) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			h(w, r)
		}
	}
	a.Admin(mux, "GET /activity-log", need("view activity log", index(a)))
	a.Admin(mux, "GET /activity-log/export", need("export activity log", export(a)))
	a.Admin(mux, "POST /activity-log/restore", need("import activity log", restore(a)))
	a.Admin(mux, "POST /activity-log/delete-request", requestDeletion(a))
	a.Admin(mux, "GET /activity-log/confirm-delete/{token}", confirm(a))
	go func() { // = schedule harian 00:00 (routes/console.php)
		for {
			n := time.Now()
			time.Sleep(time.Date(n.Year(), n.Month(), n.Day()+1, 0, 0, 0, 0, n.Location()).Sub(n))
			Process(a)
		}
	}()
}

type pageLink struct {
	N       int // 0 = "…"
	URL     string
	Current bool
}

func index(a *web.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f := map[string]string{"search": q.Get("search"), "action": q.Get("action"), "user_id": q.Get("user_id"), "date_from": q.Get("date_from"), "date_to": q.Get("date_to")}
		where, args := []string{"1=1"}, []any{}
		if f["search"] != "" {
			where, args = append(where, "l.description LIKE ?"), append(args, "%"+f["search"]+"%")
		}
		if f["action"] != "" {
			where, args = append(where, "l.action=?"), append(args, f["action"])
		}
		if f["user_id"] != "" {
			where, args = append(where, "l.user_id=?"), append(args, f["user_id"])
		}
		for k, op := range map[string]string{"date_from": ">=", "date_to": "<="} {
			if _, err := time.Parse("2006-01-02", f[k]); err == nil {
				where, args = append(where, "DATE(l.created_at)"+op+"?"), append(args, f[k])
			} else {
				f[k] = ""
			}
		}
		cond := strings.Join(where, " AND ")
		var total int
		a.DB.QueryRow(`SELECT COUNT(*) FROM activity_logs l WHERE `+cond, args...).Scan(&total)
		last := max(1, (total+perPage-1)/perPage)
		page, _ := strconv.Atoi(q.Get("page"))
		page = min(max(page, 1), last)

		rows, _ := web.Rows(a.DB, `SELECT l.id,l.action,l.description,l.changes,l.created_at,u.name AS user_name FROM activity_logs l LEFT JOIN users u ON u.id=l.user_id
			WHERE `+cond+` ORDER BY l.created_at DESC, l.id DESC LIMIT ? OFFSET ?`, append(slices.Clone(args), perPage, (page-1)*perPage)...)
		now := time.Now().UTC()
		for _, row := range rows {
			row["Ago"] = ago(fmt.Sprint(row["created_at"]), now)
			row["Color"] = color(fmt.Sprint(row["action"]))
			row["Changes"] = prettyChanges(row["changes"])
			if row["user_name"] == nil {
				row["user_name"] = "System"
			}
		}

		link := func(p int) string {
			v := url.Values{}
			for k, s := range f {
				if s != "" {
					v.Set(k, s)
				}
			}
			v.Set("page", strconv.Itoa(p))
			return a.CP("/activity-log?" + v.Encode())
		}
		var links []pageLink
		for p := 1; p <= last; p++ {
			if p == 1 || p == last || p-page <= 2 && page-p <= 2 {
				links = append(links, pageLink{p, link(p), p == page})
			} else if links[len(links)-1].N != 0 {
				links = append(links, pageLink{})
			}
		}
		actions, _ := web.Rows(a.DB, `SELECT DISTINCT action FROM activity_logs ORDER BY action`)
		users, _ := web.Rows(a.DB, `SELECT id,name FROM users ORDER BY name`)
		reqs, _ := web.Rows(a.DB, `SELECT year,status,created_at FROM activity_log_deletion_requests WHERE status IN ('pending','confirmed') ORDER BY id DESC`)
		for _, x := range reqs {
			x["Ago"] = ago(fmt.Sprint(x["created_at"]), now)
		}
		a.Render(w, r, "activitylog/index.html", map[string]any{
			"Rows": rows, "F": f, "Actions": actions, "Users": users, "Requests": reqs,
			"Filtered": f["search"]+f["action"]+f["user_id"]+f["date_from"]+f["date_to"] != "",
			"Total":    total, "From": min((page-1)*perPage+1, total), "To": min(page*perPage, total), "Last": last, "Links": links,
			"Prev": cond2(page > 1, link(page-1)), "Next": cond2(page < last, link(page+1)),
			"Month": now.Format("2006-01"), "Year": now.Year(),
		})
	}
}

func cond2(ok bool, s string) string {
	if ok {
		return s
	}
	return ""
}

// color = match() di blade: delete/fail merah, ban kuning, creat hijau.
func color(action string) string {
	switch {
	case strings.Contains(action, "delete"), strings.Contains(action, "fail"):
		return "red"
	case strings.Contains(action, "ban"):
		return "amber"
	case strings.Contains(action, "creat"):
		return "green"
	}
	return "zinc"
}

// prettyChanges: JSON diff untuk modal "Changes"; kosong ("", null, [], {}) → "" (tombol mata disembunyikan).
func prettyChanges(v any) string {
	s, _ := v.(string)
	if s == "" || s == "null" || s == "[]" || s == "{}" {
		return ""
	}
	var x any
	if json.Unmarshal([]byte(s), &x) != nil {
		return ""
	}
	b, _ := json.MarshalIndent(x, "", "  ")
	return string(b)
}

// ago = Carbon::diffForHumans() (hanya bentuk "… ago").
func ago(s string, now time.Time) string {
	t, err := time.Parse(layout, s)
	if err != nil {
		return s
	}
	d := max(now.Sub(t), 0)
	n, unit := 0, ""
	switch {
	case d < time.Minute:
		n, unit = int(d.Seconds()), "second"
	case d < time.Hour:
		n, unit = int(d.Minutes()), "minute"
	case d < 24*time.Hour:
		n, unit = int(d.Hours()), "hour"
	case d < 30*24*time.Hour:
		n, unit = int(d.Hours()/24), "day"
	case d < 365*24*time.Hour:
		n, unit = int(d.Hours()/24/30), "month"
	default:
		n, unit = int(d.Hours()/24/365), "year"
	}
	if n != 1 {
		unit += "s"
	}
	return fmt.Sprintf("%d %s ago", n, unit)
}

func back(a *web.App, w http.ResponseWriter, r *http.Request, msg string) {
	web.Flash(w, msg)
	http.Redirect(w, r, a.CP("/activity-log"), http.StatusFound)
}

// export = ActivityLogManager::export(): JSON per bulan/tahun, bentuk yang sama dengan yang dibaca restore().
func export(a *web.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var from, to time.Time
		var name string
		var err error
		if q.Get("period") == "year" {
			if from, err = time.Parse("2006", q.Get("year")); err == nil {
				to, name = from.AddDate(1, 0, 0), "activity-log-"+from.Format("2006")+".json"
			}
		} else if from, err = time.Parse("2006-01", q.Get("month")); err == nil {
			to, name = from.AddDate(0, 1, 0), "activity-log-"+from.Format("2006-01")+".json"
		}
		if err != nil {
			back(a, w, r, "Please pick a valid month or year to export.")
			return
		}
		rows, _ := web.Rows(a.DB, `SELECT id,user_id,action,subject_type,subject_id,description,changes,properties,created_at FROM activity_logs
			WHERE created_at>=? AND created_at<? ORDER BY created_at, id`, from.Format(layout), to.Format(layout))
		out := []map[string]any{}
		for _, row := range rows {
			for _, k := range []string{"changes", "properties"} {
				if s, _ := row[k].(string); json.Valid([]byte(s)) {
					row[k] = json.RawMessage(s)
				} else {
					row[k] = nil
				}
			}
			out = append(out, row)
		}
		b, _ := json.MarshalIndent(out, "", "    ")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		w.Write(b)
	}
}

// restore = ActivityLogManager::restore(): upsert per id, jadi restore file yang sama dua kali aman.
func restore(a *web.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 20<<20)
		f, h, err := r.FormFile("restoreFile")
		if err != nil {
			back(a, w, r, "The restore file field is required (max 20 MB).")
			return
		}
		defer f.Close()
		if !strings.EqualFold(filepath.Ext(h.Filename), ".json") {
			back(a, w, r, "Please upload a .json export file.")
			return
		}
		var rows []map[string]any
		if json.NewDecoder(f).Decode(&rows) != nil {
			back(a, w, r, "That file is not a valid activity log export.")
			return
		}
		n := 0
		for _, row := range rows {
			id := web.Int64(row["id"])
			if id == 0 {
				continue
			}
			str := func(k string) any {
				if s, ok := row[k].(string); ok {
					return s
				}
				return nil
			}
			js := func(k string) any {
				if row[k] == nil {
					return nil
				}
				b, _ := json.Marshal(row[k])
				return string(b)
			}
			created := web.Now()
			if s, _ := row["created_at"].(string); s != "" {
				if t, err := time.Parse(time.RFC3339, s); err == nil {
					created = t.UTC().Format(layout)
				} else {
					created = s
				}
			}
			action := "unknown"
			if s, _ := row["action"].(string); s != "" {
				action = s
			}
			desc, _ := row["description"].(string)
			if _, err := a.DB.Exec(`INSERT INTO activity_logs(id,user_id,action,subject_type,subject_id,description,changes,properties,created_at) VALUES(?,?,?,?,?,?,?,?,?)
				ON DUPLICATE KEY UPDATE user_id=VALUES(user_id),action=VALUES(action),subject_type=VALUES(subject_type),subject_id=VALUES(subject_id),description=VALUES(description),changes=VALUES(changes),properties=VALUES(properties),created_at=VALUES(created_at)`,
				id, str("user_id"), action, str("subject_type"), str("subject_id"), desc, js("changes"), js("properties"), created); err == nil {
				n++
			}
		}
		if n == 1 {
			back(a, w, r, "1 entry restored.")
		} else {
			back(a, w, r, fmt.Sprintf("%d entries restored.", n))
		}
	}
}

func hash(token string) string { h := sha256.Sum256([]byte(token)); return hex.EncodeToString(h[:]) }

// requestDeletion = langkah 1 hard delete per tahun (superadmin): buat request pending + email link konfirmasi. Belum ada yang dihapus.
func requestDeletion(a *web.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := web.CurrentUser(r)
		if !u.IsSuper() {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		year, err := strconv.Atoi(r.FormValue("year"))
		if now := time.Now().Year(); err != nil || year < 2000 || year > now {
			back(a, w, r, fmt.Sprintf("The delete year field must be a year between 2000 and %d.", now))
			return
		}
		b := make([]byte, 32)
		rand.Read(b)
		token := hex.EncodeToString(b)
		n := web.Now()
		a.DB.Exec(`INSERT INTO activity_log_deletion_requests(year,requested_by,token_hash,status,expires_at,created_at,updated_at) VALUES(?,?,?,'pending',?,?,?)`,
			year, u.ID, hash(token), time.Now().UTC().Add(24*time.Hour).Format(layout), n, n)
		link := strings.TrimRight(os.Getenv("APP_URL"), "/") + a.CP("/activity-log/confirm-delete/"+token)
		mail.Send(u.Email, fmt.Sprintf("Confirm: permanently delete %d activity log entries", year), fmt.Sprintf(
			"A request was made to permanently delete every activity log entry from %d.\n\nThis cannot be undone. If you did not request this, ignore this email — nothing will happen without your confirmation.\n\nConfirm deletion: %s\n\nThe deletion will run automatically at 00:00 on the first night after you confirm.\nThis confirmation link expires in 24 hours.\n", year, link))
		back(a, w, r, "A confirmation email was sent to your address. Nothing will be deleted until you click the link.")
	}
}

// confirm = ConfirmDeletionController: langkah 2, hanya membalik status ke confirmed; hapus sungguhan di Process().
func confirm(a *web.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var id, year int
		var status, expires string
		err := a.DB.QueryRow(`SELECT id,year,status,expires_at FROM activity_log_deletion_requests WHERE token_hash=?`, hash(r.PathValue("token"))).Scan(&id, &year, &status, &expires)
		exp, _ := time.Parse(layout, expires)
		st := "confirmed"
		switch {
		case !web.CurrentUser(r).IsSuper():
			st = "forbidden"
		case err != nil:
			st = "not_found"
		case status == "executed":
			st = "already_executed"
		case status == "confirmed":
			st = "already_confirmed"
		case status == "expired" || exp.Before(time.Now().UTC()):
			st = "expired"
		}
		if st == "confirmed" {
			n := web.Now()
			a.DB.Exec(`UPDATE activity_log_deletion_requests SET status='confirmed',confirmed_at=?,updated_at=? WHERE id=?`, n, n, id)
		}
		a.Render(w, r, "activitylog/confirm-delete.html", map[string]any{"Status": st, "Year": year})
	}
}

// Process = activity-log:process-deletions (tiap 00:00 dan `go run . activity_log:process`): eksekusi request confirmed, expire yang pending lewat 24 jam.
func Process(a *web.App) (executed, expired int) {
	rows, _ := web.Rows(a.DB, `SELECT id,year,requested_by FROM activity_log_deletion_requests WHERE status='confirmed'`)
	for _, row := range rows {
		year := int(web.Int64(row["year"]))
		res, err := a.DB.Exec(`DELETE FROM activity_logs WHERE created_at BETWEEN ? AND ?`, fmt.Sprintf("%d-01-01 00:00:00", year), fmt.Sprintf("%d-12-31 23:59:59", year))
		if err != nil {
			continue
		}
		deleted, _ := res.RowsAffected()
		n := web.Now()
		a.DB.Exec(`UPDATE activity_log_deletion_requests SET status='executed',executed_at=?,deleted_count=?,updated_at=? WHERE id=?`, n, deleted, n, row["id"])
		RecordProps(a, "", "activity_log_year_deleted", "", "", fmt.Sprintf("Permanently deleted %d activity log entries from %d", deleted, year), nil,
			map[string]any{"year": year, "deleted_count": deleted, "requested_by": row["requested_by"]})
		executed++
	}
	res, _ := a.DB.Exec(`UPDATE activity_log_deletion_requests SET status='expired' WHERE status='pending' AND expires_at<?`, web.Now())
	if res != nil {
		n, _ := res.RowsAffected()
		expired = int(n)
	}
	return
}
