// Package auth = app/Modules/Auth: Fortify (login, forgot/reset, verify, confirm) + Settings (profile, security, appearance).
// Registrasi publik dimatikan seperti source (Features::registration tidak ada); user dibuat lewat `setup` atau modul Users.
package auth

import (
	"database/sql"
	"net/http"
	"os"
	"strings"
	"time"

	"draftcms-lite/internal/mail"
	"draftcms-lite/internal/modules/activitylog"
	"draftcms-lite/internal/web"

	"golang.org/x/crypto/bcrypt"
)

func Register(mux *http.ServeMux, a *web.App) {
	cp := a.CP
	mux.HandleFunc("GET "+cp("/login"), a.Guest(func(w http.ResponseWriter, r *http.Request) { a.Render(w, r, "auth/login.html", nil) }))
	mux.HandleFunc("POST "+cp("/login"), a.Guest(func(w http.ResponseWriter, r *http.Request) {
		var id, hash, name string
		var banned sql.NullString
		err := a.DB.QueryRow(`SELECT id,password,banned_at,name FROM users WHERE email=?`, strings.ToLower(r.FormValue("email"))).Scan(&id, &hash, &banned, &name)
		if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(r.FormValue("password"))) != nil {
			if err != nil {
				id = ""
			}
			// Listener Failed: subject = user bila emailnya dikenal, actor kosong (belum login).
			email := r.FormValue("email")
			subj := ""
			if id != "" {
				subj = "user"
			}
			activitylog.RecordProps(a, "", "login_failed", subj, id, "Failed login attempt for "+email, nil, map[string]any{"ip": activitylog.IP(r), "email": email})
			w.WriteHeader(http.StatusUnprocessableEntity)
			a.Render(w, r, "auth/login.html", map[string]any{"Error": "These credentials do not match our records.", "Email": r.FormValue("email")})
			return
		}
		if banned.Valid {
			w.WriteHeader(http.StatusUnprocessableEntity)
			a.Render(w, r, "auth/login.html", map[string]any{"Error": "Your account has been banned.", "Email": r.FormValue("email")})
			return
		}
		a.Login(w, r, id)
		activitylog.RecordProps(a, id, "logged_in", "user", id, name+" logged in", nil, map[string]any{"ip": activitylog.IP(r)})
		http.Redirect(w, r, cp("/dashboard"), http.StatusFound)
	}))
	mux.HandleFunc("POST "+cp("/logout"), func(w http.ResponseWriter, r *http.Request) {
		if u := web.CurrentUser(r); u != nil {
			activitylog.RecordProps(a, u.ID, "logged_out", "user", u.ID, u.Name+" logged out", nil, map[string]any{"ip": activitylog.IP(r)})
		}
		a.Logout(w, r)
		http.Redirect(w, r, cp("/login"), http.StatusFound)
	})

	mux.HandleFunc("GET "+cp("/forgot-password"), a.Guest(func(w http.ResponseWriter, r *http.Request) { a.Render(w, r, "auth/forgot-password.html", nil) }))
	mux.HandleFunc("POST "+cp("/forgot-password"), a.Guest(func(w http.ResponseWriter, r *http.Request) {
		SendResetLink(a, strings.ToLower(r.FormValue("email")))
		a.Render(w, r, "auth/forgot-password.html", map[string]any{"Status": "A reset link will be sent if the account exists."})
	}))
	mux.HandleFunc("GET "+cp("/reset-password/{token}"), a.Guest(func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "auth/reset-password.html", map[string]any{"Token": r.PathValue("token"), "Email": r.URL.Query().Get("email")})
	}))
	mux.HandleFunc("POST "+cp("/reset-password"), a.Guest(func(w http.ResponseWriter, r *http.Request) {
		email, tok, pw := strings.ToLower(r.FormValue("email")), r.FormValue("token"), r.FormValue("password")
		var created string
		err := a.DB.QueryRow(`SELECT created_at FROM password_reset_tokens WHERE email=? AND token=?`, email, tok).Scan(&created)
		t, _ := time.Parse("2006-01-02 15:04:05", created)
		if err != nil || time.Since(t) > time.Hour || len(pw) < 8 || pw != r.FormValue("password_confirmation") {
			w.WriteHeader(http.StatusUnprocessableEntity)
			a.Render(w, r, "auth/reset-password.html", map[string]any{"Token": tok, "Email": email, "Error": "Invalid token or password (min 8, must match)."})
			return
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(pw), 12)
		a.DB.Exec(`UPDATE users SET password=?,updated_at=? WHERE email=?`, hash, web.Now(), email)
		a.DB.Exec(`DELETE FROM password_reset_tokens WHERE email=?`, email)
		web.Flash(w, "Your password has been reset.")
		http.Redirect(w, r, cp("/login"), http.StatusFound)
	}))

	mux.HandleFunc("GET "+cp("/verify-email"), func(w http.ResponseWriter, r *http.Request) {
		u := web.CurrentUser(r)
		switch {
		case u == nil:
			http.Redirect(w, r, cp("/login"), http.StatusFound)
		case u.EmailVerifiedAt.Valid:
			http.Redirect(w, r, cp("/dashboard"), http.StatusFound)
		default:
			a.Render(w, r, "auth/verify-email.html", nil)
		}
	})
	mux.HandleFunc("POST "+cp("/email/verification-notification"), func(w http.ResponseWriter, r *http.Request) {
		if u := web.CurrentUser(r); u != nil {
			SendVerification(a, u.ID, u.Email)
			web.Flash(w, "verification-link-sent")
		}
		http.Redirect(w, r, cp("/verify-email"), http.StatusFound)
	})
	mux.HandleFunc("GET "+cp("/verify-email/{id}/{hash}"), func(w http.ResponseWriter, r *http.Request) {
		// ponytail: hash = token di tabel cache, bukan signed URL Laravel
		var val string
		if a.DB.QueryRow(`SELECT value FROM cache WHERE cache.key=? AND expiration>?`, "verify:"+r.PathValue("id"), time.Now().Unix()).Scan(&val) == nil && val == r.PathValue("hash") {
			a.DB.Exec(`UPDATE users SET email_verified_at=? WHERE id=? AND email_verified_at IS NULL`, web.Now(), r.PathValue("id"))
			a.DB.Exec(`DELETE FROM cache WHERE cache.key=?`, "verify:"+r.PathValue("id"))
		}
		http.Redirect(w, r, cp("/dashboard?verified=1"), http.StatusFound)
	})

	mux.HandleFunc("GET "+cp("/user/confirm-password"), a.Auth(func(w http.ResponseWriter, r *http.Request) { a.Render(w, r, "auth/confirm-password.html", nil) }))
	mux.HandleFunc("POST "+cp("/user/confirm-password"), a.Auth(func(w http.ResponseWriter, r *http.Request) {
		if !CheckPassword(a.DB, web.CurrentUser(r).ID, r.FormValue("password")) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			a.Render(w, r, "auth/confirm-password.html", map[string]any{"Error": "The password is incorrect."})
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "pw_confirmed", Value: "1", Path: "/", HttpOnly: true, MaxAge: 3 * 3600})
		http.Redirect(w, r, r.URL.Query().Get("next"), http.StatusFound)
	}))

	// routes/settings.php — tanpa prefix CMS, seperti source.
	mux.HandleFunc("GET /settings", a.Auth(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/settings/profile", http.StatusFound)
	}))
	mux.HandleFunc("GET /settings/profile", a.Auth(func(w http.ResponseWriter, r *http.Request) { a.Render(w, r, "auth/profile.html", nil) }))
	mux.HandleFunc("POST /settings/profile", a.Auth(func(w http.ResponseWriter, r *http.Request) {
		u := web.CurrentUser(r)
		name, email := strings.TrimSpace(r.FormValue("name")), strings.ToLower(strings.TrimSpace(r.FormValue("email")))
		if name == "" || !strings.Contains(email, "@") {
			w.WriteHeader(http.StatusUnprocessableEntity)
			a.Render(w, r, "auth/profile.html", map[string]any{"Error": "Name and valid email required."})
			return
		}
		q := `UPDATE users SET name=?,email=?,updated_at=? WHERE id=?`
		if email != u.Email {
			q = `UPDATE users SET name=?,email=?,updated_at=?,email_verified_at=NULL WHERE id=?`
		}
		if _, err := a.DB.Exec(q, name, email, web.Now(), u.ID); err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			a.Render(w, r, "auth/profile.html", map[string]any{"Error": "The email has already been taken."})
			return
		}
		web.Flash(w, "Saved.")
		http.Redirect(w, r, "/settings/profile", http.StatusFound)
	}))
	mux.HandleFunc("POST /settings/profile/delete", a.Auth(func(w http.ResponseWriter, r *http.Request) {
		u := web.CurrentUser(r)
		if !CheckPassword(a.DB, u.ID, r.FormValue("password")) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			a.Render(w, r, "auth/profile.html", map[string]any{"Error": "The password is incorrect."})
			return
		}
		a.Logout(w, r)
		a.DB.Exec(`DELETE FROM sessions WHERE user_id=?`, u.ID)
		a.DB.Exec(`DELETE FROM users WHERE id=?`, u.ID)
		http.Redirect(w, r, cp("/login"), http.StatusFound)
	}))
	mux.HandleFunc("GET /settings/security", a.Auth(a.ConfirmPassword(func(w http.ResponseWriter, r *http.Request) { a.Render(w, r, "auth/security.html", nil) })))
	mux.HandleFunc("POST /settings/security/password", a.Auth(a.ConfirmPassword(func(w http.ResponseWriter, r *http.Request) {
		u := web.CurrentUser(r)
		pw := r.FormValue("password")
		if !CheckPassword(a.DB, u.ID, r.FormValue("current_password")) || len(pw) < 8 || pw != r.FormValue("password_confirmation") {
			w.WriteHeader(http.StatusUnprocessableEntity)
			a.Render(w, r, "auth/security.html", map[string]any{"Error": "Current password wrong, or new password invalid (min 8, must match)."})
			return
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(pw), 12)
		a.DB.Exec(`UPDATE users SET password=?,updated_at=? WHERE id=?`, hash, web.Now(), u.ID)
		web.Flash(w, "Password updated.")
		http.Redirect(w, r, "/settings/security", http.StatusFound)
	})))
	mux.HandleFunc("GET /settings/appearance", a.Auth(func(w http.ResponseWriter, r *http.Request) { a.Render(w, r, "auth/appearance.html", nil) }))
}

// SendResetLink = Password::sendResetLink(): token baru + email. Dipakai "Forgot password" dan aksi admin di modul Users.
func SendResetLink(a *web.App, email string) {
	tok := web.Token()
	a.DB.Exec(`REPLACE INTO password_reset_tokens(email,token,created_at) VALUES(?,?,?)`, email, tok, web.Now())
	mail.Send(email, "Reset Password Notification", "Reset your password: "+os.Getenv("APP_URL")+a.CP("/reset-password/")+tok+"?email="+email+"\nThis link expires in 60 minutes.")
}

func SendVerification(a *web.App, id, email string) {
	tok := web.Token()
	a.DB.Exec(`REPLACE INTO cache(cache.key,value,expiration) VALUES(?,?,?)`, "verify:"+id, tok, time.Now().Add(time.Hour).Unix())
	mail.Send(email, "Verify Email Address", "Click to verify: "+os.Getenv("APP_URL")+a.CP("/verify-email/")+id+"/"+tok)
}

func CheckPassword(db *sql.DB, userID, pw string) bool {
	var hash string
	db.QueryRow(`SELECT password FROM users WHERE id=?`, userID).Scan(&hash)
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}
