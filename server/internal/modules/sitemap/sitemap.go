// Package sitemap = app/Modules/Sitemap: admin /sitemap + publik /sitemap.xml, /sitemap-{collection}.xml.
package sitemap

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"strings"

	"draftcms-lite/internal/web"
)

func Register(mux *http.ServeMux, a *web.App) {
	a.Page(mux, "/sitemap", "sitemap/index.html", func(r *http.Request) map[string]any {
		rows, _ := web.Rows(a.DB, `SELECT * FROM sitemaps ORDER BY site,collection,url`)
		return map[string]any{"Rows": rows}
	})
	base := os.Getenv("APP_URL")
	mux.HandleFunc("GET /sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		rows, _ := web.Rows(a.DB, `SELECT DISTINCT collection FROM sitemaps WHERE is_active=1`)
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, xml.Header+`<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
		for _, c := range rows {
			fmt.Fprintf(w, "<sitemap><loc>%s/sitemap-%s.xml</loc></sitemap>", base, c["collection"])
		}
		fmt.Fprint(w, "</sitemapindex>")
	})
}

// Collection = /sitemap-{collection}.xml. ServeMux tidak bisa wildcard di tengah segmen, jadi dipanggil dari catch-all main.go.
func Collection(a *web.App, w http.ResponseWriter, r *http.Request) bool {
	c, ok := strings.CutPrefix(r.URL.Path, "/sitemap-")
	if c, ok = strings.CutSuffix(c, ".xml"); !ok || c == "" {
		return false
	}
	{
		rows, _ := web.Rows(a.DB, `SELECT url,changefreq,priority,lastmod FROM sitemaps WHERE is_active=1 AND collection=?`, c)
		base := os.Getenv("APP_URL")
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, xml.Header+`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
		for _, u := range rows {
			fmt.Fprintf(w, "<url><loc>%s%s</loc><changefreq>%s</changefreq><priority>%v</priority></url>", base, u["url"], u["changefreq"], u["priority"])
		}
		fmt.Fprint(w, "</urlset>")
	}
	return true
}
