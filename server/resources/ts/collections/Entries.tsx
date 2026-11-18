// Padanan EntryManager + entry-manager.blade.php + partials/entry-tree-row.blade.php: tabel (koleksi flat) atau kartu pohon
// (structured), cari + filter site, aksi massal, duplikat, hapus (+ versi bahasa lain) dan drag-and-drop urut (orderable).
import { DndContext, KeyboardSensor, PointerSensor, closestCenter, useSensor, useSensors, type Modifier } from '@dnd-kit/core';
import { SortableContext, arrayMove, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { useEffect, useState, type ReactNode } from 'react';
import { ago } from '../media/api';
import { call, visit } from '../navigation/api';
import { Cell, Checkbox, Column, ContextMenu, Menu, MenuItem, Table, useToasts, type MenuAction } from '../shared/flux';
import { Button, Heading, Icon, Input, Modal, ModalActions, Select, Text, Toasts, cn } from '../shared/ui';
import { COLORS } from '../taxonomies/Terms';

interface Entry {
  id: string; site: string; title: string; slug: string; blueprint: string; status: string; order: number | null; updatedAt: string;
  canEdit: boolean; canDelete: boolean; canPublish: boolean; siblingCount: number; children?: Entry[];
}
interface Data {
  collection: { handle: string; title: string; structured: boolean; orderable: boolean }; entries: Entry[]; sites: { handle: string; name: string }[];
  createSite: string; blueprints: string[]; defaultBlueprint: string; canCreate: boolean; statuses: Record<string, string>; bulkAvailable: boolean;
}

// flux:badge color="green|blue|amber|zinc"
const BADGE: Record<string, string> = {
  published: 'bg-green-400/20 text-green-800 dark:bg-green-400/40 dark:text-green-200',
  scheduled: 'bg-blue-400/20 text-blue-800 dark:bg-blue-400/40 dark:text-blue-200',
  need_review: 'bg-amber-400/25 text-amber-800 dark:bg-amber-400/40 dark:text-amber-200',
};
const StatusBadge = ({ status, label, sm }: { status: string; label: string; sm?: boolean }) => (
  <span className={cn('inline-flex items-center rounded-md font-medium whitespace-nowrap [print-color-adjust:exact]', sm ? 'px-2 py-1 text-xs' : 'px-2.5 py-1.5 text-sm',
    BADGE[status] ?? 'bg-zinc-400/15 text-zinc-700 dark:bg-zinc-400/40 dark:text-zinc-200')}>{label}</span>
);

const Home = ({ className }: { className: string }) => (
  <svg className={className} aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="24" height="24" fill="none" viewBox="0 0 24 24">
    <path stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="m4 12 8-8 8 8M6 10.5V19a1 1 0 0 0 1 1h3v-3a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v3h3a1 1 0 0 0 1-1v-8.5" />
  </svg>
);

// Drag & drop urut (wire:sort di source): tiap list (root, tiap daftar anak) punya DndContext sendiri → tidak bisa dipindah antar-list.
const verticalOnly: Modifier = ({ transform }) => ({ ...transform, x: 0 });
function SortList({ ids, onSort, children }: { ids: string[]; onSort: (id: string, position: number) => void; children: ReactNode }) {
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 4 } }), useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }));
  return (
    <DndContext sensors={sensors} collisionDetection={closestCenter} modifiers={[verticalOnly]} onDragEnd={({ active, over }) => {
      if (!over || active.id === over.id) return;
      onSort(String(active.id), arrayMove(ids, ids.indexOf(String(active.id)), ids.indexOf(String(over.id))).indexOf(String(active.id)));
    }}>
      <SortableContext items={ids} strategy={verticalListSortingStrategy}>{children}</SortableContext>
    </DndContext>
  );
}
const useRow = (id: string, on: boolean) => {
  const s = useSortable({ id, disabled: !on });
  return { ...s, style: { transform: CSS.Translate.toString(s.transform), transition: s.transition } };
};
const Grip = ({ s }: { s: ReturnType<typeof useRow> }) => (
  <span ref={s.setActivatorNodeRef} {...s.attributes} {...s.listeners} className={cn('shrink-0 touch-none text-zinc-400', s.isDragging ? 'cursor-grabbing' : 'cursor-grab')}><Icon name="bars-3" className="size-4" /></span>
);

