# Draft CMS Lite

Port dari `source/` (Laravel 13 + Livewire + Flux, 18 modul di `app/Modules`) ke **Go + Astro**.
Struktur, nama modul, tabel, route, dan permission mengikuti source 1:1. Baca `source/docs/AI-CONTEXT.md` untuk aturan modul aslinya.

```
server/                         Go stdlib + mysql driver + bcrypt. Admin panel = html/template.
  main.go                       = bootstrap/providers.php + routes/web.php (daftar modul, /api, catch-all)
  setup.go                      = php artisan draft:setup / migrate:fresh
  internal/db/schema.sql        = database/migrations (34 tabel, MySQL)
  internal/web/                 = layout, session, flash, Auth/Admin/Guest middleware, permission, Resource CRUD generik
  internal/mail/                = SMTP (MAIL_*), kosong → tulis ke log
  internal/modules/<modul>/     = app/Modules/<Modul>  (satu package per modul, lihat tabel di bawah)
  templates/<modul>/            = app/Modules/<Modul>/resources/views
site/                           Astro. src/cms.ts = pengganti @collection/@taxonomy/@global/@sites (fetch /api/* saat build)
```

## Onboarding dev baru

Prasyarat sekali saja di mesin: **Go 1.26+**, **Node 20+**, **GNU make**, **MySQL** di `127.0.0.1:3306` user `root` tanpa password (DBngin paling gampang). Opsional: `brew install fswatch` supaya server Go restart sendiri saat file `.go` berubah.

```sh
git clone <repo-url> draftcms-lite && cd draftcms-lite/app-lite
make install EMAIL=kamu@example.com PASSWORD=rahasia1234   # npm i (server + site), copy .env, buat database + schema, super admin, site, pages, homepage, sidebar
make dev                                                   # Go :8000 + Vite :5173 + Astro :4321
```

Buka `http://localhost:8000/admin-panel`, login dengan email/password di atas. Selesai.

Hari-hari berikutnya cukup `make dev` dan `make stop`. Kalau `schema.sql` berubah (dikabari di chat), jalankan `make fresh` sekali: database di-drop dan dibuat ulang.
Pindah modul = buka `internal/modules/<modul>/` dan `templates/<modul>/`, tabel pembagian ada di bawah. Sebelum push: `make test`.

## Jalankan (MySQL 127.0.0.1:3306 root tanpa password, DBngin)

```sh
make install EMAIL=admin@example.com PASSWORD=secret1234   # sekali: schema + super admin + site + pages + homepage + sidebar
make dev                                                   # Go :8000 + Vite :5173 (TS + Tailwind, HMR) + Astro :4321
make stop                                                  # matikan semua yang dijalankan make dev
make fresh                                                 # drop database, install ulang (dev only)
make test                                                  # go vet + go test
```

Admin panel: `http://localhost:8000/admin-panel` (prefix dari `CMS_ROUTE`, seperti `config/cms.php`).
Registrasi publik **tidak ada**, sama seperti source. User dibuat lewat `make install` atau modul Users.

## Modul = pembagian kerja

Satu modul = satu package `internal/modules/<modul>/` + satu folder `templates/<modul>/`. Dev hanya menyentuh modul miliknya.
Yang dipakai bersama hanya `internal/web/` dan `schema.sql`; perubahan di dua itu wajib dikabari.

Status: **done** = fungsi inti ada · **list** = baru index/create/delete generik via `web.Resource` · **stub** = halaman kosong.

