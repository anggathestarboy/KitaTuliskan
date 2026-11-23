package main

import (
	"bufio"
	"flag"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"slices"
	"strings"

	"draftcms-lite/internal/db"
	"draftcms-lite/internal/modules/about"
	"draftcms-lite/internal/modules/activitylog"
	"draftcms-lite/internal/modules/auth"
	"draftcms-lite/internal/modules/blueprints"
	"draftcms-lite/internal/modules/collections"
	"draftcms-lite/internal/modules/forms"
	"draftcms-lite/internal/modules/globals"
	"draftcms-lite/internal/modules/media"
	"draftcms-lite/internal/modules/navigation"
	"draftcms-lite/internal/modules/notfounddetector"
	"draftcms-lite/internal/modules/packageupdates"
	"draftcms-lite/internal/modules/redirects"
	"draftcms-lite/internal/modules/sitemap"
	"draftcms-lite/internal/modules/sites"
	"draftcms-lite/internal/modules/sitesettings"
	"draftcms-lite/internal/modules/taxonomies"
	"draftcms-lite/internal/modules/users"
	"draftcms-lite/internal/web"
)

// = bootstrap/providers.php: satu baris per modul. Structure tidak punya route.
var modules = []func(*http.ServeMux, *web.App){
	auth.Register, users.Register,
	collections.Register, taxonomies.Register, blueprints.Register, globals.Register, media.Register, forms.Register,
	navigation.Register, sites.Register, sitemap.Register, sitesettings.Register, redirects.Register,
	notfounddetector.Register, packageupdates.Register, about.Register,
}

func main() {
	loadEnv(".env")
	d, err := db.Open(env("DB_HOST", "127.0.0.1"), env("DB_PORT", "3306"), env("DB_USERNAME", "root"), os.Getenv("DB_PASSWORD"), env("DB_DATABASE", "draftcms_lite"), len(os.Args) > 1 && os.Args[1] == "fresh")
	if err != nil {
		log.Fatal(err)
	}
	app := web.New(d, env("CMS_ROUTE", "admin-panel"), "templates")
	users.EnsureSuperAdminRole(app)

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "setup":
			setup(app, os.Args[2:])
		case "fresh": // drop + recreate schema, lalu setup
			setup(app, os.Args[2:])
		case "media": // make media {title} = php artisan make:media; flag di depan judul: media -type private "Judul"
			fs := flag.NewFlagSet("media", flag.ExitOnError)
			kind := fs.String("type", "", "public | private (kosong = tanya)")
			fs.Parse(os.Args[2:])
			if err := media.Make(app, strings.TrimSpace(fs.Arg(0)), *kind); err != nil {
				log.Fatal(err)
			}
		case "sidebar": // = php artisan admin:sidebar {--force}
			fs := flag.NewFlagSet("sidebar", flag.ExitOnError)
			force := fs.Bool("force", false, "reset tree sidebar ke default bila sudah ada")
			fs.Parse(os.Args[2:])
			if navigation.SeedAdminSidebar(app, *force) {
				log.Println("Admin sidebar navigation seeded.")
			} else {
				log.Println("The admin sidebar navigation already exists. Pass -force to reset it back to these defaults.")
			}
		case "activity_log": // = php artisan draft:activity_log (tabel sudah dibuat db.Open lewat schema.sql)
			if err := activitylog.Enable(".env"); err != nil {
				log.Fatal(err)
			}
			log.Println("Activity log enabled. Restart the server to apply.")
		case "activity_log:process": // = php artisan activity-log:process-deletions
			e, x := activitylog.Process(app)
			log.Printf("Dispatched %d deletion(s), expired %d stale request(s).", e, x)
		case "media:remove":
			if len(os.Args) < 3 {
				log.Fatal("usage: media:remove {handle}")
			}
			if err := media.Remove(app, os.Args[2]); err != nil {
				log.Fatal(err)
			}
		}
		return
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	// routes/web.php: '/' → redirect dashboard; dashboard butuh 'permission:access cp'.
	mux.HandleFunc("GET "+app.CP("/{$}"), func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, app.CP("/dashboard"), http.StatusFound)
	})
	app.Admin(mux, "GET /dashboard", func(w http.ResponseWriter, r *http.Request) { app.Render(w, r, "dashboard/index.html", nil) })

	for _, reg := range modules {
		reg(mux, app)
	}
	if activitylog.Enabled() {
		activitylog.Register(mux, app)
	}

	mux.HandleFunc("GET /api/assets", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(media.PublicAssets(app))
	})
	// JSON publik untuk Astro (PageController + Blade directives di source diganti fetch saat build).
	mux.HandleFunc("GET /api/{table}", func(w http.ResponseWriter, r *http.Request) {
		t := r.PathValue("table")
		if !slices.Contains([]string{"collections", "entries", "taxonomies", "taxonomy_terms", "global_sets", "global_set_variables", "navigations", "trees", "sites", "redirects", "forms", "blueprints"}, t) {
			http.NotFound(w, r)
			return
		}
		q := "SELECT * FROM " + t
		if t == "entries" {
			q += " WHERE status='published'"
		}
		rows, _ := web.Rows(d, q)
		if rows == nil { // tabel kosong harus [] (bukan null), kalau tidak Astro crash di setup baru
			rows = []map[string]any{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rows)
	})

	// Catch-all publik (= PageController): sitemap/redirect dulu, sisanya diteruskan ke Astro yang merender view blueprint.
	// SITE_URL = `astro dev` (default :4321) atau host hasil build; tanpa itu halaman publik 404.
	site, _ := url.Parse(env("SITE_URL", "http://localhost:4321"))
	astro := httputil.NewSingleHostReverseProxy(site)
	astro.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "Site (Astro) tidak bisa dijangkau di "+site.String()+" — jalankan `npm run dev` di folder site/.", http.StatusBadGateway)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if sitemap.Collection(app, w, r) || redirects.Apply(app, w, r) {
			return
		}
		astro.ServeHTTP(w, r)
	})

	csrf := http.NewCrossOriginProtection() // pengganti @csrf: tolak POST lintas origin
	csrf.AddInsecureBypassPattern("/!/")    // form publik dari Astro boleh lintas origin
	log.Println("listening on", env("ADDR", ":8080"), "admin at", app.CP(""))
	log.Fatal(http.ListenAndServe(env("ADDR", ":8080"), csrf.Handler(app.LoadUser(mux))))
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// loadEnv: parser .env minimal (KEY=VALUE, # komentar). Tidak override env yang sudah ada.
func loadEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if k, v, ok := strings.Cut(line, "="); ok && !strings.HasPrefix(line, "#") && os.Getenv(k) == "" {
			os.Setenv(k, strings.Trim(v, `"'`))
		}
	}
}