interface Ctx {
  cp: string; handle: string; statuses: Record<string, string>; orderable: boolean; selected: string[]; bulk: boolean;
  toggle: (id: string) => void; duplicate: (id: string) => void; askDelete: (e: Entry) => void; openMenu: (e: React.MouseEvent, en: Entry) => void;
  reorder: (id: string, pos: number, parent: string | null) => void;
}

const Actions = ({ en, c }: { en: Entry; c: Ctx }) => (en.canEdit || en.canDelete) ? (
  <Menu trigger={<Button variant="ghost" icon="ellipsis-vertical" aria-label="Actions" />}>
    {en.canEdit && <MenuItem icon="pencil" onClick={() => visit(`${c.cp}/collections/${c.handle}/entries/${en.id}`)}>Edit</MenuItem>}
    {en.canEdit && <MenuItem icon="document-duplicate" onClick={() => c.duplicate(en.id)}>Duplicate</MenuItem>}
    {en.canDelete && <MenuItem icon="trash" danger onClick={() => c.askDelete(en)}>Delete</MenuItem>}
  </Menu>
) : null;

function FlatRow({ en, c }: { en: Entry; c: Ctx }) {
  const s = useRow(en.id, c.orderable);
  const href = `${c.cp}/collections/${c.handle}/entries/${en.id}`;
  return (
    <tr ref={s.setNodeRef} style={s.style} className={cn(s.isDragging && 'relative z-20 bg-white opacity-95 shadow-xl dark:bg-zinc-800')} onContextMenu={(e) => c.openMenu(e, en)}>
      <Cell>
        {c.orderable ? <Grip s={s} /> : c.bulk && <input type="checkbox" checked={c.selected.includes(en.id)} onChange={() => c.toggle(en.id)} />}
      </Cell>
      <Cell><div className="flex items-center gap-2"><span className={`size-2 rounded-full ${COLORS[en.site.toLowerCase()] ?? 'bg-gray-400'}`} /><span>{en.site}</span></div></Cell>
      <Cell>
        <div className="flex items-center gap-1">
          {en.order === 1 && <Home className="-mt-0.5 h-4 w-4 text-gray-800" />}
          {en.canEdit ? <a href={href} className="hover:text-orange-700">{en.title || '—'}</a> : <span>{en.title || '—'}</span>}
        </div>
      </Cell>
      <Cell>{en.slug || '—'}</Cell>
      <Cell className="w-[13%]">{en.updatedAt ? ago(en.updatedAt) : ''}</Cell>
      <Cell className="w-[10%]"><StatusBadge status={en.status} label={c.statuses[en.status] ?? en.status} /></Cell>
      <Cell className="w-32"><div className="flex justify-end gap-2"><Actions en={en} c={c} /></div></Cell>
    </tr>
  );
}

const Chip = ({ path, children }: { path: string; children: ReactNode }) => (
  <span className="flex max-w-full min-w-0 items-center gap-1 rounded-full border border-zinc-300 px-1.5 break-all dark:border-zinc-500">
    <svg className="h-3 w-3 shrink-0" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="24" height="24" fill="none" viewBox="0 0 24 24"><path stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d={path} /></svg>
    {children}
  </span>
);

