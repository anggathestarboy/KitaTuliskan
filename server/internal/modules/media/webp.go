package media

import (
	"bytes"
	"image"
	"image/gif"
	"os"
	"strconv"
	"strings"

	_ "golang.org/x/image/bmp" // decoder BMP
	xdraw "golang.org/x/image/draw"

	"github.com/gen2brain/webp" // libwebp → Go murni (tanpa cgo); mendaftarkan decoder "webp" juga
	_ "image/jpeg"
	_ "image/png"
)

// WebP = Services/WebpImageConverter + config/media.php di source. Env:
// MEDIA_CONVERT_TO_WEBP, MEDIA_WEBP_{MIN,MAX}_SIZE_KB, MEDIA_WEBP_{MIN,MAX}_QUALITY, MEDIA_WEBP_MIN_DIMENSION.
const hardMaxKB = 800 // batas mutlak hasil konversi, berapa pun env/slider-nya

type WebpConfig struct {
	Enabled                          bool
	MinKB, MaxKB, MinQ, MaxQ, MinDim int
}

func envInt(k string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return v
	}
	return def
}

func clamp(v, lo, hi int) int { return max(lo, min(hi, v)) }

// webpConfig menggabungkan env dengan override per container (settings.convert_to_webp).
func (c Container) webpConfig() WebpConfig {
	on := os.Getenv("MEDIA_CONVERT_TO_WEBP") != "false"
	if v, ok := c.Settings["convert_to_webp"].(bool); ok {
		on = v
	}
	maxKB := min(max(1, envInt("MEDIA_WEBP_MAX_SIZE_KB", hardMaxKB)), hardMaxKB)
	maxQ := clamp(envInt("MEDIA_WEBP_MAX_QUALITY", 90), 1, 100)
	return WebpConfig{
		Enabled: on, MaxKB: maxKB, MinKB: min(max(1, envInt("MEDIA_WEBP_MIN_SIZE_KB", 200)), maxKB),
		MaxQ: maxQ, MinQ: min(maxQ, max(1, envInt("MEDIA_WEBP_MIN_QUALITY", 40))), MinDim: max(1, envInt("MEDIA_WEBP_MIN_DIMENSION", 320)),
	}
}

// Slider: level 0 = kualitas terbaik/file terbesar, 100 = file terkecil (compressionTargetKb/compressionQuality di source).
func (w WebpConfig) Target(level int) int {
	return int(float64(w.MaxKB) - float64(w.MaxKB-w.MinKB)*float64(clamp(level, 0, 100))/100 + .5)
}
func (w WebpConfig) Quality(level int) int {
	return int(float64(w.MaxQ) - float64(w.MaxQ-w.MinQ)*float64(clamp(level, 0, 100))/100 + .5)
}

// supportsWebp = WebpImageConverter::supports. AVIF tidak ada decoder Go murni, jadi disimpan apa adanya.
func supportsWebp(mime, ext string) bool {
	switch strings.ToLower(mime) {
	case "image/jpeg", "image/pjpeg", "image/png", "image/gif", "image/webp", "image/bmp", "image/x-ms-bmp":
		return true
	case "", "application/octet-stream":
		switch strings.ToLower(ext) {
		case "jpg", "jpeg", "png", "gif", "webp", "bmp":
			return true
		}
	}
	return false
}

type webpResult struct {
	Data          []byte
	Width, Height int
	Quality       int
}

// animated: GIF >1 frame atau WebP dengan flag animasi. Encoder hanya menyimpan frame pertama, jadi file dibiarkan apa adanya.
func animated(data []byte) bool {
	if len(data) > 21 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" && string(data[12:16]) == "VP8X" {
		return data[20]&0x02 != 0
	}
	if bytes.HasPrefix(data, []byte("GIF8")) {
		g, err := gif.DecodeAll(bytes.NewReader(data))
		return err == nil && len(g.Image) > 1
	}
	return false
}

func encodeQ(img image.Image, q int) ([]byte, error) {
	var b bytes.Buffer
	err := webp.Encode(&b, img, webp.Options{Quality: q})
	return b.Bytes(), err
}

