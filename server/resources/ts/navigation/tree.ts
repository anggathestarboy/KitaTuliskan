// Operasi tree navigation di memori = NavigationTreeService + Structure\TreeService di source (kedalaman maks 2 level).
// Semua fungsi murni: mengembalikan tree baru, tidak mengubah input. Server tetap memvalidasi ulang saat simpan.
export const MAX_DEPTH = 2;

export interface Item {
  id: string; type: 'internal' | 'external' | 'page'; title: string; url: string; target: '_self' | '_blank';
  page_id: string; roles: string[]; data?: unknown; children: Item[];
}

export const blank = (): Item => ({ id: crypto.randomUUID(), type: 'internal', title: '', url: '', target: '_self', page_id: '', roles: [], children: [] });

// Data dari server bisa tidak punya field kosong (omitempty): lengkapi supaya input terkontrol selalu punya nilai.
export const normalize = (items: Partial<Item>[] | null): Item[] => (items ?? []).map((i) => ({
  id: i.id ?? crypto.randomUUID(), type: i.type ?? 'internal', title: i.title ?? '', url: i.url ?? '', target: i.target === '_blank' ? '_blank' : '_self',
  page_id: i.page_id ?? '', roles: i.roles ?? [], data: i.data, children: normalize(i.children ?? []),
}));

export const find = (nodes: Item[], id: string): Item | null => {
  for (const n of nodes) { if (n.id === id) return n; const f = find(n.children, id); if (f) return f; }
  return null;
};

const depthOf = (nodes: Item[], id: string, depth = 1): number | null => {
  for (const n of nodes) { if (n.id === id) return depth; const d = depthOf(n.children, id, depth + 1); if (d) return d; }
  return null;
};
const height = (n: Item): number => (n.children.length ? 1 + Math.max(...n.children.map(height)) : 1);

const map = (nodes: Item[], id: string, fn: (n: Item) => Item): Item[] => nodes.map((n) => (n.id === id ? fn(n) : { ...n, children: map(n.children, id, fn) }));
export const update = (nodes: Item[], id: string, patch: Partial<Item>) => map(nodes, id, (n) => ({ ...n, ...patch }));
export const remove = (nodes: Item[], id: string): Item[] => nodes.filter((n) => n.id !== id).map((n) => ({ ...n, children: remove(n.children, id) }));

export function add(nodes: Item[], parentId: string | null, node: Item): Item[] {
  if (parentId === null) return [...nodes, node];
  const d = depthOf(nodes, parentId);
  if (d === null) throw new Error(`Parent node not found: ${parentId}`);
  if (d + 1 > MAX_DEPTH) throw new Error('Maximum tree depth reached.');
  return map(nodes, parentId, (p) => ({ ...p, children: [...p.children, node] }));
}

export function move(nodes: Item[], id: string, parentId: string | null, position: number): Item[] {
  const node = find(nodes, id);
  if (!node) throw new Error(`Node not found: ${id}`);
  const rest = remove(nodes, id);
  let target = 1;
  if (parentId !== null) {
    const d = depthOf(rest, parentId);
    if (d === null) throw new Error(`Parent node not found: ${parentId}`);
    target = d + 1;
  }
  if (target + height(node) - 1 > MAX_DEPTH) throw new Error('Moving this item here would exceed the maximum tree depth.');
  const put = (list: Item[]) => [...list.slice(0, position), node, ...list.slice(position)];
  return parentId === null ? put(rest) : map(rest, parentId, (p) => ({ ...p, children: put(p.children) }));
}

const fresh = (n: Item): Item => ({ ...n, id: crypto.randomUUID(), children: n.children.map(fresh) });
export function duplicate(nodes: Item[], id: string): Item[] {
  const n = find(nodes, id);
  if (!n) throw new Error(`Node not found: ${id}`);
  const copy = fresh(n);
  const ins = (list: Item[]): Item[] => list.flatMap((x) => (x.id === id ? [x, copy] : [{ ...x, children: ins(x.children) }]));
  return ins(nodes);
}

// Anak item dijadikan sibling level 1 (setelah semua root), atau dibuang — pilihan di modal "Move item".
export function promoteChildren(nodes: Item[], id: string): Item[] {
  const n = find(nodes, id);
  if (!n || !n.children.length) return nodes;
  return [...update(nodes, id, { children: [] }), ...n.children];
}
export const clearChildren = (nodes: Item[], id: string) => update(nodes, id, { children: [] });
