package users

import (
	"encoding/json"
	"fmt"

	"draftcms-lite/internal/web"
)

// Perm = satu checkbox permission (PermissionRegistry::permission()).
type Perm struct {
	Value       string  `json:"value"`
	Label       string  `json:"label"`
	Description *string `json:"description"`
	Children    []Perm  `json:"children"`
}

type Group struct {
	Group       string `json:"group"`
	Permissions []Perm `json:"permissions"`
}

func p(value, label string, desc ...string) Perm {
	x := Perm{Value: value, Label: label, Children: []Perm{}}
	if len(desc) > 0 {
		x.Description = &desc[0]
	}
	return x
}

func withChildren(x Perm, c []Perm) Perm { x.Children = c; return x }

// Groups = PermissionRegistry::groups(): daftar checkbox yang bisa diberikan ke sebuah role. Grup per resource
// (collections, navigation, ...) dibangun dari isi DB, grup kosong dibuang.
func Groups(a *web.App) []Group {
	type res struct{ handle, title, settings string }
	load := func(q string) (out []res) {
		rows, err := a.DB.Query(q)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var r res
			var s []byte
			rows.Scan(&r.handle, &r.title, &s)
			r.settings = string(s)
			out = append(out, r)
		}
		return
	}

	cp := Group{"Control Panel", []Perm{
		p("access cp", "Access the Control Panel", "Allows access into the control panel, but doesn't guarantee anything can be done once inside."),
		p("configure sites", "Configure Sites", "Ability to configure sites when multi-site is enabled."),
		p("configure fields", "Configure Fields", "Ability to edit blueprints, fieldsets, and their fields."),
		p("configure form fields", "Configure Form Fields", "Ability to edit form blueprints, fieldsets, and their fields."),
		p("manage preferences", "Manage Preferences", "Ability to customize global and role-specific preferences."),
	}}

	sites := Group{Group: "Sites"}
	for _, s := range load("SELECT handle,name,NULL FROM sites ORDER BY `order`") {
		sites.Permissions = append(sites.Permissions, p(fmt.Sprintf("access %s site", s.handle), fmt.Sprintf("Access %s (%s) site", s.title, s.handle)))
	}

	cols := Group{"Collections", []Perm{p("configure collections", "Configure Collections", "Grants access to all collection related permissions")}}
	for _, c := range load("SELECT handle,title,settings FROM collections ORDER BY handle") {
		h := c.handle
		kids := []Perm{
			p("edit "+h+" entries", "Edit entries"),
			p("create "+h+" entries", "Create new entries"),
			p("delete "+h+" entries", "Delete entries"),
			p("publish "+h+" entries", "Manage publish state", "Ability to change from draft to published and vice versa"),
		}
		var st struct{ Orderable bool }
		json.Unmarshal([]byte(c.settings), &st)
		if st.Orderable {
			kids = append(kids, p("reorder "+h+" entries", "Reorder entries", "Enables drag and drop reordering"))
		}
		// "other authors'": entry dengan Meta Author hanya bisa diubah orang lain bila punya varian ini.
		kids = append(kids,
			p("edit other authors "+h+" entries", "Edit other authors' entries"),
			p("publish other authors "+h+" entries", "Manage publish state of other authors' entries"),
			p("delete other authors "+h+" entries", "Delete other authors' entries"))
		cols.Permissions = append(cols.Permissions, withChildren(p("view "+h+" entries", "View "+c.title+" entries"), kids))
	}

	nav := Group{"Navigation", []Perm{p("configure navigation", "Configure Navigation", "Grants access to all navigation related permissions")}}
	for _, n := range load("SELECT handle,title,NULL FROM navigations ORDER BY handle") {
		nav.Permissions = append(nav.Permissions, withChildren(p("view "+n.handle+" navigation", "View "+n.title+" navigation"),
			[]Perm{p("edit "+n.handle+" navigation", "Edit navigation"), p("delete "+n.handle+" navigation", "Delete navigation")}))
	}

	glob := Group{"Globals", []Perm{p("configure globals", "Configure Globals", "Grants access to all global related permissions")}}
	for _, g := range load("SELECT handle,title,NULL FROM global_sets ORDER BY handle") {
		glob.Permissions = append(glob.Permissions, withChildren(p("view "+g.handle+" globals", "View "+g.title+" globals"),
			[]Perm{p("edit "+g.handle+" globals", "Edit globals"), p("delete "+g.handle+" globals", "Delete globals")}))
	}

	tax := Group{"Taxonomies", []Perm{p("configure taxonomies", "Configure Taxonomies", "Grants access to all taxonomy related permissions")}}
	for _, t := range load("SELECT handle,title,NULL FROM taxonomies ORDER BY handle") {
		tax.Permissions = append(tax.Permissions, withChildren(p("view "+t.handle+" terms", "View "+t.title+" terms"),
			[]Perm{p("edit "+t.handle+" terms", "Edit terms"), p("create "+t.handle+" terms", "Create new terms"), p("delete "+t.handle+" terms", "Delete terms")}))
	}

	assets := Group{"Assets", []Perm{p("configure assets", "Configure Asset Containers", "Grants access to all asset related permissions")}}
	for _, c := range load("SELECT handle,title,NULL FROM asset_containers ORDER BY handle") {
		h := c.handle
		assets.Permissions = append(assets.Permissions, withChildren(p("view "+h+" assets", "View "+c.title+" assets"), []Perm{
			p("upload "+h+" assets", "Upload new assets"), p("edit folders "+h+" assets", "Edit folders"), p("edit "+h+" assets", "Edit assets"),
			p("move "+h+" assets", "Move assets"), p("rename "+h+" assets", "Rename assets"), p("delete "+h+" assets", "Delete assets"),
		}))
	}

	usr := Group{"Users", []Perm{
		p("view users", "View users"), p("edit users", "Edit users"), p("create users", "Create users"), p("delete users", "Delete users"),
		p("ban users", "Ban users", "Prevents a user from logging in until unbanned."),
		p("change passwords", "Change passwords"),
		p("send password reset", "Send password reset link", "Emails the user a link to reset their own password."),
		p("assign user groups", "Assign groups to users"), p("assign user roles", "Assign roles to users"),
		p("edit user groups", "Edit groups"), p("edit roles", "Edit roles"),
	}}

	forms := Group{"Forms", []Perm{p("configure forms", "Configure forms", "Grants access to all form related permissions")}}
	log := Group{"Activity Log", []Perm{
		p("view activity log", "Access activity log", "Ability to open the admin activity log page."),
		p("view activity log detail", "View entry detail", "Ability to view the full before/after change details of a logged entry."),
		p("export activity log", "Export activity log", "Ability to export activity log entries (per month/year) as a JSON file."),
		p("import activity log", "Import activity log", "Ability to restore activity log entries back from a previously exported file."),
	}}
	pkg := Group{"Package Updates", []Perm{p("view package updates", "View package updates", "Ability to open the admin page listing available Composer/npm package updates.")}}

	var out []Group
	for _, g := range []Group{cp, sites, cols, nav, glob, tax, assets, usr, forms, log, pkg} {
		if len(g.Permissions) > 0 {
			out = append(out, g)
		}
	}
	return out
}

func flatten(ps []Perm) (out []string) {
	for _, x := range ps {
		out = append(out, x.Value)
		out = append(out, flatten(x.Children)...)
	}
	return
}

// Values = PermissionRegistry::values(): semua nilai valid (untuk validasi + prune).
func Values(a *web.App) (out []string) {
	for _, g := range Groups(a) {
		out = append(out, flatten(g.Permissions)...)
	}
	return
}
