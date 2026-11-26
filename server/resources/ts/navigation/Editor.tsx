// Padanan NavigationTreeEditor (mode legacy tree) + navigation-tree-editor.blade.php. Semua edit terjadi di memori;
// hanya tombol Save (atau Ctrl+S) yang menyimpan, persis seperti di source.
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { useEffect, useMemo, useRef, useState } from 'react';
import { Badge, Checkbox, ContextMenu, type MenuAction, useToasts } from '../shared/flux';
import { Button, Heading, Icon, Input, Modal, ModalActions, Select, Text, Toasts, cn } from '../shared/ui';
import { type TreeData, client } from './api';
import { SortList } from './SortList';
import * as T from './tree';

type Role = TreeData['roles'][number];

interface Ctx {
  data: TreeData; expanded: string[]; sidebar: boolean; canEdit: boolean;
  toggle: (id: string) => void; patch: (id: string, p: Partial<T.Item>) => void; setType: (id: string, t: T.Item['type']) => void;
  addChild: (parent: string) => void; roleTitle: (h: string) => string; openMenu: (e: React.MouseEvent<HTMLElement>, id: string) => void;
  sortChildren: (parent: string) => (id: string, pos: number) => void;
}

// Satu baris item: header (grip, judul, aksi) + form edit saat terbuka. Anak hanya satu level (kedalaman maks 2).
// Komponen level atas (bukan di dalam Editor) supaya input tidak kehilangan fokus tiap render.
function Row({ item, child, c }: { item: T.Item; child: boolean; c: Ctx }) {
  const open = c.expanded.includes(item.id);
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({ id: item.id });
  return (
    <li ref={setNodeRef} style={{ transform: CSS.Translate.toString(transform), transition }}
      className={cn('relative overflow-hidden rounded-lg border border-zinc-200 bg-white dark:border-zinc-700 dark:bg-zinc-900', isDragging && 'z-20 border-blue-400 opacity-95 shadow-xl ring-2 ring-blue-400/40 dark:border-blue-500')}>
      <div className={cn('flex items-center gap-2 bg-zinc-50 px-3 dark:bg-zinc-800', !child && 'py-1.5')}>
        <span ref={setActivatorNodeRef} {...attributes} {...listeners} className={cn('shrink-0 touch-none text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300', isDragging ? 'cursor-grabbing' : 'cursor-grab')}><Icon name="bars-3" className="size-4" /></span>
        <button type="button" onClick={() => c.toggle(item.id)} className="flex min-w-0 flex-1 cursor-pointer items-center gap-2 text-left font-medium">
          <span className="truncate">{item.title !== '' ? item.title : '(untitled)'}</span>
          {c.sidebar && !open && item.roles.map((r) => <Badge key={r}>{c.roleTitle(r)}</Badge>)}
        </button>
        {c.canEdit && (
          <div className="relative flex shrink-0 items-center gap-1">
            {!child && <Button className="h-6 gap-1 ps-1 pe-2 text-xs" aria-label="Add child" onClick={() => c.addChild(item.id)}><Icon name="plus" className="size-3" /><span className="hidden sm:inline">Add child</span></Button>}
            <button type="button" onClick={(e) => c.openMenu(e, item.id)} className="cursor-pointer rounded p-1.5 hover:bg-zinc-200 dark:hover:bg-zinc-700"><Icon name="ellipsis-horizontal" className="size-4" /></button>
          </div>
        )}
      </div>
      {open && <ItemForm item={item} child={child} c={c} />}
      {open && !child && item.children.length > 0 && <Children parent={item} c={c} />}
    </li>
  );
}