| Modul (source) | Package lite | Route admin | Tabel | Status | Yang masih harus dibuat (lihat source) |
|---|---|---|---|---|---|
| Auth | `auth` | `/login` `/forgot-password` `/reset-password` `/verify-email` `/user/confirm-password`, `/settings/*` | users, sessions, password_reset_tokens | done | 2FA (TOTP + recovery codes), passkeys, ban middleware sudah ada |
| Users | `users` | `/users` `/roles` `/user-groups` | users, roles, user_groups, 3 pivot | list | UserForm (edit, assign roles/groups), RoleForm dengan checkbox PermissionRegistry per collection/site, ban/unban UI |
| Blueprints | `blueprints` | `/blueprints` `/fieldsets` | blueprints, fieldsets | list | **Field builder** (HasFieldBuilder): tipe text textarea markdown bard toggle date integer number select radio array assets taxonomy author link replicator section; kondisi field (EvaluatesFieldConditions); render + parse form dari JSON → `web.RenderFields/ParseFields` dipakai Collections, Taxonomies, Globals, Forms, Navigation |
| Collections | `collections` | `/collections` `/collections/{handle}` | collections, entries | list | CollectionManager settings (route, dated, orderable, blueprint), EntryForm (form dari blueprint, slug/uri resolver, status draft/published/scheduled, seo, localization origin_id), PreviewController, publish-scheduled job |
| Taxonomies | `taxonomies` | `/taxonomies` `/taxonomies/{handle}` | taxonomies, taxonomy_terms | list | TermForm dari blueprint, uri, localization |
| Globals | `globals` | `/globals` `/globals/{handle}` | global_sets, global_set_variables | list | GlobalSetForm dari blueprint per locale |
| Media | `media` | `/media` `/media/api/*` `/media/download` `/media/file/{container}/{path}` | asset_containers, assets_meta | done (WebP otomatis + slider kompresi: gen2brain/webp, Go murni; AVIF disimpan apa adanya) | AssetPicker untuk field `assets` (menunggu field builder Blueprints). UI = React island (`resources/ts/media`); buat container: `make media "Nama" [TYPE=public|private]`, hapus: `make media-remove handle`. File: `storage/public` (/storage/) dan `storage/private` (hanya lewat route ber-auth) |
| Forms | `forms` | `/forms` `/forms/{handle}` ; publik `POST /!/forms/{handle}` | forms, form_submissions | done (dasar) | validasi dari blueprint form, submission detail + export txt/json, reCAPTCHA, honeypot |
| Navigation | `navigation` | `/navigations` `/navigations/{handle}` `/navigations/api/*` | navigations, trees | done (sidebar-admin) | Mode blueprint (form dari field replicator + localization per site) menunggu modul Blueprints. UI = React island (`resources/ts/navigation`). `sidebar-admin` terproteksi: handle/judul tidak bisa diubah atau dihapus, hanya tree-nya. Seed/reset: `make sidebar [FORCE=1]` |
| Structure | `structure` | — | trees | done | helper Load/Save dipakai Navigation & Collections. Jangan duplikasi logic tree |
| Sites | `sites` | `/sites` | sites | list | edit, order, resolve site dari host/path untuk entries |
| Sitemap | `sitemap` | `/sitemap` ; publik `/sitemap.xml` `/sitemap-{collection}.xml` | sitemaps | done (dasar) | SitemapSyncer (isi tabel dari entries published), xsl stylesheet, toggle per baris |
| Redirects | `redirects` | `/redirect-manager` ; middleware publik | redirects | done | edit, start/end window UI, hits sudah dihitung |
| SiteSettings | `sitesettings` | `/site-settings` ; publik `/robots.txt` `/llms.txt` | site_settings | done (dasar) | schema markup blocks, integrations, LlmsTxtGenerator |
| ActivityLog | `activitylog` | `/activity-log` `/activity-log/export` `/activity-log/restore` `/activity-log/delete-request` `/activity-log/confirm-delete/{token}` | activity_logs, activity_log_deletion_requests | done (list + filter + pagination, export/restore JSON, hapus per tahun via email konfirmasi, event login/logout/gagal login) | Opt-in, default mati: `make activity_log` (= `php artisan draft:activity_log`) set `ACTIVITY_LOG_ENABLED=true` di `server/.env`, lalu restart server (`make dev` restart sendiri). Modul lain mencatat lewat `activitylog.Record()` / `RecordProps()` (no-op saat mati). Hapus per tahun khusus superadmin: request → email link (24 jam) → konfirmasi → dieksekusi tiap 00:00, atau manual `go run . activity_log:process`. Permission: `view activity log`, `view activity log detail`, `export activity log`, `import activity log`. Pencatatan sinkron (tanpa queue); link email memakai `APP_URL` |
| NotFoundDetector | `notfounddetector` | `/404-detector` | — | stub | NotFoundScanner |
| PackageUpdates | `packageupdates` | `/package-updates` | — | stub (tampilkan go.mod) | PackageUpdateChecker |
| About | `about` | `/about` | — | done | — |

