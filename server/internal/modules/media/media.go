// Package media = app/Modules/Media: asset containers + upload ke storage/<container>/ + serve file.
package media

import (
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"draftcms-lite/internal/web"
)

const Root = "storage" // = disk "public" di source

func Register(mux *http.ServeMux, a *web.App) {
	a.Page(mux, "/media", "media/index.html", func(r *http.Request) map[string]any {
		containers, _ := web.Rows(a.DB, `SELECT * FROM asset_containers ORDER BY handle`)
		c := r.URL.Query().Get("container")
		if c == "" && len(containers) > 0 {
			c = containers[0]["handle"].(string)
		}
		assets, _ := web.Rows(a.DB, `SELECT * FROM assets_meta WHERE container=? ORDER BY path`, c)
		return map[string]any{"Containers": containers, "Container": c, "Assets": assets}
	})
	a.Admin(mux, "POST /media/containers", func(w http.ResponseWriter, r *http.Request) {
		h := r.FormValue("handle")
		if !web.ValidHandle(h) {
			web.Flash(w, "Invalid handle.")
		} else if _, err := a.DB.Exec(`INSERT INTO asset_containers(handle,title,disk,created_at,updated_at) VALUES(?,?,'public',?,?)`, h, r.FormValue("title"), web.Now(), web.Now()); err != nil {
			web.Flash(w, err.Error())
		}
		http.Redirect(w, r, a.CP("/media?container="+h), http.StatusFound)
	})
	a.Admin(mux, "POST /media/upload", func(w http.ResponseWriter, r *http.Request) {
		c := r.URL.Query().Get("container")
		if !web.ValidHandle(c) {
			http.Error(w, "bad container", 400)
			return
		}
		r.ParseMultipartForm(64 << 20)
		for _, fh := range r.MultipartForm.File["file"] {
			name := filepath.Base(fh.Filename) // cegah path traversal
			if name == "." || name == "" {
				continue
			}
			os.MkdirAll(filepath.Join(Root, c), 0o755)
			dst, err := os.Create(filepath.Join(Root, c, name))
			if err != nil {
				continue
			}
			src, _ := fh.Open()
			io.Copy(dst, src)
			src.Close()
			dst.Close()
			ext := strings.TrimPrefix(filepath.Ext(name), ".")
			a.DB.Exec(`INSERT INTO assets_meta(container,folder,basename,filename,extension,path,created_at,updated_at) VALUES(?,'',?,?,?,?,?,?)
				ON DUPLICATE KEY UPDATE updated_at=VALUES(updated_at)`, c, name, strings.TrimSuffix(name, "."+ext), ext, name, web.Now(), web.Now())
		}
		http.Redirect(w, r, a.CP("/media?container="+c), http.StatusFound)
	})
	a.Admin(mux, "POST /media/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		var c, p string
		if a.DB.QueryRow(`SELECT container,path FROM assets_meta WHERE id=?`, r.PathValue("id")).Scan(&c, &p) == nil {
			os.Remove(filepath.Join(Root, c, filepath.Base(p)))
			a.DB.Exec(`DELETE FROM assets_meta WHERE id=?`, r.PathValue("id"))
		}
		http.Redirect(w, r, a.CP("/media?container="+c), http.StatusFound)
	})
	// media.file: GET /admin-panel/media/file/{container}/{path...}
	a.Admin(mux, "GET /media/file/{container}/{path...}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(Root, web.CleanSegment(r.PathValue("container")), path.Clean("/"+r.PathValue("path"))))
	})
	// publik untuk Astro: /storage/{container}/{path} (= storage:link di source)
	mux.Handle("GET /storage/", http.StripPrefix("/storage/", http.FileServer(http.Dir(Root))))
}