function TreeRow({ en, c }: { en: Entry; c: Ctx }) {
  const s = useRow(en.id, c.orderable);
  const href = `${c.cp}/collections/${c.handle}/entries/${en.id}`;
  const kids = en.children ?? [];
  return (
    <li ref={s.setNodeRef} style={s.style} onContextMenu={(e) => c.openMenu(e, en)}
      className={cn('overflow-hidden rounded-lg border border-zinc-200 dark:border-zinc-700', s.isDragging && 'relative z-20 opacity-95 shadow-xl ring-2 ring-blue-400/40')}>
      <div className="flex items-center gap-2 bg-white p-1.5 sm:gap-3 dark:bg-zinc-900">
        <div className="flex flex-col items-center gap-2">
          <span className={`size-2 shrink-0 rounded-full ${COLORS[en.site.toLowerCase()] ?? 'bg-gray-400'}`} />
          {c.orderable && <Grip s={s} />}
        </div>
        <div className="min-w-0 flex-1">
          <div className="group flex min-w-0 items-center gap-1.5">
            {en.order === 1 && <Home className="h-3.5 w-3.5 text-gray-800 dark:text-zinc-400" />}
            {en.canEdit ? <a href={href} className="text-md font-medium break-words group-hover:text-orange-700">{en.title || '—'}</a> : <span className="text-md font-medium break-words">{en.title || '—'}</span>}
          </div>
          <div className="mt-1 flex flex-wrap gap-1 text-xs text-zinc-500 dark:text-zinc-400">
            <Chip path="M13.213 9.787a3.391 3.391 0 0 0-4.795 0l-3.425 3.426a3.39 3.39 0 0 0 4.795 4.794l.321-.304m-.321-4.49a3.39 3.39 0 0 0 4.795 0l3.424-3.426a3.39 3.39 0 0 0-4.794-4.795l-1.028.961">{en.slug || '—'}</Chip>
            <Chip path="M5 19V4a1 1 0 0 1 1-1h12a1 1 0 0 1 1 1v13H7a2 2 0 0 0-2 2Zm0 0a2 2 0 0 0 2 2h12M9 3v14m7 0v4">{en.blueprint || '—'}</Chip>
            <Chip path="M12 8v4l3 3m6-3a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z">{en.updatedAt ? ago(en.updatedAt) : ''}</Chip>
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1 sm:gap-2">
          <StatusBadge sm status={en.status} label={c.statuses[en.status] ?? en.status} />
          <Actions en={en} c={c} />
        </div>
      </div>
      {kids.length > 0 && (
        <ul className="space-y-1.5 border-t border-zinc-200 bg-gray-100 p-1.5 dark:border-zinc-700 dark:bg-zinc-950">
          <SortList ids={kids.map((k) => k.id)} onSort={(id, pos) => c.reorder(id, pos, en.id)}>
            {kids.map((k) => <TreeRow key={k.id} en={k} c={c} />)}
          </SortList>
        </ul>
      )}
    </li>
  );
}

// Pohon disaring seperti query EntryManager: anak yang induknya tidak lolos filter ikut hilang.
const prune = (nodes: Entry[], ok: (e: Entry) => boolean): Entry[] => nodes.filter(ok).map((n) => ({ ...n, children: prune(n.children ?? [], ok) }));

