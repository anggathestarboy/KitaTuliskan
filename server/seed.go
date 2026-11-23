package main

import (
	"encoding/json"

	"draftcms-lite/internal/modules/structure"
	"draftcms-lite/internal/web"
)

type m = map[string]any

func fld(handle, typ, display string, extra m) m {
	f := m{"handle": handle, "type": typ, "display": display}
	for k, v := range extra {
		f[k] = v
	}
	return f
}

// menuLoop = field replicator yang dibaca Header/Footer di site (navigation.header.menu_loop → menu_label, menu_url).
func menuLoop() m {
	return fld("menu_loop", "replicator", "Menu", m{"replicator_fields": []m{
		fld("menu_label", "text", "Label", m{"required": true}),
		fld("menu_url", "text", "URL", nil),
	}})
}

func menuRows(items ...[2]string) []m {
	rows := []m{}
	for _, it := range items {
		rows = append(rows, m{"menu_label": it[0], "menu_url": it[1], "_enabled": true})
	}
	return rows
}

// seedContent = contoh konten awal supaya site langsung tampil setelah setup pertama: homepage + about (collection pages),
// navigation header/footer, global site_settings. Idempoten: hanya mengisi yang belum ada, tidak menimpa hasil edit.
func seedContent(a *web.App, siteHandle, siteName string) {
	now := web.Now()
	blueprint := func(ns, handle string, data m) {
		b, _ := json.Marshal(data)
		a.DB.Exec(`INSERT IGNORE INTO blueprints(namespace,handle,data,created_at,updated_at) VALUES(?,?,?,?,?)`, ns, handle, string(b), now, now)
	}
	// template = nama view di site/src/views (blueprint tidak bisa disimpan bila view-nya tidak ada)
	blueprint("collections.pages", "homepage", m{"title": "Homepage", "template": "pages.static.homepage", "fields": []m{
		fld("heading", "text", "Heading", m{"required": true}), fld("subheading", "textarea", "Subheading", nil), fld("content", "bard", "Content", nil)}})
	blueprint("collections.pages", "about", m{"title": "About", "template": "pages.static.about", "fields": []m{
		fld("title", "text", "Title", m{"required": true}), fld("content", "bard", "Content", nil)}})
	blueprint("navigations.header", "header", m{"title": "Header", "fields": []m{menuLoop()}})
	blueprint("navigations.footer", "footer", m{"title": "Footer", "fields": []m{menuLoop()}})
	blueprint("globals.site_settings", "site_settings", m{"title": "Site Settings", "fields": []m{
		fld("site_name", "text", "Site name", nil), fld("tagline", "text", "Tagline", nil)}})

	entry := func(slug, uri, bp string, data m) {
		var n int
		a.DB.QueryRow(`SELECT COUNT(*) FROM entries WHERE collection='pages' AND slug=?`, slug).Scan(&n)
		if n > 0 {
			return
		}
		b, _ := json.Marshal(data)
		a.DB.Exec(`INSERT INTO entries(id,site,published,status,slug,uri,collection,blueprint,data,created_at,updated_at) VALUES(?,?,1,'published',?,?,'pages',?,?,?,?)`,
			web.UUID(), siteHandle, slug, uri, bp, string(b), now, now)
	}
	entry("home", "/", "homepage", m{"heading": "Welcome to " + siteName, "subheading": "Your new site is up and running. Edit this page from the admin panel.",
		"content": "<p>This is example content. Change it in <strong>Collections → Pages → home</strong>.</p>"})
	entry("about", "/about", "about", m{"title": "About", "content": "<p>Tell visitors who you are. Edit this page in <strong>Collections → Pages → about</strong>.</p>"})

	nav := func(handle, title string, rows []m) {
		a.DB.Exec(`INSERT IGNORE INTO navigations(handle,title,settings,created_at,updated_at) VALUES(?,?,'{"blueprint":null}',?,?)`, handle, title, now, now)
		if len(structure.LoadData(a.DB, handle, "navigation", "")) == 0 {
			structure.SaveData(a.DB, handle, "navigation", "", m{"menu_loop": rows})
		}
	}
	nav("header", "Header", menuRows([2]string{"Home", "/"}, [2]string{"About", "/about"}))
	nav("footer", "Footer", menuRows([2]string{"Home", "/"}, [2]string{"About", "/about"}))

	a.DB.Exec(`INSERT IGNORE INTO global_sets(handle,title,settings,created_at,updated_at) VALUES('site_settings','Site Settings','{"blueprint":null,"sites":[],"save_action":"listing"}',?,?)`, now, now)
	var n int
	a.DB.QueryRow(`SELECT COUNT(*) FROM global_set_variables WHERE handle='site_settings'`).Scan(&n)
	if n == 0 {
		b, _ := json.Marshal(m{"site_name": siteName, "tagline": "Powered by Draft CMS"})
		a.DB.Exec(`INSERT INTO global_set_variables(handle,locale,data,created_at,updated_at) VALUES('site_settings',NULL,?,?,?)`, string(b), now, now)
	}
}
