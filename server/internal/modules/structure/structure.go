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
	b, err := json.Marshal(items)
	if err != nil {
		return err
	}
	var id int64
	err = db.QueryRow(`SELECT id FROM trees WHERE handle=? AND type=? AND locale IS NULL ORDER BY id LIMIT 1`, handle, typ).Scan(&id)
	if err == sql.ErrNoRows {
		_, err = db.Exec(`INSERT INTO trees(handle,type,tree,created_at,updated_at) VALUES(?,?,?,?,?)`, handle, typ, string(b), web.Now(), web.Now())
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