### Urutan yang disarankan

1. **Blueprints** dulu, satu orang, sampai `web.RenderFields/ParseFields` jadi. Collections, Taxonomies, Globals, Forms, Navigation semua menunggu ini.
2. Setelah itu paralel: **Dev A** Collections → Taxonomies → Sites → `site/` Astro. **Dev B** Media (AssetPicker) → Navigation → Globals → Forms → Users.
3. Sisanya (Sitemap syncer, 404 Detector, Package Updates) kapan saja, tidak ada yang menunggu.

### Pola yang dipakai

- Route modul lewat `a.Admin(mux, "GET /path", h)` → otomatis `/admin-panel/path` + auth + verified + `access cp`. Publik langsung `mux.HandleFunc`.
- Permission: `web.CurrentUser(r).Can("edit roles")`. `super` lolos semua. Nama permission sama persis dengan PermissionRegistry source.
- List + create + delete satu tabel: `a.Resource(mux, web.Resource{...})`, template `resource/index.html`. Halaman edit dan form khusus ditulis di modul.
- Form HTML biasa, redirect setelah POST, pesan lewat `web.Flash`. JS hanya kalau perlu (tree editor, field builder).
- SQL langsung di handler. Tanpa ORM, repository, service layer.
- Tabel relasi ikut modul induknya (entries di collections, terms di taxonomies, submissions di forms, variables di globals).
- Definisi selesai per modul: route di tabel di atas ada semua, validasi di server, `/api/<tabel>` cukup untuk Astro, satu `*_test.go` kecil, `make test` hijau.

### UI admin = Tailwind + Alpine (fondasi yang sama dengan Flux UI di source)

- Komponen ada di `templates/layouts/ui.html`: `button input select checkbox switch badge status page-header card-open/close th td-class tr-class menu-* ellipsis-trigger modal-open/close modal-x toast callout heading subheading icon-*`.
- Pakai: `{{template "input" (dict "Label" "Title" "Name" "title" "Required" true)}}`, `{{template "button" (dict "Label" "Save" "Variant" "primary" "Type" "submit")}}`.
- Modal = `<dialog>` native, dibuka dengan `showModal()`. Dropdown = `x-data="{open:false}"` + `menu-open/close`. Toast otomatis dari `web.Flash`.
- Frontend admin lewat **Vite + vanilla TypeScript** (`server/vite.config.ts`): entry `resources/ts/app.ts` (Alpine dari npm + `resources/css/app.css` Tailwind v4). Template memanggil `{{vite "resources/ts/app.ts"}}` = `@vite()` Laravel: dev server :5173 dengan HMR saat `APP_DEBUG=true`, manifest `static/build/` di production.
- Navigasi admin memakai **Turbo Drive** (= `wire:navigate` di source): link dan form di-fetch lalu body di-swap, prefetch saat hover, progress bar. Konsekuensi: respons form yang **tidak redirect** (render ulang dengan error) wajib berstatus 422, pakai `w.WriteHeader(http.StatusUnprocessableEntity)` sebelum `a.Render`. Sukses selalu redirect. Elemen yang tidak boleh lewat Turbo beri `data-turbo="false"`.
- Tipe JSON bersama (Field, Blueprint, TreeItem) ada di `resources/ts/types.ts`. Modul berat (tiptap, sortable, dsb) `npm i` lalu import dari `app.ts` atau file per halaman di `resources/ts/`.
- `resource/index.html` adalah contoh lengkap: header bar, tabel dalam card, dropdown per baris, modal create, modal confirm delete, Ctrl/Cmd+S submit. Halaman modul baru ikuti pola itu.
- Jangan tambah library UI lain (daisyUI, Preline, React). Kalau butuh komponen baru, tambah define di `ui.html` dan kabari.

### Dipotong dari source (sengaja, bukan lupa)

Registration publik (source juga tidak ada), 2FA/passkeys (fase 2), queue/jobs (kirim mail sinkron), Docker, reCAPTCHA, PostgreSQL.