export function Entries({ cp, handle }: { cp: string; handle: string }) {
  const api = `${cp}/collections/api`;
  const { toasts, toast } = useToasts();
  const [d, setD] = useState<Data | null>(null);
  const [search, setSearch] = useState('');
  const [site, setSite] = useState<string | null>(null); // null = belum dipilih → site pertama
  const [selected, setSelected] = useState<string[]>([]);
  const [del, setDel] = useState<Entry | null>(null);
  const [allLocales, setAllLocales] = useState(true);
  const [bulkDel, setBulkDel] = useState(false);
  const [menu, setMenu] = useState<{ en: Entry; x: number; y: number } | null>(null);

  const load = () => call<Data>(`${api}/entry-list/${encodeURIComponent(handle)}`).then(setD).catch((e) => toast(e.message, true));
  useEffect(() => { load(); }, [handle]);
  const act = async (path: string, body: unknown, ok: string) => { // ok kosong = tanpa toast (reorder)
    try { await call(`${api}/${path}/${encodeURIComponent(handle)}`, body); if (ok) toast(ok); } catch (e) { toast(e instanceof Error ? e.message : String(e), true); }
    setSelected([]); load();
  };

  if (!d) return null;
  const { collection: col } = d;
  const filter = site ?? d.createSite;
  const q = search.toLowerCase();
  const match = (e: Entry) => !q || e.slug.toLowerCase().includes(q) || e.title.toLowerCase().includes(q);
  const bySite = (e: Entry) => !filter || e.site === filter;
  const shown = col.structured ? prune(d.entries.filter(bySite), match) : d.entries.filter((e) => match(e) && bySite(e));
  const flat: Entry[] = col.structured ? [] : shown;
  const orderable = col.orderable && search === '' && filter !== '';
  const bulk = d.bulkAvailable;
  const sel = flat.filter((e) => selected.includes(e.id));
  const allOn = flat.length > 0 && flat.every((e) => selected.includes(e.id));
  const canCreate = d.blueprints.length > 0 && d.canCreate && d.createSite !== '';
  const n = (k: number, one: string, many: string) => `${k} ${k === 1 ? one : many}`;

  const c: Ctx = {
    cp, handle, statuses: d.statuses, orderable, selected, bulk,
    toggle: (id) => setSelected((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id])),
    duplicate: (id) => act('entry-duplicate', { id }, 'Entry duplicated.'),
    askDelete: (e) => { setAllLocales(true); setDel(e); },
    openMenu: (e, en) => { if (!en.canEdit && !en.canDelete) return; e.preventDefault(); e.stopPropagation(); setMenu({ en, x: e.clientX - 20, y: e.clientY + 10 }); },
    reorder: (id, position, parentId) => act('entry-reorder', { id, position, parentId, site: filter }, ''),
  };
  const menuActions = (en: Entry): MenuAction[] => [
    en.canEdit && { label: 'Edit', run: () => visit(`${cp}/collections/${handle}/entries/${en.id}`) },
    en.canEdit && { label: 'Duplicate', run: () => c.duplicate(en.id) },
    en.canDelete && { label: 'Delete', danger: true, run: () => c.askDelete(en) },
  ].filter(Boolean) as MenuAction[];
  const canStatus = (st: string) => sel.every((e) => e.canEdit && (!['published', 'scheduled'].includes(st) || e.canPublish));

  return (
    <section className="w-full" onClick={() => setMenu(null)}>
      <div className="flex flex-wrap items-center justify-between gap-3 overflow-hidden rounded-xl border border-zinc-200 px-2 py-2 md:px-4 dark:border-zinc-500">
        <div className="flex items-center gap-2">
          <svg className="h-6 w-6 text-gray-800 dark:text-white" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="24" height="24" fill="none" viewBox="0 0 24 24">
            <path stroke="currentColor" strokeLinecap="round" strokeWidth="2" d="M7.111 20A3.111 3.111 0 0 1 4 16.889v-12C4 4.398 4.398 4 4.889 4h4.444a.89.89 0 0 1 .89.889v12A3.111 3.111 0 0 1 7.11 20Zm0 0h12a.889.889 0 0 0 .889-.889v-4.444a.889.889 0 0 0-.889-.89h-4.389a.889.889 0 0 0-.62.253l-3.767 3.665a.933.933 0 0 0-.146.185c-.868 1.433-1.581 1.858-3.078 2.12Zm0-3.556h.009m7.933-10.927 3.143 3.143a.889.889 0 0 1 0 1.257l-7.974 7.974v-8.8l3.574-3.574a.889.889 0 0 1 1.257 0Z" />
          </svg>
          <h1 className="text-2xl font-medium text-zinc-800 dark:text-white">{col.title}</h1>
        </div>
        {canCreate ? (
          <form method="GET" className="flex w-full items-center gap-2 sm:w-auto" action={`${cp}/collections/${handle}/entries/create/${d.createSite}`}>
            <Select small name="blueprint" defaultValue={d.defaultBlueprint}>{d.blueprints.map((b) => <option key={b} value={b}>{b}</option>)}</Select>
            <Button variant="primary" type="submit" className="shrink-0">Create {col.title}</Button>
          </form>
        ) : <Button variant="primary" disabled>New entry</Button>}
      </div>

      <div className="mt-3 space-y-5">
        {d.blueprints.length === 0 && <Text>This collection has no blueprint linked yet — link one in the Collections module before creating entries.</Text>}
        <div className="flex flex-wrap items-end gap-3">
          <div className="w-full min-w-0 flex-1 sm:w-auto sm:min-w-64"><Input small icon="magnifying-glass" placeholder="Search by slug or title…" value={search} onChange={(e) => { setSearch(e.target.value); setSelected([]); }} /></div>
          {d.sites.length > 1 && (
            <div className="w-full sm:w-48">
              <Select small value={filter} onChange={(e) => { setSite(e.target.value); setSelected([]); }}>
                <option value="">All sites</option>
                {d.sites.map((s) => <option key={s.handle} value={s.handle}>{s.name}</option>)}
              </Select>
            </div>
          )}
        </div>
      </div>

      <div className="mt-7 mb-3">{orderable && <p className="text-xs text-zinc-500 dark:text-white/70">Drag the handle to reorder. The top entry becomes the site's homepage.</p>}</div>

      {bulk && selected.length > 0 && (
        <div className="mb-3 flex flex-wrap items-end gap-2">
          <p className="text-sm text-zinc-500 dark:text-white/70">{sel.length} selected</p>
          {sel.every((e) => e.canEdit) && (
            <div className="w-40">
              <Select small value="" onChange={(e) => { const v = e.target.value; if (v) act('entry-bulk', { action: 'status', ids: selected, status: v }, n(selected.length, 'entry updated.', 'entries updated.')); }}>
                <option value="">Set status…</option>
                {Object.entries(d.statuses).filter(([v]) => canStatus(v)).map(([v, l]) => <option key={v} value={v}>{l}</option>)}
              </Select>
            </div>
          )}
          {sel.every((e) => e.canDelete) && <Button variant="danger" onClick={() => setBulkDel(true)}>Delete</Button>}
        </div>
      )}

      {col.structured ? (
        <ul className="space-y-2">
          {shown.length === 0 && <li className="text-sm text-zinc-500">{search || filter ? 'No entries match your filters.' : 'No entries yet.'}</li>}
          <SortList ids={shown.map((e) => e.id)} onSort={(id, pos) => c.reorder(id, pos, null)}>
            {shown.map((e) => <TreeRow key={e.id} en={e} c={c} />)}
          </SortList>
        </ul>
      ) : (
        <div className="overflow-x-auto rounded-xl bg-zinc-100 p-1.5 dark:bg-zinc-900">
          <div className="rounded-xl bg-white dark:bg-zinc-800">
            <SortList ids={flat.map((e) => e.id)} onSort={(id, pos) => c.reorder(id, pos, null)}>
              <Table>
                <thead className="bg-zinc-100 dark:bg-zinc-900">
                  <tr>
                    <Column className="w-8">{bulk && flat.length > 0 && <input type="checkbox" checked={allOn} onChange={() => setSelected(allOn ? [] : flat.map((e) => e.id))} />}</Column>
                    <Column className="w-25">Site</Column><Column>Title</Column><Column>Slug</Column><Column>Updated</Column><Column>Status</Column><Column className="w-32" />
                  </tr>
                </thead>
                <tbody>
                  {flat.length === 0 && <tr><Cell colSpan={7}>{search || filter ? 'No entries match your filters.' : 'No entries yet.'}</Cell></tr>}
                  {flat.map((e) => <FlatRow key={e.id} en={e} c={c} />)}
                </tbody>
              </Table>
            </SortList>
          </div>
        </div>
      )}

      {menu && <ContextMenu x={menu.x} y={menu.y} actions={menuActions(menu.en)} close={() => setMenu(null)} />}

      <Modal open={!!del} onClose={() => setDel(null)} className="min-w-[26rem]">
        <div className="space-y-4">
          <Heading className="text-lg">Delete this entry?</Heading>
          <Text>This cannot be undone.</Text>
          {del && del.siblingCount > 0 && <Checkbox label="Also delete its other language versions" description={`${del.siblingCount} other entries`} checked={allLocales} onChange={setAllLocales} />}
          <ModalActions>
            <Button variant="filled" onClick={() => setDel(null)}>Cancel</Button>
            <Button variant="danger" onClick={() => { const id = del!.id; setDel(null); act('entry-delete', { id, deleteLocales: allLocales }, 'Entry deleted.'); }}>Delete</Button>
          </ModalActions>
        </div>
      </Modal>

      <Modal open={bulkDel} onClose={() => setBulkDel(false)} className="min-w-[26rem]">
        <div className="space-y-4">
          <Heading className="text-lg">Delete {selected.length} selected entries?</Heading>
          <Text>This also deletes their other language versions. This cannot be undone.</Text>
          <ModalActions>
            <Button variant="filled" onClick={() => setBulkDel(false)}>Cancel</Button>
            <Button variant="danger" onClick={() => { setBulkDel(false); act('entry-bulk', { action: 'delete', ids: selected }, n(selected.length, 'entry deleted.', 'entries deleted.')); }}>Delete</Button>
          </ModalActions>
        </div>
      </Modal>

      <Toasts toasts={toasts} />
    </section>
  );
}
