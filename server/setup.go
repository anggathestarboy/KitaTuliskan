package main

import (
	"flag"
	"fmt"
	"log"

	"draftcms-lite/internal/modules/navigation"
	"draftcms-lite/internal/modules/users"
	"draftcms-lite/internal/web"
)

// setup = `php artisan draft:setup`: super admin, media container, default site, pages collection,
// homepage blueprint + entry, sidebar admin. Idempoten: pakai INSERT IGNORE.
func setup(a *web.App, args []string) {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	name := fs.String("name", "Admin", "admin name")
	email := fs.String("email", "admin@example.com", "admin email")
	pw := fs.String("password", "", "admin password (min 8)")
	siteName := fs.String("site-name", "My Site", "default site name")
	siteHandle := fs.String("site-handle", "indonesia", "default site handle")
	fs.Parse(args)
	if len(*pw) < 8 {
		log.Fatal("setup: -password wajib, min 8 karakter")
	}
	now := web.Now()
	var n int
	a.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE email=?`, *email).Scan(&n)
	if n == 0 {
		if _, _, err := users.Create(a, *name, *email, *pw, "super_admin"); err != nil {
			log.Fatal(err)
		}
		fmt.Println("✓ super admin", *email)
	}
	a.DB.Exec(`INSERT IGNORE INTO asset_containers(handle,title,disk,created_at,updated_at) VALUES('assets','Assets','public',?,?)`, now, now)
	a.DB.Exec(`INSERT IGNORE INTO sites(handle,name,url,locale,lang,attributes,created_at,updated_at) VALUES(?,?,'/','id_ID','id','{}',?,?)`, *siteHandle, *siteName, now, now)
	a.DB.Exec(`INSERT IGNORE INTO collections(handle,title,settings,created_at,updated_at) VALUES('pages','Pages','{"route":"/{slug}"}',?,?)`, now, now)
	a.DB.Exec(`INSERT IGNORE INTO blueprints(namespace,handle,data,created_at,updated_at) VALUES('collections.pages','homepage',
		'{"title":"Homepage","template":"homepage","fields":[{"handle":"title","type":"text","display":"Title","required":true},{"handle":"content","type":"markdown","display":"Content"}]}',?,?)`, now, now)
	a.DB.QueryRow(`SELECT COUNT(*) FROM entries WHERE collection='pages' AND slug='home'`).Scan(&n)
	if n == 0 {
		a.DB.Exec(`INSERT INTO entries(id,site,published,status,slug,uri,collection,blueprint,data,created_at,updated_at) VALUES(?,?,1,'published','home','/','pages','homepage',?,?,?)`,
			web.UUID(), *siteHandle, `{"title":"Welcome to Draft CMS","content":"If your dreams don't scare you, they are not big enough."}`, now, now)
	}
	navigation.SeedAdminSidebar(a)
	fmt.Println("✓ setup selesai. Login di", a.CP("/login"))
}
