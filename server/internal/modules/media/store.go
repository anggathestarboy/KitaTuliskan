package media

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"draftcms-lite/internal/web"
)

// Disk "public" = storage/public (dilayani /storage/); disk "local" = storage/private (hanya lewat route ber-auth).
// Nama disk di DB sama dengan source (public|local) supaya data bisa dipindah 1:1.
const (
	PublicRoot  = "storage/public"
	PrivateRoot = "storage/private"
)

// Error = MediaException: pesan aman ditampilkan ke user (HTTP 422).
type Error string

func (e Error) Error() string { return string(e) }

var blockedExt = []string{"php", "php3", "php4", "php5", "php7", "phtml", "pht", "phar", "cgi", "pl", "py", "rb", "sh", "bash", "asp", "aspx", "jsp", "jspx", "exe", "htaccess", "svg", "htm", "html"}

type Container struct {
	Handle, Title, Disk string
	Settings            map[string]any
}

type Asset struct {
	ID                                              int64
	Container, Folder, Basename, Filename, Extension string
	Path                                            string
	Meta                                            map[string]any
	UpdatedAt                                       time.Time
}

type Store struct{ DB *sql.DB }

func root(disk string) string {
	if disk == "public" {
		return PublicRoot
	}
	return PrivateRoot
}

