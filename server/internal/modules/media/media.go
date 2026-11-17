// Package media = app/Modules/Media: asset containers + folder/file ops. UI = React island (resources/ts/media),
// data lewat JSON API di bawah /media/api. Container dibuat lewat CLI: `make media {nama}` (lihat cli.go).
package media

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"draftcms-lite/internal/web"
)

const maxUploadKB = 51200 // validasi 'uploads.*' => max:51200 di source

var errForbidden = errors.New("Forbidden")

type item struct{ Type, Key string }

func perm(verb, handle string) string { return verb + " " + handle + " assets" }

func Register(mux *http.ServeMux, a *web.App) {
	s := Store{a.DB}

	a.Page(mux, "/media", "media/index.html", nil)

	// api membungkus: container dari ?container=, wajib punya "view {container} assets", error → JSON {error}.
	api := func(pattern string, h func(w http.ResponseWriter, r *http.Request, u *web.User, c Container) (any, error)) {
		a.Admin(mux, pattern, func(w http.ResponseWriter, r *http.Request) {
			u := CurrentUser(r)
			c, err := s.Container(r.URL.Query().Get("container"))
			if err == nil && !u.Can(perm("view", c.Handle)) {
				err = errForbidden
			}
			var out any
			if err == nil {
				out, err = h(w, r, u, c)
			}
			reply(w, out, err)
		})
	}

	// GET listing: sekaligus bootstrap UI (container yang boleh dilihat user, permission, breadcrumbs dihitung di client).
	a.Admin(mux, "GET /media/api/listing", func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r)
		all, err := s.Containers()
		var vis []Container
		for _, c := range all {
			if u.Can(perm("view", c.Handle)) {
				vis = append(vis, c)
			}
		}
		res := map[string]any{"containers": []map[string]string{}, "container": "", "path": "", "folders": []Folder{}, "files": []any{}, "can": map[string]bool{}}
		cs := make([]map[string]string, 0, len(vis))
		for _, c := range vis {
			cs = append(cs, map[string]string{"handle": c.Handle, "title": c.Title})
		}
		res["containers"] = cs
		q := r.URL.Query()
		if h := q.Get("container"); h != "" && !u.Can(perm("view", h)) {
			if _, e := s.Container(h); e == nil { // container ada tapi tidak boleh dilihat → 403 (mount() di source)
				reply(w, nil, errForbidden)
				return
			}
		}
		var cur *Container
		for i := range vis {
			if vis[i].Handle == q.Get("container") || (cur == nil && i == 0) {
				cur = &vis[i]
			}
		}
		if err == nil && cur != nil {
			p, e := clean(q.Get("path"))
			if err = e; err == nil {
				var folders []Folder
				var files []Asset
				if t := q.Get("search"); t != "" {
					folders, files, err = s.Search(*cur, t)
				} else {
					folders, files, err = s.List(*cur, p)
				}
				out := make([]any, 0, len(files))
				for _, f := range files {
					out = append(out, view(a, *cur, f))
				}
				res["container"], res["path"], res["folders"], res["files"] = cur.Handle, p, folders, out
				res["can"] = map[string]bool{
					"upload": u.Can(perm("upload", cur.Handle)), "editFolders": u.Can(perm("edit folders", cur.Handle)), "edit": u.Can(perm("edit", cur.Handle)),
					"move": u.Can(perm("move", cur.Handle)), "rename": u.Can(perm("rename", cur.Handle)), "delete": u.Can(perm("delete", cur.Handle)),
				}
			}
		}
		reply(w, res, err)
	})

	api("POST /media/api/folder", func(w http.ResponseWriter, r *http.Request, u *web.User, c Container) (any, error) {
		var in struct{ Op, Path, Name string }
		if err := decode(w, r, &in); err != nil {
			return nil, err
		}
		if !u.Can(perm("edit folders", c.Handle)) {
			return nil, errForbidden
		}
		p, err := clean(in.Path)
		if err != nil {
			return nil, err
		}
		switch in.Op {
		case "create":
			n, err := name(in.Name)
			if err != nil {
				return nil, err
			}
			return nil, s.CreateFolder(c, join(p, n))
		case "rename":
			n, err := name(in.Name)
			if err != nil {
				return nil, err
			}
			return nil, s.MoveFolder(c, p, join(dir(p), n))
		}
		return nil, Error("Unknown operation.")
	})

	// paste: cut = pindah (juga dipakai drag-drop ke folder), copy = salin. to = folder tujuan.
	api("POST /media/api/paste", func(w http.ResponseWriter, r *http.Request, u *web.User, c Container) (any, error) {
		var in struct {
			Mode  string
			Items []item
			To    string
		}
		if err := decode(w, r, &in); err != nil {
			return nil, err
		}
		to, err := clean(in.To)
		if err != nil {
			return nil, err
		}
		cut := in.Mode == "cut"
		if cut {
			if !u.Can(perm("move", c.Handle)) {
				return nil, errForbidden
			}
		} else if err := authorize(u, c, in.Items, "edit", "edit folders"); err != nil {
			return nil, err
		}
		moved := 0
		for _, it := range in.Items {
			if it.Type == "folder" {
				k, err := clean(it.Key)
				if err != nil {
					return nil, err
				}
				if !cut {
					err = s.CopyFolder(c, k, to)
				} else if dst := join(to, path.Base(k)); dst != k && to != k && !strings.HasPrefix(to, k+"/") { // pindah ke diri sendiri = no-op
					err = s.MoveFolder(c, k, dst)
				} else {
					continue
				}
				if err != nil {
					return nil, err
				}
				moved++
				continue
			}
			as, err := s.Asset(c, it.Key)
			if err == sql.ErrNoRows {
				continue
			} else if err != nil {
				return nil, err
			}
			if !cut {
				err = s.CopyFile(c, as, to)
			} else if as.Folder != to {
				err = s.MoveFile(c, as, to)
			} else {
				continue
			}
			if err != nil {
				return nil, err
			}
			moved++
		}
		return map[string]int{"moved": moved}, nil
	})

	api("POST /media/api/duplicate", func(w http.ResponseWriter, r *http.Request, u *web.User, c Container) (any, error) {
		return nil, each(w, r, u, c, s, "edit", "edit folders", func(as *Asset, folder string) error {
			if as != nil {
				return s.CopyFile(c, *as, as.Folder)
			}
			return s.DuplicateFolder(c, folder)
		})
	})

	api("POST /media/api/delete", func(w http.ResponseWriter, r *http.Request, u *web.User, c Container) (any, error) {
		return nil, each(w, r, u, c, s, "delete", "edit folders", func(as *Asset, folder string) error {
			if as != nil {
				return s.DeleteFile(c, *as)
			}
			return s.DeleteFolder(c, folder)
		})
	})

	api("POST /media/api/rename", func(w http.ResponseWriter, r *http.Request, u *web.User, c Container) (any, error) {
		var in struct{ ID, Name string }
		if err := decode(w, r, &in); err != nil {
			return nil, err
		}
		if !u.Can(perm("rename", c.Handle)) {
			return nil, errForbidden
		}
		n, err := name(in.Name)
		if err != nil {
			return nil, err
		}
		as, err := s.Asset(c, in.ID)
		if err != nil {
			return nil, err
		}
		if as.Extension != "" {
			n += "." + as.Extension
		}
		return nil, s.RenameFile(c, as, n)
	})

	api("POST /media/api/meta", func(w http.ResponseWriter, r *http.Request, u *web.User, c Container) (any, error) {
		var in struct{ ID, Title, Alt, Loading string }
		if err := decode(w, r, &in); err != nil {
			return nil, err
		}
		if !u.Can(perm("edit", c.Handle)) {
			return nil, errForbidden
		}
		if len(in.Title) > 255 || len(in.Alt) > 255 || (in.Loading != "lazy" && in.Loading != "eager") {
			return nil, Error("The given data was invalid.")
		}
		as, err := s.Asset(c, in.ID)
		if err != nil {
			return nil, err
		}
		return nil, s.SaveMeta(as, in.Title, in.Alt, in.Loading)
	})

	api("POST /media/api/upload", func(w http.ResponseWriter, r *http.Request, u *web.User, c Container) (any, error) {
		if !u.Can(perm("upload", c.Handle)) {
			return nil, errForbidden
		}
		folder, err := clean(r.URL.Query().Get("path"))
		if err != nil {
			return nil, err
		}
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return nil, Error("The upload could not be read.")
		}
		failures := []string{}
		for _, fh := range r.MultipartForm.File["file"] {
			if fh.Size > maxUploadKB*1024 {
				failures = append(failures, fmt.Sprintf("%s must not be greater than %d kilobytes.", fh.Filename, maxUploadKB))
			} else if err := s.Upload(c, folder, fh); err != nil {
				var me Error
				if !errors.As(err, &me) {
					return nil, err
				}
				failures = append(failures, me.Error())
			}
		}
		return map[string]any{"failures": failures}, nil
	})

	// download file/folder (folder = zip, di-stream tanpa file sementara).
	a.Admin(mux, "GET /media/download", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		c, err := s.Container(q.Get("container"))
		if err == nil && !CurrentUser(r).Can(perm("view", c.Handle)) {
			err = errForbidden
		}
		if err != nil {
			reply(w, nil, err)
			return
		}
		if q.Get("type") == "folder" {
			p, err := clean(q.Get("key"))
			if err != nil {
				reply(w, nil, err)
				return
			}
			zipFolder(w, c, p)
			return
		}
		as, err := s.Asset(c, q.Get("key"))
		if err != nil {
			reply(w, nil, err)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''`+url.PathEscape(as.Basename))
		serve(w, r, c, as.Path)
	})

	// media.file: file container private, hanya untuk user dengan "view {container} assets". Range (video) ditangani http.ServeContent.
	a.Admin(mux, "GET /media/file/{container}/{path...}", func(w http.ResponseWriter, r *http.Request) {
		c, err := s.Container(r.PathValue("container"))
		if err == nil && !CurrentUser(r).Can(perm("view", c.Handle)) {
			err = errForbidden
		}
		p, perr := clean(r.PathValue("path"))
		if err != nil || perr != nil || p == "" {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		serve(w, r, c, p)
	})

	// publik untuk Astro: /storage/{container}/{path} (= storage:link di source); hanya disk public.
	mux.Handle("GET /storage/", http.StripPrefix("/storage/", http.FileServer(http.Dir(PublicRoot))))
}

func CurrentUser(r *http.Request) *web.User { return web.CurrentUser(r) }

func serve(w http.ResponseWriter, r *http.Request, c Container, rel string) {
	f, err := os.Open(c.abs(rel))
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	http.ServeContent(w, r, path.Base(rel), st.ModTime(), f)
}

func zipFolder(w http.ResponseWriter, c Container, p string) {
	n := path.Base(p)
	if p == "" {
		n = c.Handle
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''`+url.PathEscape(n+".zip"))
	zw := zip.NewWriter(w)
	defer zw.Close()
	base := c.abs(p)
	filepath.WalkDir(base, func(fp string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(base, fp)
		in, err := os.Open(fp)
		if err != nil {
			return nil
		}
		defer in.Close()
		if out, err := zw.Create(filepath.ToSlash(rel)); err == nil {
			io.Copy(out, in)
		}
		return nil
	})
}

