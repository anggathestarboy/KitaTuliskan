// Fetch data dari Go saat build. Tabel yang diizinkan: lihat /api/{table} di server/main.go.
// Pengganti @collection(), @taxonomy(), @global(), @sites() di source.
const base = import.meta.env.CMS_URL ?? 'http://localhost:8000';
const api = <T = Record<string, unknown>>(table: string) => fetch(`${base}/api/${table}`).then((r) => r.json() as Promise<T[]>);
const json = (v: unknown) => (typeof v === 'string' ? JSON.parse(v) : v);

export type Entry = { id: string; site: string; slug: string; uri: string; status: string; collection: string; blueprint: string; data: Record<string, unknown> };

export const collection = async (handle: string, site?: string): Promise<Entry[]> =>
  (await api<Entry>('entries')).filter((e) => e.collection === handle && (!site || e.site === site)).map((e) => ({ ...e, data: json(e.data) }));
export const taxonomy = async (handle: string) => (await api('taxonomy_terms')).filter((t) => t.taxonomy === handle).map((t) => ({ ...t, data: json(t.data) }));
export const global = async (handle: string) => { const v = (await api('global_set_variables')).find((g) => g.handle === handle); return v ? json(v.data) : {}; };
export const sites = () => api('sites');
export const nav = async (handle: string) => { const t = (await api('trees')).find((t) => t.handle === handle && t.type === 'navigation'); return t ? json(t.tree) : []; };
export const formAction = (handle: string) => `${base}/!/forms/${handle}`;
