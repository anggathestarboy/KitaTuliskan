// Padanan CollectionManager + collection-manager.blade.php.
import { useEffect, useState } from 'react';
import { call, slug, visit } from '../navigation/api';
import { Cell, Checkbox, Column, ContextMenu, Menu, MenuItem, Radio, Table, useToasts, type MenuAction } from '../shared/flux';
import { Button, Heading, Input, Modal, ModalActions, Text, Toasts, cn } from '../shared/ui';

interface Coll {
  id: number; handle: string; title: string; blueprint: string; sites: string[]; routeMode: string; route: string; routes: Record<string, string>;
  structured: boolean; orderable: boolean; visibleOrder: boolean; livePreview: boolean; schemaEnabled: boolean; seoEnabled: boolean;
  maxDepth: string; expectsRoot: boolean; saveAction: string;
}
interface Site { handle: string; name: string }
type Form = Omit<Coll, 'blueprint'>;

export function Manager({ cp }: { cp: string }) {
  const api = `${cp}/collections/api`;
  const { toasts, toast } = useToasts();
  const [list, setList] = useState<Coll[] | null>(null);
  const [canConfigure, setCanConfigure] = useState(false);
  const [sites, setSites] = useState<Site[]>([]);
  const [form, setForm] = useState<Form | null>(null);
  const [error, setError] = useState('');
  const [del, setDel] = useState<number | null>(null);
  const [menu, setMenu] = useState<{ id: number; x: number; y: number } | null>(null);

  // Site pertama = default/origin: selalu tercentang dan tidak bisa dilepas.
  const origin = sites[0]?.handle;
  const load = () => call<{ collections: Coll[]; sites: Site[]; canConfigure: boolean }>(`${api}/list`).then((d) => { setList(d.collections); setCanConfigure(d.canConfigure); setSites(d.sites); }).catch((e) => toast(e.message, true));
  useEffect(() => { load(); }, []);

  const open = (f: Form) => { setError(''); setForm(f); };
  const create = () => open({ id: 0, handle: '', title: '', sites: sites.map((s) => s.handle), routeMode: 'single', route: '', routes: {}, structured: false, orderable: false, visibleOrder: true, livePreview: false, schemaEnabled: false, seoEnabled: false, maxDepth: '', expectsRoot: false, saveAction: 'listing' });
  const edit = (c: Coll) => { const { blueprint: _, ...f } = c; open({ ...f, sites: c.sites.length ? c.sites : sites.map((s) => s.handle) }); };
  const set = (p: Partial<Form>) => setForm((f) => ({ ...f!, ...p }));

  // updatedSites(): default site selalu ikut; satu site saja → route kembali "single".
  const setSelected = (hs: string[]) => { const next = [...new Set([...hs, ...(origin ? [origin] : [])])]; set({ sites: next, ...(next.length <= 1 ? { routeMode: 'single' } : {}) }); };
  // updatedStructured(): sub-setting khusus tree ikut mati.
  const setStructured = (v: boolean) => set({ structured: v, ...(v ? {} : { maxDepth: '', expectsRoot: false }) });

  const save = async () => {
    if (!form) return;
    try {
      await call(`${api}/save`, form);
      setForm(null); toast('Collection saved.'); load();
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
  };
  const remove = async () => {
    const id = del!; setDel(null);
    try { await call(`${api}/delete`, { id }); toast('Collection deleted.'); } catch (e) { toast(e instanceof Error ? e.message : String(e), true); }
    load();
  };

  const actions = (c: Coll): MenuAction[] => [
    { label: 'Entries', run: () => visit(`${cp}/collections/${c.handle}`) },
    ...(canConfigure ? [{ label: 'Settings', run: () => edit(c) }, { label: 'Delete', danger: true, run: () => setDel(c.id) }] : []),
  ];

  if (!list) return null;
  const f = form;
  const vars = `Available variables: {slug}, {year}, {month}, {day}${f?.structured ? ', {parent_uri}' : ''}`;
  const multi = !!f && f.sites.length > 1;
  const checkRow = (label: string, description: string, key: keyof Form, onChange?: (v: boolean) => void) => (
    <Checkbox label={label} description={description} checked={!!f![key]} onChange={onChange ?? ((v) => set({ [key]: v } as Partial<Form>))} />
  );

  return (
    <section className="w-full space-y-6" onClick={() => setMenu(null)}>
      <div className="flex flex-wrap items-center justify-between gap-3 overflow-hidden rounded-xl border border-zinc-200 px-2 py-2 md:px-4 dark:border-zinc-500">
        <div className="flex items-center gap-2">
          <svg className="h-6 w-6 text-gray-800 dark:text-white" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="24" height="24" fill="none" viewBox="0 0 24 24">
            <path stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 6.03v13m0-13c-2.819-.831-4.715-1.076-8.029-1.023A.99.99 0 0 0 3 6v11c0 .563.466 1.014 1.03 1.007 3.122-.043 5.018.212 7.97 1.023m0-13c2.819-.831 4.715-1.076 8.029-1.023A.99.99 0 0 1 21 6v11c0 .563-.466 1.014-1.03 1.007-3.122-.043-5.018.212-7.97 1.023" />
          </svg>
          <h1 className="text-2xl font-medium text-zinc-800 dark:text-white">Collections</h1>
        </div>
        {canConfigure && <Button variant="primary" onClick={create}>New collection</Button>}
      </div>

      <div className="rounded-xl bg-zinc-100 p-1.5 dark:bg-zinc-900">
        <div className="rounded-xl bg-white dark:bg-zinc-800">
          <Table>
            <thead className="bg-zinc-100 dark:bg-zinc-900">
              <tr><Column>Title</Column><Column>Handle</Column><Column>Blueprint</Column><Column /></tr>
            </thead>
            <tbody>
              {list.length === 0 && <tr><Cell colSpan={4}>No collections yet.</Cell></tr>}
              {list.map((c) => (
                <tr key={c.id} onContextMenu={(e) => { e.preventDefault(); e.stopPropagation(); setMenu({ id: c.id, x: e.clientX - 20, y: e.clientY + 10 }); }}>
                  <Cell>{c.title}</Cell>
                  <Cell>{c.handle}</Cell>
                  <Cell>{c.blueprint || '—'}</Cell>
                  <Cell>
                    <div className="flex justify-end gap-2">
                      <Menu trigger={<Button variant="ghost" icon="ellipsis-vertical" aria-label="Actions" />}>
                        <MenuItem icon="document-text" onClick={() => visit(`${cp}/collections/${c.handle}`)}>Entries</MenuItem>
                        {canConfigure && <MenuItem icon="cog-6-tooth" onClick={() => edit(c)}>Settings</MenuItem>}
                        {canConfigure && <MenuItem icon="trash" danger onClick={() => setDel(c.id)}>Delete</MenuItem>}
                      </Menu>
                    </div>
                  </Cell>
                </tr>
              ))}
            </tbody>
          </Table>
        </div>
      </div>

      {menu && <ContextMenu x={menu.x} y={menu.y} actions={actions(list.find((x) => x.id === menu.id)!)} close={() => setMenu(null)} />}

      <Modal open={!!f} onClose={() => setForm(null)} className="max-w-lg">
        {f && (
          <form className="space-y-6" onSubmit={(e) => { e.preventDefault(); save(); }}>
            <Heading className="text-lg">{f.id ? 'Edit collection' : 'New collection'}</Heading>
            <Input label="Title" required autoFocus value={f.title} onChange={(e) => set({ title: e.target.value })} />
            <div className="flex items-end gap-2">
              <div className="flex-1"><Input label="Handle" placeholder="blog_posts" required value={f.handle} onChange={(e) => set({ handle: e.target.value })} /></div>
              <Button icon="arrow-path" title="Regenerate from title" aria-label="Regenerate from title" onClick={() => set({ handle: slug(f.title, '_') })} />
            </div>
            {error && <p className="text-sm font-medium text-red-500 dark:text-red-400">{error}</p>}

            {sites.length > 0 && (
              <div className="space-y-3">
                <div><p className="text-sm font-medium text-zinc-800 dark:text-white">Sites</p><Text className="mt-1">Which sites is this collection available on? Leave all checked for no restriction. The default site is the origin language and can't be removed.</Text></div>
                {sites.map((s) => (
                  <Checkbox key={s.handle} label={s.name} description={s.handle === origin ? 'Default — origin language' : undefined} disabled={s.handle === origin}
                    checked={f.sites.includes(s.handle)} onChange={(v) => setSelected(v ? [...f.sites, s.handle] : f.sites.filter((h) => h !== s.handle))} />
                ))}
              </div>
            )}

            {multi && (
              <div>
                <p className="mb-3 text-sm font-medium text-zinc-800 dark:text-white">Route</p>
                <div className="inline-flex w-full rounded-lg bg-zinc-800/5 p-0.5 dark:bg-white/10">
                  {([['single', 'Single'], ['per_site', 'Per-site']] as const).map(([v, l]) => (
                    <button key={v} type="button" onClick={() => set({ routeMode: v })}
                      className={cn('flex-1 cursor-pointer rounded-md px-3 py-1.5 text-sm font-medium', f.routeMode === v ? 'bg-white text-zinc-800 shadow-xs dark:bg-white/20 dark:text-white' : 'text-zinc-500 hover:text-zinc-800 dark:text-white/70 dark:hover:text-white')}>{l}</button>
                  ))}
                </div>
              </div>
            )}

            {f.routeMode === 'per_site' && multi
              ? <div className="space-y-4">{sites.filter((s) => f.sites.includes(s.handle)).map((s) => (
                <div key={s.handle}>
                  <Input label={`${s.name} route`} placeholder="blog/{slug}" description={vars} value={f.routes[s.handle] ?? ''} onChange={(e) => set({ routes: { ...f.routes, [s.handle]: e.target.value } })} />
                </div>
              ))}</div>
              : <Input label={multi ? undefined : 'Route'} placeholder={f.structured ? '{parent_uri}/{slug}' : 'blog/{slug}'} description={vars} value={f.route} onChange={(e) => set({ route: e.target.value })} />}

            <div className="space-y-4 border-t border-zinc-200 pt-6 dark:border-zinc-700">
              {checkRow('Structured', 'Let entries be nested under a parent entry, enabling the {parent_uri} route variable.', 'structured', setStructured)}
              {checkRow('Orderable', 'Let entries be manually reordered by drag-and-drop.', 'orderable')}
              {checkRow('Visible Order', 'Show the Order field in the entry form sidebar. Ordering still works even when hidden.', 'visibleOrder')}
              {checkRow('Live Preview', "Let editors preview an entry on the live site as they edit it, even before it's published.", 'livePreview')}
              {checkRow('Schema', "Add a Schema tab to this collection's entries for attaching schema.org JSON-LD blocks (Article, Organization, etc.) to individual entries.", 'schemaEnabled')}
              {checkRow('SEO', "Add an SEO tab to this collection's entries for meta title/description, robots, sitemap hints, etc.", 'seoEnabled')}
              {f.structured && (
                <>
                  <Input type="number" min={1} label="Max depth" placeholder="No limit" value={f.maxDepth} onChange={(e) => set({ maxDepth: e.target.value })} />
                  {checkRow('Expect a root page', 'Require exactly one top-level entry (e.g. a homepage) instead of allowing several.', 'expectsRoot')}
                </>
              )}
            </div>

            <div className="space-y-3">
              <div><p className="text-sm font-medium text-zinc-800 dark:text-white">Action after save</p><Text className="mt-1">Where the entry form goes after saving one of this collection's entries.</Text></div>
              <Radio label="Go to listing" checked={f.saveAction === 'listing'} onChange={() => set({ saveAction: 'listing' })} />
              <Radio label="Continue editing" checked={f.saveAction === 'editing'} onChange={() => set({ saveAction: 'editing' })} />
              <Radio label="Create another" checked={f.saveAction === 'create_another'} onChange={() => set({ saveAction: 'create_another' })} />
            </div>

            <ModalActions>
              <Button variant="filled" onClick={() => setForm(null)}>Cancel</Button>
              <Button variant="primary" type="submit">Save</Button>
            </ModalActions>
          </form>
        )}
      </Modal>

      <Modal open={del !== null} onClose={() => setDel(null)} className="min-w-[24rem]">
        <div className="space-y-4">
          <Heading className="text-lg">Delete this collection?</Heading>
          <Text>Collections with existing entries can't be deleted — delete those first. This cannot be undone.</Text>
          <ModalActions>
            <Button variant="filled" onClick={() => setDel(null)}>Cancel</Button>
            <Button variant="danger" onClick={remove}>Delete</Button>
          </ModalActions>
        </div>
      </Modal>

      <Toasts toasts={toasts} />
    </section>
  );
}