// withinLimit: kualitas tertinggi yang masih muat di budget (binary search antara MinQ dan MaxQ); nil bila MinQ pun kebesaran.
func withinLimit(img image.Image, limit, maxQ, minQ int) (*webpResult, error) {
	res := func(d []byte, q int) *webpResult { b := img.Bounds(); return &webpResult{d, b.Dx(), b.Dy(), q} }
	top, err := encodeQ(img, maxQ)
	if err != nil {
		return nil, err
	}
	if len(top) <= limit {
		return res(top, maxQ), nil
	}
	if maxQ == minQ {
		return nil, nil
	}
	best, err := encodeQ(img, minQ)
	if err != nil || len(best) > limit {
		return nil, err
	}
	bq := minQ
	for lo, hi := minQ+1, maxQ-1; lo <= hi; {
		q := (lo + hi) / 2
		d, err := encodeQ(img, q)
		if err != nil {
			return nil, err
		}
		if len(d) <= limit {
			best, bq, lo = d, q, q+1
		} else {
			hi = q - 1
		}
	}
	return res(best, bq), nil
}

// convertWebp = WebpImageConverter::convert. Turunkan kualitas dulu; bila belum muat, kecilkan dimensi 0.8× (maks 12 kali,
// tidak di bawah MinDim) dan encode ulang. Mengembalikan nil untuk gambar beranimasi.
func convertWebp(data []byte, cfg WebpConfig, targetKB, quality int) (*webpResult, error) {
	if animated(data) {
		return nil, nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, Error("The image could not be read for WebP conversion.")
	}
	img = orient(img, jpegOrientation(data)) // foto HP: tanpa ini hasil WebP miring karena EXIF dibuang
	limit := clamp(targetKB, 1, min(cfg.MaxKB, hardMaxKB)) * 1024
	maxQ := clamp(quality, 0, 100)
	minQ := min(cfg.MinQ, maxQ)

	r, err := withinLimit(img, limit, maxQ, minQ)
	for i := 0; err == nil && r == nil && i < 12; i++ {
		b := img.Bounds()
		long := max(b.Dx(), b.Dy())
		t := max(cfg.MinDim, int(float64(long)*0.8+.5))
		if long <= cfg.MinDim || t >= long {
			break
		}
		nw, nh := max(1, b.Dx()*t/long), max(1, b.Dy()*t/long)
		dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Over, nil)
		img = dst
		r, err = withinLimit(img, limit, maxQ, minQ)
	}
	if err != nil {
		return nil, Error("The image could not be converted to WebP.")
	}
	if r == nil { // lantai kualitas pun tidak cukup (gambar sangat detail): pakai hasil terkecil yang bisa dibuat
		d, err := encodeQ(img, minQ)
		if err != nil {
			return nil, Error("The image could not be converted to WebP.")
		}
		b := img.Bounds()
		r = &webpResult{d, b.Dx(), b.Dy(), minQ}
	}
	return r, nil
}

// jpegOrientation membaca tag EXIF Orientation (1..8) dari JPEG; 1 bila tidak ada.
func jpegOrientation(b []byte) int {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return 1
	}
	for i := 2; i+4 <= len(b) && b[i] == 0xFF; {
		m, l := b[i+1], int(b[i+2])<<8|int(b[i+3])
		if m == 0xE1 && i+10 <= len(b) && string(b[i+4:i+10]) == "Exif\x00\x00" {
			t := b[i+10 : min(len(b), i+2+l)]
			if len(t) < 8 {
				return 1
			}
			u16 := func(p int) int {
				if p+2 > len(t) {
					return 0
				}
				if t[0] == 'I' {
					return int(t[p]) | int(t[p+1])<<8
				}
				return int(t[p])<<8 | int(t[p+1])
			}
			u32 := func(p int) int { return u16(p) | u16(p+2)<<16 }
			if t[0] != 'I' {
				u32 = func(p int) int { return u16(p)<<16 | u16(p+2) }
			}
			ifd := u32(4)
			for e, n := 0, u16(ifd); e < n; e++ {
				if p := ifd + 2 + e*12; u16(p) == 0x0112 {
					if o := u16(p + 8); o >= 1 && o <= 8 {
						return o
					}
				}
			}
			return 1
		}
		if m == 0xDA {
			break
		}
		i += 2 + l
	}
	return 1
}

// orient menerapkan orientasi EXIF 2..8 (1 = tidak diubah).
func orient(src image.Image, o int) image.Image {
	if o <= 1 || o > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	nw, nh := w, h
	if o >= 5 {
		nw, nh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
