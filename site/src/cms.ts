// Pengganti Blade directive proyek lama (TemplatingServiceProvider); dipanggil di frontmatter .astro saat build:
//   @collection('blog', [...])  → const posts = await collection('blog', { limit: 3 })      lalu posts.map(p => p.data.title)
//   @taxonomy('categories')     → const terms = await taxonomy('categories')
//   @sites()                    → const all = await sites()
//   @global('site_settings')    → const global = await globalSet('site_settings')          lalu global.site_name
//   @navigation('header')       → const navigation = await navigation('header', { site })  lalu navigation.navbar_loop
// Opsi collection/taxonomy sama dengan source: site, where, sort ("field:desc" | array), random, limit, related (hanya collection).
// shortcut: filter di memori dari /api/{table} (cukup untuk build statis skala kecil), pindah ke query SQL bila data besar.
// Field "assets" di-resolve jadi objek (= AssetMeta): data.image.url / .alt / .width / .height / .loading / .title / .mime_type ...
// max_items 1 → satu objek (data.image.url), selain itu array (data.gallery.map(i => i.url)). Hanya container disk public.
// shortcut: link "entry::{id}" di data belum di-resolve (source: resolveLinkFields).
const base = import.meta.env.CMS_URL ?? 'http://localhost:8000';
type Row = Record<string, any>;
// SSR: data harus live. Cache hanya 2 detik supaya beberapa pemanggilan dalam satu render tidak mengulang fetch.
const cache = new Map<string, { at: number; rows: Promise<Row[]> }>();
export const api = (table: string) => {
  const hit = cache.get(table);
  if (hit && Date.now() - hit.at < 2000) return hit.rows;
  const rows = fetch(`${base}/api/${table}`).then((r) => r.json());
  cache.set(table, { at: Date.now(), rows });
  return rows;
};
export const json = (v: unknown) => (typeof v === 'string' ? JSON.parse(v) : v ?? {});
// = Entry::dropDisabledRows di source: baris replicator dengan _enabled off dibuang (rekursif, semua modul) dan key _enabled-nya dihapus,
// jadi template tidak perlu menyaring sendiri. Baris tanpa _enabled dianggap aktif.
const live = (v: any): any => Array.isArray(v) ? v.filter((r) => r?._enabled ?? true).map(live) : v && typeof v === 'object' ? Object.fromEntries(Object.entries(v).filter(([k]) => k !== '_enabled').map(([k, x]) => [k, live(x)])) : v;

export type Entry = Row & { id: string; site: string; slug: string; uri: string; collection: string; blueprint: string; data: Row };
type Options = { site?: string; where?: Row; sort?: string | string[]; random?: boolean; limit?: number; related?: Entry | string };

const COLUMNS = ['id', 'site', 'status', 'slug', 'uri', 'date', 'order', 'collection', 'taxonomy', 'created_at', 'updated_at'];
const val = (r: Row, f: string) => (COLUMNS.includes(f) ? r[f] : r.data?.[f]);

function query(rows: Row[], o: Options) {
  if (o.site) rows = rows.filter((r) => r.site === o.site);
  for (const [f, v] of Object.entries(o.where ?? {})) rows = rows.filter((r) => val(r, f) == v);
  if (o.random) rows = [...rows].sort(() => Math.random() - 0.5);
  else for (const s of [o.sort ?? []].flat().reverse()) { // kunci terakhir dulu → kunci pertama dominan
    const [f, dir = 'asc'] = s.split(':');
    rows = [...rows].sort((a, b) => (val(a, f) > val(b, f) ? 1 : val(a, f) < val(b, f) ? -1 : 0) * (dir === 'desc' ? -1 : 1));
  }
  return o.limit ? rows.slice(0, o.limit) : rows;
}

export const sites = async () => (await api('sites')).sort((a, b) => a.order - b.order);
const originSite = async () => (await sites())[0]?.handle;

// Blueprint per handle (PageController mencari dari entry.blueprint) atau per namespace ("globals.site_settings").
export const blueprint = async (handle: string) => (await api('blueprints')).find((b) => b.handle === handle);
const fieldsOf = async (ns: string) => json((await api('blueprints')).find((b) => b.namespace === ns)?.data).fields ?? [];

// = resolveAssetFields: id asset → objek asset, rekursif ke dalam replicator.
async function resolveAssets(data: Row, fields: Row[]): Promise<Row> {
  const assets = (await api('assets')) as Row[];
  const walk = (d: Row, fs: Row[]): Row => {
    d = { ...d };
    for (const f of fs) {
      if (f.type === 'replicator') d[f.handle] = ([d[f.handle] ?? []].flat() as Row[]).map((row) => walk(row, f.replicator_fields ?? []));
      else if (f.type === 'assets') {
        const ids = (Array.isArray(d[f.handle]) ? d[f.handle] : []).map(Number);
        const found = ids.map((id: number) => assets.find((a) => a.id === id)).filter(Boolean);
        d[f.handle] = f.max_items == null || f.max_items > 1 ? found : found[0] ?? null;
      }
    }
    return d;
  };
  return walk(live(data), fields);
}

// data entry siap pakai di template: JSON ter-parse + field assets ter-resolve.
export const entryData = async (e: Row): Promise<Row> => resolveAssets(json(e.data), json((await blueprint(e.blueprint))?.data).fields ?? []);

