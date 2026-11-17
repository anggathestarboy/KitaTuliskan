// Klien JSON API modul Navigation (internal/modules/navigation/navigation.go + content.go).
import { type Item, normalize } from './tree';

export interface Nav { id: number; handle: string; title: string; protected: boolean; canEdit: boolean; canDelete: boolean; hasBlueprint: boolean; blueprint: string }
export interface BlueprintOption { handle: string; label: string }
export interface TreeData {
  navigation: { handle: string; title: string; protected: boolean };
  tree: Item[]; roles: { handle: string; title: string }[]; pages: { id: string; title: string }[]; canEdit: boolean;
}

// Bentuk field blueprint seperti disimpan modul Blueprints.
export interface Field {
  handle: string; type: string; display?: string; instructions?: string; placeholder?: string; required?: boolean; localizable?: boolean;
  tab?: string; section?: string; options?: string[] | string; replicator_fields?: Field[]; min_items?: number | null; max_items?: number | null;
  default?: unknown; validation?: string; toolbar?: string[]; collapsed_by_default?: boolean;
  conditions?: { mode?: string; field?: string; operator?: string; value?: string } | null;
}
export interface Blueprint { handle: string; tabs: { handle: string; display: string; sections?: { handle: string; display: string }[] }[]; fields: Field[] }
export interface Labelled { id: string; group: string; site: string; label: string }
export interface ContentData {
  navigation: { handle: string; title: string }; blueprint: Blueprint; sites: { handle: string; name: string }[];
  activeSite: string; isOrigin: boolean; data: Record<string, unknown>; canEdit: boolean;
  bardContainer: string; entries: Labelled[]; terms: Labelled[]; authors: { id: string; name: string }[];
}

export class ApiError extends Error {
  constructor(message: string, public errors: Record<string, string> = {}) { super(message); }
}

async function call<T>(url: string, body?: unknown): Promise<T> {
  const r = await fetch(url, body === undefined ? undefined : { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  const j = await r.json().catch(() => ({}));
  if (!r.ok) throw new ApiError(j.error || 'Something went wrong.', j.errors);
  return j;
}

export const client = (cp: string) => {
  const base = `${cp}/navigations/api`;
  const site = (s: string) => (s ? `?site=${encodeURIComponent(s)}` : '');
  return {
    list: () => call<{ navigations: Nav[]; canCreate: boolean; blueprints: BlueprintOption[] }>(`${base}/list`),
    save: (nav: { id: number; handle: string; title: string; blueprint: string; createBlueprint: boolean }) => call<{ blueprintCreated: boolean }>(`${base}/save`, nav),
    remove: (id: number) => call(`${base}/delete`, { id }),
    tree: async (handle: string): Promise<TreeData> => { const d = await call<TreeData>(`${base}/tree/${encodeURIComponent(handle)}`); return { ...d, tree: normalize(d.tree) }; },
    saveTree: (handle: string, tree: Item[]) => call(`${base}/tree/${encodeURIComponent(handle)}`, { tree }),
    content: (handle: string, s = '') => call<ContentData>(`${base}/content/${encodeURIComponent(handle)}${site(s)}`),
    saveContent: (handle: string, s: string, data: unknown) => call(`${base}/content/${encodeURIComponent(handle)}${site(s)}`, { data }),
  };
};

// Str::slug($title) untuk tombol "Regenerate from title".
export const slug = (t: string) => t.toLowerCase().normalize('NFKD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');

export const visit = (url: string) => { const t = (window as unknown as { Turbo?: { visit: (u: string) => void } }).Turbo; t ? t.visit(url) : (location.href = url); };
