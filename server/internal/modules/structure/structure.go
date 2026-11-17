// Package structure = app/Modules/Structure: tabel trees (type: collection|navigation). Tanpa route.
// Dipakai Collections (urutan/hierarki entry) dan Navigation (menu). Jangan duplikasi logic tree di modul lain.
package structure

import (
	"database/sql"
	"encoding/json"

	"draftcms-lite/internal/web"
)

type Item struct {
	ID       string          `json:"id,omitempty"`
	Type     string          `json:"type,omitempty"` // internal|external|page
	Title    string          `json:"title,omitempty"`
	URL      string          `json:"url,omitempty"`
	Target   string          `json:"target,omitempty"`
	PageID   string          `json:"page_id,omitempty"`
	Roles    []string        `json:"roles,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
	Children []Item          `json:"children,omitempty"`
}

func Load(db *sql.DB, handle, typ string) []Item {
	var raw sql.NullString
	db.QueryRow(`SELECT tree FROM trees WHERE handle=? AND type=? AND locale IS NULL`, handle, typ).Scan(&raw)
	var items []Item
	json.Unmarshal([]byte(raw.String), &items)
	return items
}

// Save = TreeService::save (updateOrCreate). Tidak memakai ON DUPLICATE KEY: UNIQUE (handle,type,locale) tidak
// berlaku untuk locale NULL di MySQL, jadi tiap simpan akan menambah baris baru dan Load membaca yang terlama.
func Save(db *sql.DB, handle, typ string, items []Item) error {
	if items == nil {
		items = []Item{}
	}
	return put(db, handle, typ, "", items)
}

// LoadData/SaveData = tree sebagai objek per locale (mode blueprint Navigation: nilai field, bukan daftar item). locale "" = NULL.
func LoadData(db *sql.DB, handle, typ, locale string) map[string]any {
	var raw sql.NullString
	where, args := byLocale(locale)
	db.QueryRow(`SELECT tree FROM trees WHERE handle=? AND type=? AND `+where+` ORDER BY id LIMIT 1`, append([]any{handle, typ}, args...)...).Scan(&raw)
	out := map[string]any{}
	json.Unmarshal([]byte(raw.String), &out) // bukan objek (mis. list item lama) → kosong
	return out
}

func SaveData(db *sql.DB, handle, typ, locale string, data map[string]any) error {
	if data == nil {
		data = map[string]any{}
	}
	return put(db, handle, typ, locale, data)
}

func byLocale(locale string) (string, []any) {
	if locale == "" {
		return "locale IS NULL", nil
	}
	return "locale=?", []any{locale}
}

func put(db *sql.DB, handle, typ, locale string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	where, args := byLocale(locale)
	var id int64
	err = db.QueryRow(`SELECT id FROM trees WHERE handle=? AND type=? AND `+where+` ORDER BY id LIMIT 1`, append([]any{handle, typ}, args...)...).Scan(&id)
	if err == sql.ErrNoRows {
		var loc any
		if locale != "" {
			loc = locale
		}
		_, err = db.Exec(`INSERT INTO trees(handle,type,locale,tree,created_at,updated_at) VALUES(?,?,?,?,?,?)`, handle, typ, loc, string(b), web.Now(), web.Now())
		return err
	}
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE trees SET tree=?, updated_at=? WHERE id=?`, string(b), web.Now(), id)
	return err
}

// Delete menghapus semua tree (semua locale) milik satu handle.
func Delete(db *sql.DB, handle, typ string) error {
	_, err := db.Exec(`DELETE FROM trees WHERE handle=? AND type=?`, handle, typ)
	return err
}

// Rename memindahkan tree ke handle baru (dipakai saat handle navigation diubah).
func Rename(db *sql.DB, from, to, typ string) error {
	_, err := db.Exec(`UPDATE trees SET handle=? WHERE handle=? AND type=?`, to, from, typ)
	return err
}
