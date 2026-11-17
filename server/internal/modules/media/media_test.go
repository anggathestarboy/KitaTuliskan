package media

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"draftcms-lite/internal/db"
)

func TestClean(t *testing.T) {
	for in, want := range map[string]string{"": "", "/a//b/": "a/b", "a/b": "a/b"} {
		if got, err := clean(in); err != nil || got != want {
			t.Fatalf("clean(%q) = %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"..", "a/../b", "./a", `a\b`} {
		if _, err := clean(bad); err == nil {
			t.Fatalf("clean(%q) harus ditolak", bad)
		}
	}
	if Slug("Blog Images!") != "blog_images" || Slug("  --  ") != "" {
		t.Fatal("slug")
	}
}

// Butuh MySQL lokal (sama seperti make dev); dilewati kalau tidak ada.
func TestStore(t *testing.T) {
	d, err := db.Open("127.0.0.1", "3306", "root", "", "draftcms_lite_test_media", true)
	if err != nil {
		t.Skip("MySQL tidak tersedia:", err)
	}
	t.Cleanup(func() { d.Exec("DROP DATABASE draftcms_lite_test_media") })
	wd, _ := os.Getwd()
	os.Chdir(t.TempDir())
	t.Cleanup(func() { os.Chdir(wd) })

	s := Store{d}
	c, err := s.CreateContainer("Blog Images", "private")
	if err != nil || c.Handle != "blog_images" || c.Disk != "local" {
		t.Fatal(c, err)
	}
	if _, err := s.CreateContainer("Blog Images", "public"); err == nil {
		t.Fatal("handle kembar harus ditolak")
	}

	upload := func(folder, fn, content string) error {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("file", fn)
		fw.Write([]byte(content))
		mw.Close()
		r := httptest.NewRequest("POST", "/", &buf)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		r.ParseMultipartForm(1 << 20)
		return s.Upload(c, folder, r.MultipartForm.File["file"][0], nil)
	}
	if err := upload("", "shell.php", "x"); err == nil {
		t.Fatal(".php harus diblokir")
	}
	if err := upload("", "a.txt", "one"); err != nil {
		t.Fatal(err)
	}
	upload("", "a.txt", "two") // nama bentrok → "a (1).txt"
	upload("docs/inner", "b.txt", "bee")
	_, files, _ := s.List(c, "")
	if len(files) != 2 || files[0].Basename+files[1].Basename != "a (1).txta.txt" { // urutan = collation MySQL
		t.Fatalf("unique basename: %+v", files)
	}

	// pindah folder menulis ulang folder+path asset di semua turunan
	if err := s.MoveFolder(c, "docs", "archive/docs"); err != nil {
		t.Fatal(err)
	}
	if _, fs, _ := s.List(c, "archive/docs/inner"); len(fs) != 1 || fs[0].Path != "archive/docs/inner/b.txt" {
		t.Fatalf("move: %+v", fs)
	}
	if _, err := os.Stat(c.abs("archive/docs/inner/b.txt")); err != nil {
		t.Fatal(err)
	}
	if s.MoveFolder(c, "archive", "archive/docs/x") == nil {
		t.Fatal("pindah ke dalam diri sendiri harus ditolak")
	}

	// duplicate folder → "docs (1)" dengan asset baru; DeleteFolder membersihkan DB + disk
	if err := s.DuplicateFolder(c, "archive/docs"); err != nil {
		t.Fatal(err)
	}
	if _, fs, _ := s.List(c, "archive/docs (1)/inner"); len(fs) != 1 {
		t.Fatalf("duplicate: %+v", fs)
	}
	if err := s.DeleteFolder(c, "archive"); err != nil {
		t.Fatal(err)
	}
	var n int
	d.QueryRow(`SELECT COUNT(*) FROM assets_meta WHERE container=? AND folder LIKE 'archive%'`, c.Handle).Scan(&n)
	if _, err := os.Stat(filepath.Join(PrivateRoot, c.Handle, "archive")); n != 0 || !os.IsNotExist(err) {
		t.Fatal("delete folder", n, err)
	}

	// rename file menjaga ekstensi dan menolak nama yang sudah dipakai
	a, _ := s.Asset(c, strconv.FormatInt(files[1].ID, 10))
	if err := s.RenameFile(c, a, "a (1).txt"); err == nil {
		t.Fatal("rename ke nama terpakai harus ditolak")
	}
	if err := s.RenameFile(c, a, "renamed.txt"); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveContainer(c); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Container(c.Handle); err == nil {
		t.Fatal("container harus terhapus")
	}
}
