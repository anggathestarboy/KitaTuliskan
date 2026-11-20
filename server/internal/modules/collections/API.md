# Collections JSON API (final)

Base: `{cp}/collections/api` (cp = `/admin-panel`). Semua endpoint `a.Admin` (login + "access cp"). Error: `{error}` (403 "Forbidden", 404 "Not found.", 422 pesan) atau 422 `{error, errors:{key:msg}}`.

## Halaman (Render)
`GET /collections` → `collections/index.html` · `GET /collections/{handle}` → `collections/entries.html` (izin `view {h} entries`) ·
`GET /collections/{handle}/entries/create/{site}` → `collections/entry.html` (izin `create`) · `GET /collections/{handle}/entries/{id}` → `collections/entry.html` (izin `edit`).
Data template: `.Handle`, `.CollTitle`, `.ID` ("" saat create), `.Site` ("" saat edit).

## Collection manager
`GET /list` → `{collections:[C], canConfigure, sites:[{handle,name}]}`
`C = {id, handle, title, blueprint:"" | handle, sites:[handles] (kosong di DB = semua site; di sini selalu diisi semua site), routeMode:"single"|"per_site", route:"", routes:{site:route}, structured, orderable, visibleOrder, livePreview, schemaEnabled, seoEnabled, maxDepth:"" | "3", expectsRoot, saveAction:"listing"|"editing"|"create_another"}`
`POST /save` body = C tanpa blueprint (`id` 0 = baru; `maxDepth` string/angka/""). → `{ok:true}`. Aturan: semua site tercentang = tanpa pembatasan; per_site hanya bila >1 site; structured off → maxDepth/expectsRoot dikosongkan; handle `^[a-z][a-z0-9_]*$`, unik, tak bisa diganti setelah dibuat.
`POST /delete {id}` → `{ok:true}` atau 422 `Cannot delete "T" — it still has N entries. Delete those first.`

## Entry manager
`GET /entry-list/{handle}?search=&site=` (param opsional; tanpa param = semua entry) →
```
{collection:{handle,title,structured,orderable}, entries:[E], sites:[{handle,name}], createSite:"en"|"", blueprints:[handle], defaultBlueprint:"",
 canCreate, statuses:{published:"Published",...}, statusOrder:["published","scheduled","need_review","draft"], bulkAvailable (= !structured && !orderable),
 perms:{edit,publish,delete,reorder}}
E = {id, site, title, slug, blueprint, status, order:int|null, updatedAt:"…T…Z", canEdit, canDelete, canDeleteAll (entry + semua terjemahan boleh dihapus), canPublish, siblingCount, children?:[E]}
```
Flat: urut `order != 1` dulu lalu updated_at desc (SQL sama dengan versi lama), tanpa `children`. Structured: pohon root saja di `entries`, tiap node punya `children` (selalu array), urut order (null terakhir) lalu updated_at desc; entry yang induknya tak ikut daftar (terfilter) tidak muncul.
`POST /entry-delete/{handle} {id, deleteLocales?:bool (default true)}` → `{ok}`
`POST /entry-duplicate/{handle} {id}` → `{ok}` (salinan draft semua bahasa, slug `-copy`, `(Copy)` di judul, posisi tree sama)
`POST /entry-bulk/{handle} {action:"delete"|"status", ids:[], status?}` → `{ok, count}` (diabaikan bila structured/orderable; published/scheduled butuh `publish`, + `publish other authors` untuk entry orang lain)
`POST /entry-reorder/{handle} {id, position (indeks akhir di antara saudara), parentId:null|id, site}` → `{ok}`; izin `reorder {h} entries`; no-op bila tidak orderable / site kosong. Root mulai order 1, anak mulai 2.

## Entry form
`GET /entry/{handle}?id=UUID` atau `?site=S&origin=ID&blueprint=H` →
```
{item:{handle,title}, blueprint:{handle,tabs,fields}  (fieldset sudah diperluas), sites:[{handle,name,href}] ([] bila ≤1 site; href "" untuk site aktif),
 activeSite, isOrigin, data (sudah di-overlay dari origin / diprefill untuk terjemahan baru), canEdit:true,
 collection:{handle,title,structured,visibleOrder,livePreview,schemaEnabled,seoEnabled,saveAction},
 entry:{id:"", title, slug, status:"draft", date:"YYYY-MM-DDTHH:mm"|"", order:"" | "3", parentId:null|id, origin:null|id, blueprint, url:"" (URL publik; Visit)},
 statuses, statusOrder, canEditStatus, parents:[{id,title}] (structured saja; site yang sama, tanpa entry ini),
 seo:{meta_title,meta_description,meta_canonical,robots:[],meta_featured_image:[assetId],meta_author,sitemap_priority,sitemap_changefreq} ({} bila seo off; meta_featured_image & meta_author non-localizable diambil dari origin),
 schemaBlocks:[Block] (sitesettings.Block, semua key ada, same_as selalu array), schemaTypes:[...],
 seoOptions:{robots:{index:"Index",...}, robotsOrder:[...], priorities:[...], changefreqs:[...]},
 + Lookups: entries, terms, authors, bardContainer}
```
`POST /entry/{handle}` body `{id ("" = baru), site, origin, blueprint, title, slug, status, date:"YYYY-MM-DDTHH:mm", order:""|int|"3", parentId:""|id, data, seo, schemaBlocks}` → `{ok, id, site, blueprint, saveAction}` + cookie flash "Entry saved.".
Kunci `errors`: `title`, `slug`, `status`, `date`, `order`, `blueprint`, `parentId`, path field blueprint tanpa prefix `data.` (mis. `links.0.title`), `seo.meta_title|meta_description|meta_canonical|robots|meta_featured_image|meta_author|sitemap_priority|sitemap_changefreq`, `schemaBlocks.0.type|name|email|…`.
Aturan: published/scheduled + tanggal depan → tersimpan `scheduled` (dan sebaliknya `published`); ubah status butuh `publish {h} entries` (403 jika tidak); root pertama sebuah site pada collection structured+orderable otomatis `order=1` (homepage, uri kosong); `uri` dihitung dari route collection; sitemap disinkronkan (hanya published, bukan noindex, punya route).

## Scheduler
`collections.PublishScheduled(a)` membalik `scheduled` yang tanggalnya lewat jadi `published` (goroutine tiap menit dari `Register`).