// clean = normalisasi path relatif dalam container; menolak "..", "\" dan segmen kosong (anti path traversal).
func clean(p string) (string, error) {
	var out []string
	for _, s := range strings.Split(p, "/") {
		switch {
		case s == "":
		case s == "." || s == ".." || strings.Contains(s, `\`) || strings.ContainsRune(s, 0):
			return "", Error("Invalid path.")
		default:
			out = append(out, s)
		}
	}
	return strings.Join(out, "/"), nil
}

// name = satu segmen nama file/folder (regex ^[^/\\]+$ di source, ditambah . dan ..).
func name(n string) (string, error) {
	n = strings.TrimSpace(n)
	if n == "" || len(n) > 255 || n == "." || n == ".." || strings.ContainsAny(n, `/\`) || strings.ContainsRune(n, 0) {
		return "", Error("The name is invalid.")
	}
	return n, nil
}

func join(a, b string) string { return strings.Trim(a+"/"+b, "/") }

func dir(p string) string {
	d := path.Dir(p)
	if d == "." {
		return ""
	}
	return d
}

func (c Container) abs(rel string) string { return filepath.Join(root(c.Disk), c.Handle, filepath.FromSlash(rel)) }

func (s Store) Container(handle string) (Container, error) {
	var c Container
	var st sql.NullString
	err := s.DB.QueryRow(`SELECT handle,title,disk,settings FROM asset_containers WHERE handle=?`, handle).Scan(&c.Handle, &c.Title, &c.Disk, &st)
	if st.Valid {
		json.Unmarshal([]byte(st.String), &c.Settings)
	}
	return c, err
}

func (s Store) Containers() (out []Container, err error) {
	rows, err := s.DB.Query(`SELECT handle,title,disk FROM asset_containers ORDER BY title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var c Container
		rows.Scan(&c.Handle, &c.Title, &c.Disk)
		out = append(out, c)
	}
	return
}

const assetCols = `id,container,folder,basename,filename,extension,path,meta,updated_at`

type scanner interface{ Scan(...any) error }

func scanAsset(r scanner) (a Asset, err error) {
	var meta, upd sql.NullString
	if err = r.Scan(&a.ID, &a.Container, &a.Folder, &a.Basename, &a.Filename, &a.Extension, &a.Path, &meta, &upd); err != nil {
		return
	}
	if meta.Valid {
		json.Unmarshal([]byte(meta.String), &a.Meta)
	}
	a.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", upd.String)
	return
}

func (s Store) assets(where string, args ...any) (out []Asset, err error) {
	rows, err := s.DB.Query(`SELECT `+assetCols+` FROM assets_meta WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s Store) Asset(c Container, id string) (Asset, error) {
	return scanAsset(s.DB.QueryRow(`SELECT `+assetCols+` FROM assets_meta WHERE container=? AND id=?`, c.Handle, id))
}

type Folder struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func like(s string) string { return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s) }

// List = listFolder: sub-folder dari disk (urut nama), file dari assets_meta (urut basename).
func (s Store) List(c Container, p string) (folders []Folder, files []Asset, err error) {
	folders = []Folder{}
	if ents, e := os.ReadDir(c.abs(p)); e == nil {
		for _, e := range ents {
			if e.IsDir() {
				folders = append(folders, Folder{e.Name(), join(p, e.Name())})
			}
		}
	}
	files, err = s.assets(`container=? AND folder=? ORDER BY basename`, c.Handle, p)
	return
}

// Search = seluruh container, rekursif, berdasarkan nama.
func (s Store) Search(c Container, term string) (folders []Folder, files []Asset, err error) {
	folders = []Folder{}
	base := c.abs("")
	filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() && p != base && strings.Contains(strings.ToLower(d.Name()), strings.ToLower(term)) {
			rel, _ := filepath.Rel(base, p)
			folders = append(folders, Folder{d.Name(), filepath.ToSlash(rel)})
		}
		return nil
	})
	slices.SortStableFunc(folders, func(a, b Folder) int { return strings.Compare(a.Name, b.Name) })
	files, err = s.assets(`container=? AND basename LIKE ? ORDER BY basename`, c.Handle, "%"+like(term)+"%")
	return
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func (s Store) CreateFolder(c Container, p string) error {
	if p == "" || exists(c.abs(p)) {
		return Error(fmt.Sprintf("A folder or file already exists at \"%s\".", p))
	}
	return os.MkdirAll(c.abs(p), 0o755)
}

// within = assetsWithinFolder: file di folder itu sendiri dan semua turunannya.
func (s Store) within(c Container, p string) ([]Asset, error) {
	return s.assets(`container=? AND (folder=? OR folder LIKE ?)`, c.Handle, p, like(p)+"/%")
}

func replacePrefix(folder, from, to string) string {
	if folder == from {
		return to
	}
	return to + strings.TrimPrefix(folder, from)
}

func (s Store) MoveFolder(c Container, from, to string) error {
	if from == "" || to == from || strings.HasPrefix(to, from+"/") {
		return Error("A folder cannot be moved into itself.")
	}
	if exists(c.abs(to)) {
		return Error(fmt.Sprintf("A folder or file already exists at \"%s\".", to))
	}
	if err := os.MkdirAll(filepath.Dir(c.abs(to)), 0o755); err != nil {
		return err
	}
	if err := os.Rename(c.abs(from), c.abs(to)); err != nil {
		return err
	}
	as, err := s.within(c, from)
	for _, a := range as {
		nf := replacePrefix(a.Folder, from, to)
		if _, err = s.DB.Exec(`UPDATE assets_meta SET folder=?, path=?, updated_at=? WHERE id=?`, nf, join(nf, a.Basename), web.Now(), a.ID); err != nil {
			return err
		}
	}
	return err
}

func (s Store) DeleteFolder(c Container, p string) error {
	if p == "" {
		return Error("Invalid path.")
	}
	if err := os.RemoveAll(c.abs(p)); err != nil {
		return err
	}
	_, err := s.DB.Exec(`DELETE FROM assets_meta WHERE container=? AND (folder=? OR folder LIKE ?)`, c.Handle, p, like(p)+"/%")
	return err
}

func (s Store) uniqueBasename(c Container, folder, want string) (string, error) {
	ext := path.Ext(want)
	stem := strings.TrimSuffix(want, ext)
	b := want
	for i := 1; ; i++ {
		var n int
		if err := s.DB.QueryRow(`SELECT COUNT(*) FROM assets_meta WHERE container=? AND folder=? AND basename=?`, c.Handle, folder, b).Scan(&n); err != nil || n == 0 {
			return b, err
		}
		b = fmt.Sprintf("%s (%d)%s", stem, i, ext)
	}
}

func (s Store) uniqueFolder(c Container, parent, want string) string {
	n := want
	for i := 1; exists(c.abs(join(parent, n))); i++ {
		n = fmt.Sprintf("%s (%d)", want, i)
	}
	return join(parent, n)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func (s Store) insert(c Container, folder, basename, ext string, meta map[string]any) error {
	m, _ := json.Marshal(meta)
	now := web.Now()
	_, err := s.DB.Exec(`INSERT INTO assets_meta(container,folder,basename,filename,extension,path,meta,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`,
		c.Handle, folder, basename, strings.TrimSuffix(basename, path.Ext(basename)), ext, join(folder, basename), string(m), now, now)
	return err
}

// CopyFolder = copyFolderTo; dipakai juga oleh duplicate (toParent = parent folder asal).
func (s Store) CopyFolder(c Container, p, toParent string) error {
	if p == "" {
		return Error("Invalid path.")
	}
	np := s.uniqueFolder(c, toParent, path.Base(p))
	if np == p || strings.HasPrefix(np, p+"/") { // salin ke dalam dirinya sendiri: rekursi tak berujung
		return Error("A folder cannot be copied into itself.")
	}
	src := c.abs(p)
	if err := os.MkdirAll(c.abs(np), 0o755); err != nil {
		return err
	}
	var files []string
	filepath.WalkDir(src, func(fp string, d os.DirEntry, err error) error {
		if err != nil || fp == src {
			return nil
		}
		rel, _ := filepath.Rel(src, fp)
		if d.IsDir() {
			os.MkdirAll(c.abs(join(np, filepath.ToSlash(rel))), 0o755)
		} else {
			files = append(files, rel)
		}
		return nil
	})
	for _, rel := range files {
		if err := copyFile(filepath.Join(src, rel), filepath.Join(c.abs(np), rel)); err != nil {
			return err
		}
	}
	as, err := s.within(c, p)
	for _, a := range as {
		nf := replacePrefix(a.Folder, p, np)
		if err = s.insert(c, nf, a.Basename, a.Extension, a.Meta); err != nil {
			return err
		}
	}
	return err
}

func (s Store) DuplicateFolder(c Container, p string) error { return s.CopyFolder(c, p, dir(p)) }

func (s Store) CopyFile(c Container, a Asset, toFolder string) error {
	b, err := s.uniqueBasename(c, toFolder, a.Basename)
	if err != nil {
		return err
	}
	if err = copyFile(c.abs(a.Path), c.abs(join(toFolder, b))); err != nil {
		return err
	}
	return s.insert(c, toFolder, b, a.Extension, a.Meta)
}

func (s Store) taken(c Container, a Asset, folder, basename string) bool {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM assets_meta WHERE container=? AND folder=? AND basename=? AND id<>?`, c.Handle, folder, basename, a.ID).Scan(&n)
	return n > 0
}

func (s Store) RenameFile(c Container, a Asset, nb string) error {
	if nb == a.Basename {
		return nil
	}
	if s.taken(c, a, a.Folder, nb) {
		return Error(fmt.Sprintf("A file named \"%s\" already exists in this folder.", nb))
	}
	np := join(a.Folder, nb)
	if err := os.Rename(c.abs(a.Path), c.abs(np)); err != nil {
		return err
	}
	_, err := s.DB.Exec(`UPDATE assets_meta SET basename=?, filename=?, extension=?, path=?, updated_at=? WHERE id=?`,
		nb, strings.TrimSuffix(nb, path.Ext(nb)), strings.ToLower(strings.TrimPrefix(path.Ext(nb), ".")), np, web.Now(), a.ID)
	return err
}

func (s Store) MoveFile(c Container, a Asset, toFolder string) error {
	if s.taken(c, a, toFolder, a.Basename) {
		return Error(fmt.Sprintf("A file named \"%s\" already exists in the destination folder.", a.Basename))
	}
	np := join(toFolder, a.Basename)
	if err := os.MkdirAll(filepath.Dir(c.abs(np)), 0o755); err != nil {
		return err
	}
	if err := os.Rename(c.abs(a.Path), c.abs(np)); err != nil {
		return err
	}
	_, err := s.DB.Exec(`UPDATE assets_meta SET folder=?, path=?, updated_at=? WHERE id=?`, toFolder, np, web.Now(), a.ID)
	return err
}

func (s Store) DeleteFile(c Container, a Asset) error {
	if err := os.Remove(c.abs(a.Path)); err != nil && !os.IsNotExist(err) {
		return err
	}
	_, err := s.DB.Exec(`DELETE FROM assets_meta WHERE id=?`, a.ID)
	return err
}

// SaveMeta menulis meta.data.{title,alt,loading}; key meta lain (size, mime_type, ...) tetap.
func (s Store) SaveMeta(a Asset, title, alt, loading string) error {
	if a.Meta == nil {
		a.Meta = map[string]any{}
	}
	a.Meta["data"] = map[string]any{"title": title, "alt": alt, "loading": loading}
	m, _ := json.Marshal(a.Meta)
	_, err := s.DB.Exec(`UPDATE assets_meta SET meta=?, updated_at=? WHERE id=?`, string(m), web.Now(), a.ID)
	return err
}

// Upload = uploadFile (tanpa konversi WebP: lihat README, ditunda).
func (s Store) Upload(c Container, folder string, fh *multipart.FileHeader) (err error) {
	base := path.Base(strings.ReplaceAll(fh.Filename, `\`, "/"))
	if base, err = name(base); err != nil {
		return Error("The file name is invalid.")
	}
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(base), "."))
	if slices.Contains(blockedExt, ext) {
		return Error(fmt.Sprintf("The extension \".%s\" is not allowed for security reasons.", ext))
	}
	if al, _ := c.Settings["allowed_extensions"].([]any); len(al) > 0 && !slices.ContainsFunc(al, func(v any) bool { return strings.EqualFold(fmt.Sprint(v), ext) }) {
		return Error(fmt.Sprintf("The extension \".%s\" is not allowed in this container.", ext))
	}
	if kb, _ := c.Settings["max_size_kb"].(float64); kb > 0 && fh.Size > int64(kb)*1024 {
		return Error(fmt.Sprintf("The file exceeds the maximum allowed size of %d KB.", int64(kb)))
	}
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	head := make([]byte, 512)
	n, _ := src.Read(head)
	src.Seek(0, io.SeekStart)
	mt := strings.SplitN(http.DetectContentType(head[:n]), ";", 2)[0]
	if mt == "application/octet-stream" || mt == "text/plain" {
		if t := mime.TypeByExtension("." + ext); t != "" {
			mt = strings.SplitN(t, ";", 2)[0]
		}
	}
	b, err := s.uniqueBasename(c, folder, base)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(c.abs(folder), 0o755); err != nil {
		return err
	}
	dst, err := os.Create(c.abs(join(folder, b)))
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	if cerr := dst.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(c.abs(join(folder, b)))
		return err
	}
	meta := map[string]any{"size": fh.Size, "mime_type": mt}
	if strings.HasPrefix(mt, "image/") {
		src.Seek(0, io.SeekStart)
		if cfg, _, e := image.DecodeConfig(src); e == nil { // shortcut: png/jpeg/gif saja; webp tanpa dimensi, upgrade pakai golang.org/x/image/webp
			meta["width"], meta["height"] = cfg.Width, cfg.Height
		}
	}
	return s.insert(c, folder, b, ext, meta)
}

// Slug = Str::slug($title, '_').
func Slug(t string) string {
	var b strings.Builder
	us := false
	for _, r := range strings.ToLower(t) {
		if r < 128 && (r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			b.WriteRune(r)
			us = false
		} else if !us && b.Len() > 0 {
			b.WriteByte('_')
			us = true
		}
	}
	return strings.TrimRight(b.String(), "_")
}

// CreateContainer = AssetContainer::create + makeDirectory. kind: "public" | "private".
func (s Store) CreateContainer(title, kind string) (Container, error) {
	h := Slug(title)
	if !web.ValidHandle(h) {
		return Container{}, Error("Container title must contain letters or numbers.")
	}
	disk := "local"
	if kind == "public" {
		disk = "public"
	}
	now := web.Now()
	if _, err := s.DB.Exec(`INSERT INTO asset_containers(handle,title,disk,settings,created_at,updated_at) VALUES(?,?,?,NULL,?,?)`, h, title, disk, now, now); err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			return Container{}, Error(fmt.Sprintf("A container with handle \"%s\" already exists. Choose a different title.", h))
		}
		return Container{}, err
	}
	c := Container{Handle: h, Title: title, Disk: disk}
	return c, os.MkdirAll(c.abs(""), 0o755)
}

// RemoveContainer = remove:media: hapus folder, baris assets_meta, lalu container.
func (s Store) RemoveContainer(c Container) error {
	if err := os.RemoveAll(c.abs("")); err != nil {
		return err
	}
	if _, err := s.DB.Exec(`DELETE FROM assets_meta WHERE container=?`, c.Handle); err != nil {
		return err
	}
	_, err := s.DB.Exec(`DELETE FROM asset_containers WHERE handle=?`, c.Handle)
	return err
}
