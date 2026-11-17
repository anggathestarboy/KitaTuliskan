// Klien JSON API modul Navigation (internal/modules/navigation/navigation.go).
import { type Item, normalize } from './tree';

export interface Nav { id: number; handle: string; title: string; protected: boolean; canEdit: boolean; canDelete: boolean; hasBlueprint: boolean; blueprint: string }
export interface TreeData {
  navigation: { handle: string; title: string; protected: boolean };
  tree: Item[]; roles: { handle: string; title: string }[]; pages: { id: string; title: string }[]; canEdit: boolean;
}

export class ApiError extends Error {}

async function call<T>(url: string, body?: unknown): Promise<T> {
  const r = await fetch(url, body === undefined ? undefined : { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  const j = await r.json().catch(() => ({}));
  if (!r.ok) throw new ApiError(j.error || 'Something went wrong.');
  return j;
}

export const client = (cp: string) => {
  const base = `${cp}/navigations/api`;
  return {
    list: () => call<{ navigations: Nav[]; canCreate: boolean }>(`${base}/list`),
    save: (nav: { id: number; handle: string; title: string }) => call(`${base}/save`, nav),
    remove: (id: number) => call(`${base}/delete`, { id }),
    tree: async (handle: string): Promise<TreeData> => { const d = await call<TreeData>(`${base}/tree/${encodeURIComponent(handle)}`); return { ...d, tree: normalize(d.tree) }; },
    saveTree: (handle: string, tree: Item[]) => call(`${base}/tree/${encodeURIComponent(handle)}`, { tree }),
  };
};

// Str::slug($title) untuk tombol "Regenerate from title".
export const slug = (t: string) => t.toLowerCase().normalize('NFKD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');

export const visit = (url: string) => { const t = (window as unknown as { Turbo?: { visit: (u: string) => void } }).Turbo; t ? t.visit(url) : (location.href = url); };
