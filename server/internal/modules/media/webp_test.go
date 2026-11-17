package media

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"math/rand"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"draftcms-lite/internal/db"

	"github.com/gen2brain/webp"
)

// noisePNG = gambar acak (sulit dikompres) supaya melebihi 800 KB.
func noisePNG(w, h int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewSource(1))
	for i := range img.Pix {
		img.Pix[i] = byte(r.Intn(256))
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

func TestSliderFormula(t *testing.T) {
	w := WebpConfig{MinKB: 200, MaxKB: 800, MinQ: 40, MaxQ: 90}
	for lvl, want := range map[int][2]int{0: {800, 90}, 50: {500, 65}, 100: {200, 40}, 999: {200, 40}, -5: {800, 90}} {
		if w.Target(lvl) != want[0] || w.Quality(lvl) != want[1] {
			t.Fatalf("level %d = %d KB / Q%d", lvl, w.Target(lvl), w.Quality(lvl))
		}
	}
}

func TestConvertWebp(t *testing.T) {
	cfg := WebpConfig{Enabled: true, MinKB: 200, MaxKB: 800, MinQ: 40, MaxQ: 90, MinDim: 320}
	src := noisePNG(1400, 1000) // ±5 MB sebagai PNG
	hi, err := convertWebp(src, cfg, cfg.Target(0), cfg.Quality(0))
	if err != nil || hi == nil || len(hi.Data) > 800*1024 {
		t.Fatalf("level 0: %v %v", err, hi)
	}
	lo, err := convertWebp(src, cfg, cfg.Target(100), cfg.Quality(100))
	if err != nil || len(lo.Data) > 200*1024 || len(lo.Data) > len(hi.Data) {
		t.Fatalf("level 100 harus ≤ 200 KB dan ≤ level 0: %d vs %d", len(lo.Data), len(hi.Data))
	}
	if _, err := webp.Decode(bytes.NewReader(lo.Data)); err != nil || lo.Width == 0 || lo.Height == 0 {
		t.Fatal("hasil bukan WebP valid", err)
	}
	if _, err := convertWebp([]byte("bukan gambar"), cfg, 800, 90); err == nil {
		t.Fatal("data rusak harus error")
	}

	var g bytes.Buffer // GIF 2 frame = animasi → tidak dikonversi
	pal := color.Palette{color.Black, color.White}
	gif.EncodeAll(&g, &gif.GIF{Image: []*image.Paletted{image.NewPaletted(image.Rect(0, 0, 4, 4), pal), image.NewPaletted(image.Rect(0, 0, 4, 4), pal)}, Delay: []int{1, 1}})
	if r, err := convertWebp(g.Bytes(), cfg, 800, 90); r != nil || err != nil {
		t.Fatalf("GIF animasi harus dilewati: %v %v", r, err)
	}
}

func TestOrient(t *testing.T) {
	if b := orient(image.NewNRGBA(image.Rect(0, 0, 4, 2)), 6).Bounds(); b.Dx() != 2 || b.Dy() != 4 {
		t.Fatal("orientasi 6 harus memutar 90°", b)
	}
	if jpegOrientation([]byte("bukan jpeg")) != 1 {
		t.Fatal("default 1")
	}
}

func TestUploadConvertsToWebp(t *testing.T) {
	d, err := db.Open("127.0.0.1", "3306", "root", "", "draftcms_lite_test_media", true)
	if err != nil {
		t.Skip("MySQL tidak tersedia:", err)
	}
	t.Cleanup(func() { d.Exec("DROP DATABASE draftcms_lite_test_media") })
	wd, _ := os.Getwd()
	os.Chdir(t.TempDir())
	t.Cleanup(func() { os.Chdir(wd) })

	s := Store{d}
	c, _ := s.CreateContainer("Photos", "public")
	up := func(c Container, name string, data []byte, level *int) error {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("file", name)
		fw.Write(data)
		mw.Close()
		r := httptest.NewRequest("POST", "/", &buf)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		r.ParseMultipartForm(1 << 20)
		return s.Upload(c, "", r.MultipartForm.File["file"][0], level)
	}
	big := noisePNG(1400, 1000)
	lvl := 100
	if err := up(c, "foto.png", big, &lvl); err != nil {
		t.Fatal(err)
	}
	up(c, "foto.png", big, nil) // nama bentrok → foto (1).webp
	_, files, _ := s.List(c, "")
	names := []string{}
	for _, f := range files {
		names = append(names, f.Basename)
		if f.Extension != "webp" || f.Meta["mime_type"] != "image/webp" || f.Meta["width"] == nil {
			t.Fatalf("meta: %+v", f)
		}
	}
	if len(files) != 2 || strings.Join(names, ",") != "foto (1).webp,foto.webp" {
		t.Fatal(names)
	}
	if _, err := os.Stat(c.abs("foto.webp")); err != nil {
		t.Fatal(err)
	}

	// konversi dimatikan per container → file apa adanya
	off := Container{Handle: c.Handle, Title: c.Title, Disk: c.Disk, Settings: map[string]any{"convert_to_webp": false}}
	if err := up(off, "asli.png", big, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(c.abs("asli.png")); err != nil {
		t.Fatal("convert_to_webp=false harus menyimpan PNG asli", err)
	}

	// isi WebP kecil tapi bernama .png → tidak di-encode ulang, ekstensi jadi .webp
	var wb bytes.Buffer
	webp.Encode(&wb, image.NewNRGBA(image.Rect(0, 0, 10, 10)))
	if err := up(c, "menyamar.png", wb.Bytes(), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(c.abs("menyamar.webp")); err != nil {
		t.Fatal("WebP bernama .png harus disimpan sebagai .webp", err)
	}
	// gambar kecil biasa juga dikonversi (bukan hanya >800 KB)
	var sm bytes.Buffer
	png.Encode(&sm, image.NewNRGBA(image.Rect(0, 0, 20, 20)))
	up(c, "kecil.png", sm.Bytes(), nil)
	if _, err := os.Stat(c.abs("kecil.webp")); err != nil {
		t.Fatal("PNG kecil harus jadi .webp", err)
	}
}