// authorize = authorizeItemTypes: izin per tipe item dicek di depan, sebelum ada yang diubah.
func authorize(u *web.User, c Container, items []item, filePerm, folderPerm string) error {
	for _, it := range items {
		v := filePerm
		if it.Type == "folder" {
			v = folderPerm
		}
		if !u.Can(perm(v, c.Handle)) {
			return errForbidden
		}
	}
	return nil
}

// each menjalankan fn untuk tiap item JSON {items:[{type,key}]}: file → asset terisi, folder → path.
func each(w http.ResponseWriter, r *http.Request, u *web.User, c Container, s Store, filePerm, folderPerm string, fn func(*Asset, string) error) error {
	var in struct{ Items []item }
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if err := authorize(u, c, in.Items, filePerm, folderPerm); err != nil {
		return err
	}
	for _, it := range in.Items {
		if it.Type == "folder" {
			p, err := clean(it.Key)
			if err == nil {
				err = fn(nil, p)
			}
			if err != nil {
				return err
			}
			continue
		}
		as, err := s.Asset(c, it.Key)
		if err == sql.ErrNoRows {
			continue
		}
		if err == nil {
			err = fn(&as, "")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func decode(w http.ResponseWriter, r *http.Request, v any) error {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		return Error("Invalid request.")
	}
	return nil
}

func reply(w http.ResponseWriter, out any, err error) {
	w.Header().Set("Content-Type", "application/json")
	var me Error
	switch {
	case err == nil:
		if out == nil {
			out = map[string]bool{"ok": true}
		}
	case errors.Is(err, errForbidden):
		w.WriteHeader(http.StatusForbidden)
		out = map[string]string{"error": "Forbidden"}
	case errors.Is(err, sql.ErrNoRows):
		w.WriteHeader(http.StatusNotFound)
		out = map[string]string{"error": "Not found."}
	case errors.As(err, &me):
		w.WriteHeader(http.StatusUnprocessableEntity)
		out = map[string]string{"error": me.Error()}
	default:
		log.Println("media:", err)
		w.WriteHeader(http.StatusInternalServerError)
		out = map[string]string{"error": "Something went wrong."}
	}
	json.NewEncoder(w).Encode(out)
}

// view = bentuk asset untuk UI: accessor AssetMeta (url, copy_url, title, alt, loading, size, mime_type, ...).
func view(a *web.App, c Container, as Asset) map[string]any {
	num := func(k string) float64 { v, _ := as.Meta[k].(float64); return v }
	str := func(k string) string { d, _ := as.Meta["data"].(map[string]any); v, _ := d[k].(string); return v }
	mt, _ := as.Meta["mime_type"].(string)
	human := titleCase(strings.NewReplacer("-", " ", "_", " ").Replace(as.Filename))
	or := func(v, d string) string {
		if v != "" {
			return v
		}
		return d
	}
	u := "/storage/" + escape(c.Handle+"/"+as.Path)
	if c.Disk != "public" {
		u = a.CP("/media/file/" + escape(c.Handle+"/"+as.Path))
	}
	return map[string]any{
		"id": as.ID, "basename": as.Basename, "filename": as.Filename, "extension": as.Extension, "folder": as.Folder, "path": as.Path,
		"size": int64(num("size")), "mime_type": mt, "width": int(num("width")), "height": int(num("height")),
		"title": or(str("title"), human), "alt": or(str("alt"), human), "loading": or(str("loading"), "lazy"),
		"url": u, "copy_url": strings.TrimRight(os.Getenv("APP_URL"), "/") + u, "updated_at": as.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func escape(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

// titleCase = Str::title (huruf pertama tiap kata kapital, sisanya kecil), spasi dirapatkan (squish).
func titleCase(s string) string {
	w := strings.Fields(s)
	for i, x := range w {
		r := []rune(strings.ToLower(x))
		r[0] = []rune(strings.ToUpper(string(r[0])))[0]
		w[i] = string(r)
	}
	return strings.Join(w, " ")
}
