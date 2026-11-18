// Pengganti Blade directive proyek lama (TemplatingServiceProvider); dipanggil di frontmatter .astro saat build:
//   @collection('blog', [...])  → const posts = await collection('blog', { limit: 3 })      lalu posts.map(p => p.data.title)
//   @taxonomy('categories')     → const terms = await taxonomy('categories')
//   @sites()                    → const all = await sites()
//   @global('site_settings')    → const global = await globalSet('site_settings')          lalu global.site_name
//   @navigation('header')       → const navigation = await navigation('header', { site })  lalu navigation.navbar_loop
// Opsi collection/taxonomy sama dengan source: site, where, sort ("field:desc" | array), random, limit, related (hanya collection).
// shortcut: filter di memori dari /api/{table} (cukup untuk build statis skala kecil), pindah ke query SQL bila data besar.
// shortcut: link "entry::{id}" dan id asset di data belum di-resolve (source: resolveLinkFields/resolveAssetFields).
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

// @collection: hanya entry published (/api/entries sudah memfilternya).
export async function collection(handle: string, o: Options = {}): Promise<Entry[]> {
  let rows = (await api('entries')).filter((e) => e.collection === handle).map((e) => ({ ...e, data: json(e.data) }));
  if (o.related) {
    const rel = typeof o.related === 'string' ? rows.find((e) => e.id === o.related) : o.related;
    const taxFields = json((await blueprint(rel?.blueprint))?.data).fields?.filter((f: Row) => f.type === 'taxonomy').map((f: Row) => f.handle) ?? [];
    const ids = new Set(taxFields.flatMap((h: string) => [rel?.data?.[h] ?? []].flat()));
    rows = rows.filter((e) => e.id !== rel?.id && taxFields.some((h: string) => [e.data[h] ?? []].flat().some((t: unknown) => ids.has(t))));
  }
  return query(rows, { ...o, sort: o.random || o.sort ? o.sort : ['order', 'date:desc'] }) as Entry[];
}

export async function taxonomy(handle: string, o: Options = {}): Promise<Row[]> {
  return query((await api('taxonomy_terms')).filter((t) => t.taxonomy === handle).map((t) => ({ ...t, data: json(t.data) })), o);
}

// @global: field non-localizable selalu dari site asal; localizable/replicator memakai nilai site sendiri bila terisi.
export async function globalSet(handle: string, o: { site?: string } = {}): Promise<Row | null> {
  const origin = await originSite(), site = o.site ?? origin;
  const rows = (await api('global_set_variables')).filter((g) => g.handle === handle);
  const pick = (l?: string) => rows.find((g) => (g.locale ?? '') === (l ?? ''));
  const first = pick(origin) ?? pick('');
  if (!first) return null;
  const data = json(first.data), own = site !== origin ? pick(site) : undefined;
  if (own) for (const f of await fieldsOf(`globals.${handle}`)) if ((f.type === 'replicator' || f.localizable) && json(own.data)[f.handle]) data[f.handle] = json(own.data)[f.handle];
  return data;
}

// @navigation: mode blueprint → objek nilai field; mode lama → daftar item. locale NULL bila hanya satu site.
export async function navigation(handle: string, o: { site?: string } = {}): Promise<any> {
  const multi = (await sites()).length > 1, origin = await originSite();
  const tree = async (l?: string) => (await api('trees')).find((t) => t.handle === handle && t.type === 'navigation' && (t.locale ?? '') === (l ?? ''));
  const first = await tree(multi ? origin : undefined);
  if (!first) return null;
  const data = json(first.tree), site = o.site ?? origin, own = multi && site !== origin ? await tree(site) : undefined;
  if (own && !Array.isArray(data)) for (const f of await fieldsOf(`navigations.${handle}`)) if (f.localizable && json(own.tree)[f.handle] !== undefined) data[f.handle] = json(own.tree)[f.handle];
  return data;
}

export const formAction = (handle: string) => `${base}/!/forms/${handle}`;

// Path publik entry = prefix path URL site + entry.uri (uri homepage "" atau "/").
export const entryPath = async (e: Entry) => {
  const site = (await sites()).find((s) => s.handle === e.site);
  return [new URL(site?.url || '/', 'http://x').pathname, e.uri].join('/').split('/').filter(Boolean).join('/');
};
