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

func Save(db *sql.DB, handle, typ string, items []Item) error {
	b, _ := json.Marshal(items)
	_, err := db.Exec(`INSERT INTO trees(handle,type,tree,created_at,updated_at) VALUES(?,?,?,?,?)
		ON DUPLICATE KEY UPDATE tree=VALUES(tree), updated_at=VALUES(updated_at)`, handle, typ, string(b), web.Now(), web.Now())
	return err
}
