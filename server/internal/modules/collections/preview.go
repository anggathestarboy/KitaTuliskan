package collections

// = EntryForm::refreshPreviewCache + PreviewController: state form yang belum disimpan ditaruh di memori per token (1 jam),
// dibaca Astro (/preview/{token}) untuk dirender. Route-nya di belakang login CMS; render template ada di Astro.
// shortcut: cache di memori proses (hilang saat Go restart, tidak dibagi antar instance), pindah ke tabel/Redis bila multi-instance.

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"draftcms-lite/internal/modules/navigation"
)

type previewEntry struct {
	payload map[string]any
	expires time.Time
}

var (
	previews   = map[string]*previewEntry{}
	previewsMu sync.Mutex
)

// putPreview = Cache::put: version naik tiap simpan (polling Astro membandingkannya); sekalian buang yang kedaluwarsa.
func putPreview(token string, p map[string]any) int {
	previewsMu.Lock()
	defer previewsMu.Unlock()
	now := time.Now()
	for k, v := range previews {
		if now.After(v.expires) {
			delete(previews, k)
		}
	}
	v := 1
	if old := previews[token]; old != nil {
		v = old.payload["version"].(int) + 1
	}
	p["version"] = v
	previews[token] = &previewEntry{p, now.Add(time.Hour)}
	return v
}

func getPreview(token string) map[string]any {
	previewsMu.Lock()
	defer previewsMu.Unlock()
	if v := previews[token]; v != nil && time.Now().Before(v.expires) {
		return v.payload
	}
	return nil
}

// previewPayload = isi cache refreshPreviewCache(): atribut entry (data memuat title) + collection/blueprint/site.
func previewPayload(collection, blueprint, site, id, origin, status, slug, date string, order any, title string, data map[string]any) map[string]any {
	d := map[string]any{}
	for k, v := range data {
		d[k] = v
	}
	d["title"] = title
	var dt any
	if date != "" {
		dt = strings.Replace(date, "T", " ", 1)
	}
	if order == "" {
		order = nil
	}
	return map[string]any{"collection": collection, "blueprint": blueprint, "site": site, "attributes": map[string]any{
		"id": id, "site": site, "origin_id": origin, "status": status, "slug": slug, "date": dt, "order": order,
		"collection": collection, "blueprint": blueprint, "data": d}}
}

// previewURL = route 'collections.preview': lewat proxy Go ke Astro, jadi cookie login CMS ikut terkirim.
func previewURL(token string) string { return strings.TrimRight(os.Getenv("APP_URL"), "/") + "/preview/" + token }

func newPreviewToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (m *mod) registerPreview(mux *http.ServeMux) {
	// Dibaca Astro (cookie sesi diteruskan). 404 bila token hilang atau collection mematikan live preview (seperti PreviewController).
	m.a.Admin(mux, "GET /collections/api/preview/{token}", func(w http.ResponseWriter, r *http.Request) {
		p := getPreview(r.PathValue("token"))
		if c, err := m.col(sv(p["collection"])); p == nil || err != nil || !c.LivePreview {
			http.NotFound(w, r)
			return
		}
		navigation.Reply(w, p, nil)
	})

	// Dipanggil form tiap field berubah (= updated() → refreshPreviewCache).
	m.a.Admin(mux, "POST /collections/api/preview/{handle}/{token}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID, Site, Origin, Blueprint, Title, Slug, Status, Date string
			Order                                                  any
			Data                                                   map[string]any
		}
		if err := navigation.Decode(w, r, &in); err != nil {
			navigation.Reply(w, nil, err)
			return
		}
		c, err := m.col(r.PathValue("handle"))
		if err != nil || !c.LivePreview {
			http.NotFound(w, r)
			return
		}
		v := putPreview(r.PathValue("token"), previewPayload(c.Handle, in.Blueprint, in.Site, in.ID, in.Origin, in.Status, in.Slug, in.Date, in.Order, in.Title, in.Data))
		navigation.Reply(w, map[string]any{"version": v}, nil)
	})
}
