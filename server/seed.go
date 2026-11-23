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

	// Collection pages structured+orderable: root pertama (order 1, uri "") = homepage, sisanya urut di belakangnya.
	tree := []structure.Item{}
	entry := func(slug, uri, bp string, order int, data m) {
		var id string
		if a.DB.QueryRow(`SELECT id FROM entries WHERE collection='pages' AND slug=?`, slug).Scan(&id) != nil {
			id = web.UUID()
			b, _ := json.Marshal(data)
			a.DB.Exec("INSERT INTO entries(id,site,published,status,slug,uri,`order`,collection,blueprint,data,created_at,updated_at) VALUES(?,?,1,'published',?,?,?,'pages',?,?,?,?)",
				id, siteHandle, slug, uri, order, bp, string(b), now, now)
		}
		tree = append(tree, structure.Item{ID: id})
	}
	entry("home", "", "homepage", 1, m{"heading": "Welcome to " + siteName, "subheading": "Your new site is up and running. Edit this page from the admin panel.",
		"content": "<p>This is example content. Change it in <strong>Collections → Pages → home</strong>.</p>"})
	entry("about", "about", "about", 2, m{"title": "About", "content": "<p>Tell visitors who you are. Edit this page in <strong>Collections → Pages → about</strong>.</p>"})
	if len(structure.LoadItems(a.DB, "pages", "collection", siteHandle)) == 0 {
		structure.SaveItems(a.DB, "pages", "collection", siteHandle, tree)
	}

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