function ItemForm({ item, child, c }: { item: T.Item; child: boolean; c: Ctx }) {
  return (
    <div className="space-y-4 bg-white p-4 dark:bg-zinc-900">
      <Input label="Title" placeholder={child ? 'Sub page' : 'About us'} value={item.title} onChange={(e) => c.patch(item.id, { title: e.target.value })} />
      <Select label="Link type" value={item.type} onChange={(e) => c.setType(item.id, e.target.value as T.Item['type'])}>
        <option value="internal">Internal path</option><option value="external">External URL</option><option value="page">Page</option>
      </Select>
      {item.type === 'page' ? (
        <Select label="Page" value={item.page_id} onChange={(e) => c.patch(item.id, { page_id: e.target.value })}>
          <option value="">Select a page</option>
          {c.data.pages.map((p) => <option key={p.id} value={p.id}>{p.title}</option>)}
        </Select>
      ) : (
        <Input label="URL" placeholder={item.type === 'external' ? 'https://example.com' : '/contact'} value={item.url} onChange={(e) => c.patch(item.id, { url: e.target.value })} />
      )}
      <Select label="Open in" value={item.target} onChange={(e) => c.patch(item.id, { target: e.target.value as T.Item['target'] })}>
        <option value="_self">Same tab</option><option value="_blank">New tab</option>
      </Select>
      {c.sidebar && (
        <div className="block">
          <label className="mb-2 inline-flex items-center text-sm font-medium text-zinc-800 dark:text-white">Visible to</label>
          <p className="mb-3 text-sm text-zinc-500 dark:text-white/60">Which roles see this link in their sidebar. A Super Admin always sees everything.</p>
          <div className="space-y-3">
            {c.data.roles.map((r) => (
              <Checkbox key={r.handle} label={r.title} checked={item.roles.includes(r.handle)}
                onChange={(v) => c.patch(item.id, { roles: v ? [...item.roles, r.handle] : item.roles.filter((x) => x !== r.handle) })} />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function Children({ parent, c }: { parent: T.Item; c: Ctx }) {
  return (
    <ul className="space-y-2 border-t border-zinc-200 bg-white p-3 pl-4 sm:pl-9 dark:border-zinc-700 dark:bg-zinc-900">
      <SortList ids={parent.children.map((x) => x.id)} onSort={c.sortChildren(parent.id)}>
        {parent.children.map((x) => <Row key={x.id} item={x} child c={c} />)}
      </SortList>
    </ul>
  );
}

export function Editor({ cp, handle }: { cp: string; handle: string }) {
  const api = useMemo(() => client(cp), [cp]);
  const { toasts, toast } = useToasts();
  const [data, setData] = useState<TreeData | null>(null);
  const [tree, setTree] = useState<T.Item[]>([]);
  const [expanded, setExpanded] = useState<string[]>([]);
  const [menu, setMenu] = useState<{ id: string; x: number; y: number } | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [moving, setMoving] = useState<{ id: string; parent: string; strategy: string } | null>(null);
  const [saving, setSaving] = useState(false);
  const [loadError, setLoadError] = useState('');

  const fail = (e: unknown) => toast(e instanceof Error ? e.message : String(e), true);
  useEffect(() => { api.tree(handle).then((d) => { setData(d); setTree(d.tree); }).catch((e) => setLoadError(e.message)); }, [api, handle]);

  // Mutasi tree: bila melanggar aturan (mis. kedalaman) tampilkan toast, tree tidak berubah.
  const apply = (fn: (t: T.Item[]) => T.Item[]) => { try { setTree((t) => fn(t)); return true; } catch (e) { fail(e); return false; } };
  const toggle = (id: string) => setExpanded((x) => (x.includes(id) ? x.filter((i) => i !== id) : [...x, id]));
  const patch = (id: string, p: Partial<T.Item>) => setTree((t) => T.update(t, id, p));
  const setType = (id: string, type: T.Item['type']) => patch(id, { type, page_id: type === 'page' ? undefined : '', url: type === 'page' ? '' : undefined } as Partial<T.Item>);

  const canEdit = !!data?.canEdit;
  const save = async () => {
    if (!canEdit || saving) return;
    setSaving(true);
    try { await api.saveTree(handle, tree); toast('Navigation saved.'); const d = await api.tree(handle); setData(d); setTree(d.tree); } catch (e) { fail(e); }
    setSaving(false);
  };
  const saveRef = useRef(save); saveRef.current = save;
  useEffect(() => {
    const k = (e: KeyboardEvent) => { if ((e.ctrlKey || e.metaKey) && e.key === 's') { e.preventDefault(); saveRef.current(); } };
    addEventListener('keydown', k); return () => removeEventListener('keydown', k);
  }, []);

  const addRoot = () => { const n = T.blank(); setTree((t) => T.add(t, null, n)); setExpanded((x) => [...x, n.id]); };
  const addChild = (parent: string) => { const n = T.blank(); if (apply((t) => T.add(t, parent, n))) setExpanded((x) => [...new Set([...x, parent, n.id])]); };
  const sortRoot = (id: string, pos: number) => apply((t) => T.move(t, id, null, pos));
  const sortChildren = (parent: string) => (id: string, pos: number) => apply((t) => T.move(t, id, parent, pos));

  const movingItem = moving ? T.find(tree, moving.id) : null;
  const confirmMove = () => {
    if (!moving || !movingItem) return;
    const target = moving.parent || null;
    let next = tree;
    if (movingItem.children.length && target !== null) {
      if (!moving.strategy) return toast("Choose what happens to this item's children first.", true);
      next = moving.strategy === 'promote' ? T.promoteChildren(next, moving.id) : T.clearChildren(next, moving.id);
    }
    try {
      const siblings = target !== null ? T.find(next, target)!.children : next;
      setTree(T.move(next, moving.id, target, siblings.length));
      setMoving(null);
    } catch (e) { fail(e); }
  };

  const menuActions = (id: string): MenuAction[] => [
    { label: 'Duplicate item', run: () => apply((t) => T.duplicate(t, id)) },
    { label: 'Move to...', run: () => setMoving({ id, parent: '', strategy: '' }) },
    { label: 'Delete item', danger: true, run: () => setDeleting(id) },
  ];


  if (!data) return loadError ? <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-200">Navigation failed to load: {loadError}</div> : null;

  const roleTitle = (h: string) => data.roles.find((r: Role) => r.handle === h)?.title ?? h;
  const sidebar = handle === 'sidebar-admin';

  const ctx: Ctx = {
    data, expanded, sidebar, canEdit, toggle, patch, setType, addChild, roleTitle,
    openMenu: (e, id) => { e.stopPropagation(); if (menu?.id === id) return setMenu(null); const r = e.currentTarget.getBoundingClientRect(); setMenu({ id, x: Math.max(8, Math.min(r.right - 160, innerWidth - 168)), y: r.bottom + 4 }); },
    sortChildren,
  };

  return (
    <div className="space-y-6" onClick={() => setMenu(null)}>
      <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-zinc-200 px-4 py-2 dark:border-zinc-700">
        <div className="flex items-center gap-2">
          <svg className="h-6 w-6 text-gray-800 dark:text-white" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="24" height="24" fill="none" viewBox="0 0 24 24">
            <path stroke="currentColor" strokeLinecap="round" strokeWidth="2" d="M6 4v10m0 0a2 2 0 1 0 0 4m0-4a2 2 0 1 1 0 4m0 0v2m6-16v2m0 0a2 2 0 1 0 0 4m0-4a2 2 0 1 1 0 4m0 0v10m6-16v10m0 0a2 2 0 1 0 0 4m0-4a2 2 0 1 1 0 4m0 0v2" />
          </svg>
          <h1 className="text-2xl font-medium text-zinc-800 dark:text-white">{data.navigation.title}</h1>
        </div>
        {canEdit && data.navigation.protected && (
          <div className="flex gap-2">
            <Button onClick={addRoot}>Add item</Button>
            <Button variant="primary" disabled={saving} onClick={save}>Save</Button>
          </div>
        )}
      </div>

      {!data.navigation.protected ? (
        <div className="rounded-lg border border-zinc-200 bg-zinc-50 px-4 py-3 text-sm text-zinc-700 dark:border-zinc-600 dark:bg-zinc-700/50 dark:text-zinc-200">
          This navigation has no blueprint yet. Its content is edited through blueprint fields, which arrive with the Blueprints module.
        </div>
      ) : (
        <section className="mx-auto mt-3 w-full space-y-4 rounded-lg border border-gray-200 bg-white p-4 sm:p-6 dark:border-gray-700 dark:bg-zinc-900">
          <ul className="space-y-2">
            {tree.length === 0 && <li className="text-zinc-500">No menu items yet.</li>}
            <SortList ids={tree.map((i) => i.id)} onSort={sortRoot}>
              {tree.map((i) => <Row key={i.id} item={i} child={false} c={ctx} />)}
            </SortList>
          </ul>
        </section>
      )}

      {menu && <ContextMenu x={menu.x} y={menu.y} actions={menuActions(menu.id)} close={() => setMenu(null)} />}

      <Modal open={deleting !== null} onClose={() => setDeleting(null)} className="max-w-sm">
        <div className="space-y-4">
          <Heading>Delete this item?</Heading>
          <Text>This will also delete any children under it.</Text>
          <ModalActions>
            <Button variant="filled" onClick={() => setDeleting(null)}>Cancel</Button>
            <Button variant="danger" onClick={() => { const id = deleting!; setDeleting(null); setTree((t) => T.remove(t, id)); }}>Delete</Button>
          </ModalActions>
        </div>
      </Modal>

      <Modal open={!!moving} onClose={() => setMoving(null)} className="max-w-sm">
        <div className="space-y-4">
          <Heading>Move item</Heading>
          <Select label="Move under" value={moving?.parent ?? ''} onChange={(e) => setMoving({ ...moving!, parent: e.target.value })}>
            <option value="">Level 1 (no parent)</option>
            {tree.filter((i) => i.id !== moving?.id).map((i) => <option key={i.id} value={i.id}>{i.title !== '' ? i.title : '(untitled)'}</option>)}
          </Select>
          {moving && moving.parent !== '' && (movingItem?.children.length ?? 0) > 0 && (
            <Select label="This item has children. What should happen to them?" value={moving.strategy} onChange={(e) => setMoving({ ...moving, strategy: e.target.value })}>
              <option value="">Choose an option</option><option value="promote">Move them to level 1</option><option value="delete">Delete them</option>
            </Select>
          )}
          <ModalActions>
            <Button variant="filled" onClick={() => setMoving(null)}>Cancel</Button>
            <Button variant="primary" onClick={confirmMove}>Move</Button>
          </ModalActions>
        </div>
      </Modal>

      <Toasts toasts={toasts} />
    </div>
  );
}
