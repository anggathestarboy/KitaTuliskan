package web

import (
	"encoding/json"
	"html/template"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// vite = @vite('resources/ts/app.ts') di Laravel.
// Dev (APP_DEBUG=true dan Vite jalan di :5173): tag ke dev server Vite, HMR aktif.
// Prod: baca static/build/.vite/manifest.json, tag ke file ber-hash.
const viteDev = "http://localhost:5173/static/build" // base di vite.config.ts berlaku juga di dev server

var (
	manifestOnce sync.Once
	manifest     map[string]struct {
		File string   `json:"file"`
		CSS  []string `json:"css"`
	}
)

func viteTags(entries ...string) template.HTML {
	var out string
	if os.Getenv("APP_DEBUG") == "true" {
		if c, err := net.DialTimeout("tcp", "localhost:5173", 100*time.Millisecond); err == nil {
			c.Close()
			out = `<script type="module" src="` + viteDev + `/@vite/client"></script>`
			for _, e := range entries {
				if strings.HasSuffix(e, ".css") {
					out += `<link rel="stylesheet" href="` + viteDev + `/` + e + `">`
				} else {
					out += `<script type="module" src="` + viteDev + `/` + e + `"></script>`
				}
			}
			return template.HTML(out)
		}
		manifestOnce = sync.Once{} // tanpa dev server: baca manifest tiap kali supaya hasil `vite build` baru terlihat
	}
	manifestOnce.Do(func() {
		b, _ := os.ReadFile("static/build/.vite/manifest.json")
		json.Unmarshal(b, &manifest)
	})
	for _, e := range entries {
		m, ok := manifest[e]
		if !ok {
			out += `<!-- vite: entry ` + e + ` tidak ada di manifest; jalankan npm run build -->`
			continue
		}
		for _, css := range m.CSS {
			out += `<link rel="stylesheet" href="/static/build/` + css + `">`
		}
		if strings.HasSuffix(m.File, ".css") {
			out += `<link rel="stylesheet" href="/static/build/` + m.File + `">`
		} else {
			out += `<script type="module" src="/static/build/` + m.File + `"></script>`
		}
	}
	return template.HTML(out)
}