// @collection: hanya entry published (/api/entries sudah memfilternya).
export async function collection(handle: string, o: Options = {}): Promise<Entry[]> {
  let rows = await Promise.all((await api('entries')).filter((e) => e.collection === handle).map(async (e) => { const data = await entryData(e); return { ...data, ...e, url: '/' + (await entryPath(e)), data }; })); // field data diratakan: e.title (kolom menang bila bentrok), e.data.title tetap ada
  if (o.related) {
    const rel = typeof o.related === 'string' ? rows.find((e) => e.id === o.related) : o.related;
    const taxFields = json((await blueprint(rel?.blueprint))?.data).fields?.filter((f: Row) => f.type === 'taxonomy').map((f: Row) => f.handle) ?? [];
    const ids = new Set(taxFields.flatMap((h: string) => [rel?.data?.[h] ?? []].flat()));
    rows = rows.filter((e) => e.id !== rel?.id && taxFields.some((h: string) => [e.data[h] ?? []].flat().some((t: unknown) => ids.has(t))));
  }
  return query(rows, { ...o, sort: o.random || o.sort ? o.sort : ['order', 'date:desc'] }) as Entry[];
}

export async function taxonomy(handle: string, o: Options = {}): Promise<Row[]> {
  return query((await api('taxonomy_terms')).filter((t) => t.taxonomy === handle).map((t) => { const data = live(json(t.data)); return { ...data, ...t, data }; }), o); // diratakan: t.title / t.news_label, t.data.title tetap ada
}

// @global: field non-localizable selalu dari site asal; localizable/replicator memakai nilai site sendiri bila terisi.
export async function globalSet(handle: string, o: { site?: string } = {}): Promise<Row | null> {
  const origin = await originSite(), site = o.site ?? origin;
  const rows = (await api('global_set_variables')).filter((g) => g.handle === handle);
  const pick = (l?: string) => rows.find((g) => (g.locale ?? '') === (l ?? ''));
  const first = pick(origin) ?? pick('');
  if (!first) return null;
  const data = json(first.data), own = site !== origin ? pick(site) : undefined, fields = await fieldsOf(`globals.${handle}`);
  if (own) for (const f of fields) if ((f.type === 'replicator' || f.localizable) && json(own.data)[f.handle]) data[f.handle] = json(own.data)[f.handle];
  return resolveAssets(data, fields);
}

// @navigation: mode blueprint → objek nilai field; mode lama → daftar item. locale NULL bila hanya satu site.
export async function navigation(handle: string, o: { site?: string } = {}): Promise<any> {
  const multi = (await sites()).length > 1, origin = await originSite();
  const tree = async (l?: string) => (await api('trees')).find((t) => t.handle === handle && t.type === 'navigation' && (t.locale ?? '') === (l ?? ''));
  const first = await tree(multi ? origin : undefined);
  if (!first) return null;
  const data = json(first.tree), site = o.site ?? origin, own = multi && site !== origin ? await tree(site) : undefined;
  if (Array.isArray(data)) return data;
  const fields = await fieldsOf(`navigations.${handle}`);
  if (own) for (const f of fields) if (f.localizable && json(own.tree)[f.handle] !== undefined) data[f.handle] = json(own.tree)[f.handle];
  return resolveAssets(data, fields);
}

// Semua yang dibutuhkan template, format fitur.handle.field: global.global_sidebar.site_image.url, navigation.header.menu_loop,
// collection.pages (daftar entry site ini), taxonomy.categories (daftar term), plus site, sites. collection/taxonomy juga tetap bisa
// dipanggil sebagai fungsi dengan opsi: collection('pages', { sort: 'date:desc', limit: 5 }). Diisi ke Astro.locals oleh pages/[...slug].astro.
// shortcut: semua collection/taxonomy dimuat tiap request (cache api 2 dtk); batasi ke handle yang dipakai bila datanya besar.
export async function context(siteHandle?: string) {
  const all = await sites(), site = all.find((s) => s.handle === siteHandle) ?? all[0];
  const keyed = async (table: string, load: (handle: string, o: { site?: string }) => Promise<unknown>) =>
    Object.fromEntries(await Promise.all((await api(table)).map(async (r) => [r.handle, await load(r.handle, { site: site?.handle })])));
  const both = async <F extends (...a: any[]) => any>(fn: F, table: string, load: (h: string, o: { site?: string }) => Promise<unknown>) => {
    const call = ((...a: Parameters<F>) => fn(...a)) as F & Record<string, any>;
    for (const [h, v] of Object.entries(await keyed(table, load))) Object.defineProperty(call, h, { value: v, enumerable: true }); // defineProperty: handle "name"/"length" tidak bentrok
    return call;
  };
  return {
    site, sites: all,
    global: await keyed('global_sets', globalSet),
    navigation: await keyed('navigations', navigation),
    collection: await both(collection, 'collections', collection),
    taxonomy: await both(taxonomy, 'taxonomies', taxonomy),
  };
}

export const formAction = (handle: string) => `${base}/!/forms/${handle}`;

// Path publik entry = prefix path URL site + entry.uri (uri homepage "" atau "/").
export const entryPath = async (e: Entry) => {
  const site = (await sites()).find((s) => s.handle === e.site);
  // order 1 = homepage (uri ""), apa pun isi kolom uri yang bisa basi; selain itu uri kosong yang basi (bekas homepage) jatuh ke slug.
  const uri = Number(e.order) === 1 ? '' : (e.uri ?? '').replace(/^\/+$/, '') || e.slug;
  return [new URL(site?.url || '/', 'http://x').pathname, uri].join('/').split('/').filter(Boolean).join('/');
};

// Payload live preview dari Go (route admin): cookie sesi CMS dari request diteruskan, jadi tanpa login = null (404), seperti PreviewController di belakang auth.
export async function previewPayload(token: string, cookie: string): Promise<Row | null> {
  const r = await fetch(`${base}/${import.meta.env.CMS_ROUTE ?? 'admin-panel'}/collections/api/preview/${encodeURIComponent(token)}`, { headers: { cookie }, redirect: 'manual' }).catch(() => null);
  return r?.ok ? r.json() : null;
}
