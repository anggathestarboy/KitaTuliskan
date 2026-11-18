// Padanan GlobalSetManager + global-set-manager.blade.php.
import { useEffect, useState } from 'react';
import { Cell, Checkbox, Column, ContextMenu, Menu, MenuItem, Radio, Table, useToasts, type MenuAction } from '../shared/flux';
import { Button, Heading, Input, Modal, ModalActions, Select, Text, Toasts } from '../shared/ui';
import { type BlueprintOption, call, slug, visit } from '../navigation/api';

interface Set { id: number; handle: string; title: string; blueprint: string; sites: string[]; saveAction: string; canEdit: boolean; canDelete: boolean }
interface Site { handle: string; name: string }
interface Form { id: number; title: string; handle: string; blueprint: string; createBlueprint: boolean; sites: string[]; saveAction: string }

export function Manager({ cp }: { cp: string }) {
  const api = `${cp}/globals/api`;
  const { toasts, toast } = useToasts();
  const [sets, setSets] = useState<Set[] | null>(null);
  const [canCreate, setCanCreate] = useState(false);
  const [blueprints, setBlueprints] = useState<BlueprintOption[]>([]);
  const [sites, setSites] = useState<Site[]>([]);
  const [form, setForm] = useState<Form | null>(null);
  const [error, setError] = useState('');
  const [del, setDel] = useState<number | null>(null);
  const [menu, setMenu] = useState<{ id: number; x: number; y: number } | null>(null);

  // Situs pertama = default/origin: selalu tercentang dan tidak bisa dilepas.
  const origin = sites[0]?.handle;
  const load = () => call<{ sets: Set[]; sites: Site[]; canCreate: boolean; blueprints: BlueprintOption[] }>(`${api}/list`).then((d) => { setSets(d.sets); setCanCreate(d.canCreate); setBlueprints(d.blueprints); setSites(d.sites); }).catch((e) => toast(e.message, true));
  useEffect(() => { load(); }, []);

  const open = (f: Form) => { setError(''); setForm(f); };
  const create = () => open({ id: 0, title: '', handle: '', blueprint: '', createBlueprint: true, sites: origin ? [origin] : [], saveAction: 'listing' });
  const edit = (g: Set) => open({ id: g.id, title: g.title, handle: g.handle, blueprint: g.blueprint, createBlueprint: false, saveAction: g.saveAction, sites: [...new Set([...g.sites, ...(origin ? [origin] : [])])] });

  const save = async () => {
    if (!form) return;
    try {
      const r = await call<{ blueprintCreated: boolean }>(`${api}/save`, form);
      setForm(null);
      if (r.blueprintCreated) { visit(`${cp}/blueprints`); return; } // pesan sukses dikirim server sebagai flash
      toast('Global set saved.');
      load();
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
  };
  const remove = async () => {
    const id = del!; setDel(null);
    try { await call(`${api}/delete`, { id }); toast('Global set deleted.'); } catch (e) { toast(e instanceof Error ? e.message : String(e), true); }
    load();
  };

  const actions = (g: Set): MenuAction[] => [
    g.canEdit && { label: 'Edit values', run: () => visit(`${cp}/globals/${g.handle}`) },
    g.canEdit && { label: 'Settings', run: () => edit(g) },
    g.canDelete && { label: 'Delete', danger: true, run: () => setDel(g.id) },
  ].filter(Boolean) as MenuAction[];

  if (!sets) return null;
  return (
    <section className="w-full space-y-6" onClick={() => setMenu(null)}>
      <div className="flex flex-wrap items-center justify-between gap-3 overflow-hidden rounded-xl border border-zinc-200 px-2 py-2 md:px-4 dark:border-zinc-700">
        <div className="flex items-center gap-2">
          <svg className="h-6 w-6 text-gray-800 dark:text-white" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="24" height="24" fill="none" viewBox="0 0 24 24">
            <path stroke="currentColor" strokeLinecap="round" strokeWidth="2" d="M4.37 7.657c2.063.528 2.396 2.806 3.202 3.87 1.07 1.413 2.075 1.228 3.192 2.644 1.805 2.289 1.312 5.705 1.312 6.705M20 15h-1a4 4 0 0 0-4 4v1M8.587 3.992c0 .822.112 1.886 1.515 2.58 1.402.693 2.918.351 2.918 2.334 0 .276 0 2.008 1.972 2.008 2.026.031 2.026-1.678 2.026-2.008 0-.65.527-.9 1.177-.9H20M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
          </svg>
          <h1 className="text-2xl font-medium text-zinc-800 dark:text-white">Globals</h1>
        </div>
        {canCreate && <Button variant="primary" onClick={create}>New global set</Button>}
      </div>

      <div className="rounded-xl bg-zinc-100 p-1.5 dark:bg-zinc-900">
        <div className="rounded-xl bg-white dark:bg-zinc-800">
          <Table>
            <thead className="bg-zinc-100 dark:bg-zinc-900">
              <tr><Column>Title</Column><Column>Handle</Column><Column /></tr>
            </thead>
            <tbody>
              {sets.length === 0 && <tr><Cell colSpan={4}>No global sets yet.</Cell></tr>}
              {sets.map((g) => {
                const operate = g.canEdit || g.canDelete;
                return (
                  <tr key={g.id} onContextMenu={(e) => { if (!operate) return; e.preventDefault(); e.stopPropagation(); setMenu({ id: g.id, x: e.clientX - 20, y: e.clientY + 10 }); }}>
                    <Cell>{g.canEdit ? <a className="hover:text-orange-700" href={`${cp}/globals/${g.handle}`}>{g.title}</a> : <span>{g.title}</span>}</Cell>
                    <Cell>{g.handle}</Cell>
                    <Cell>
                      <div className="flex justify-end gap-2">
                        {operate && (
                          <Menu trigger={<Button variant="ghost" icon="ellipsis-vertical" aria-label="Actions" />}>
                            {g.canEdit && <MenuItem icon="pencil" onClick={() => visit(`${cp}/globals/${g.handle}`)}>Edit values</MenuItem>}
                            {g.canEdit && <MenuItem icon="cog-6-tooth" onClick={() => edit(g)}>Settings</MenuItem>}
                            {g.canDelete && <MenuItem icon="trash" danger onClick={() => setDel(g.id)}>Delete</MenuItem>}
                          </Menu>
                        )}
                      </div>
                    </Cell>
                  </tr>
                );
              })}
            </tbody>
          </Table>
        </div>
      </div>

      {menu && <ContextMenu x={menu.x} y={menu.y} actions={actions(sets.find((x) => x.id === menu.id)!)} close={() => setMenu(null)} />}

      <Modal open={!!form} onClose={() => setForm(null)} className="max-w-lg">
        <form className="space-y-6" onSubmit={(e) => { e.preventDefault(); save(); }}>
          <Heading className="text-lg">{form?.id ? 'Edit global set' : 'New global set'}</Heading>
          <Input small label="Title" required autoFocus value={form?.title ?? ''} onChange={(e) => setForm({ ...form!, title: e.target.value })} />
          <div className="flex items-end gap-2">
            <div className="flex-1"><Input small label="Handle" required value={form?.handle ?? ''} onChange={(e) => setForm({ ...form!, handle: e.target.value })} /></div>
            <Button icon="arrow-path" title="Regenerate from title" aria-label="Regenerate from title" onClick={() => setForm({ ...form!, handle: slug(form!.title, '_') })} />
          </div>
          {error && <p className="text-sm font-medium text-red-500 dark:text-red-400">{error}</p>}

          {form && !form.createBlueprint && (
            <>
              <Select small label="Blueprint" value={form.blueprint} onChange={(e) => setForm({ ...form, blueprint: e.target.value })}>
                <option value="">No blueprint</option>
                {blueprints.map((b) => <option key={b.handle + b.label} value={b.handle}>{b.label}</option>)}
              </Select>
              {blueprints.length === 0 && <Text>No blueprints yet — create one in the Blueprints module first.</Text>}
            </>
          )}

          {form && !form.id && <Checkbox label="Create a blueprint for this global set" description="Recommended — you'll add its fields right after saving. Turn off to pick an existing blueprint instead." checked={form.createBlueprint} onChange={(v) => setForm({ ...form, createBlueprint: v })} />}

          {form && sites.length > 0 && (
            <div className="space-y-3">
              <div><p className="text-sm font-medium text-zinc-800 dark:text-white">Languages</p><Text className="mt-1">Which sites does this global set have values for? The default site is the origin language and is always included.</Text></div>
              {sites.map((s) => (
                <Checkbox key={s.handle} label={s.name} description={s.handle === origin ? 'Default — origin language' : undefined} disabled={s.handle === origin}
                  checked={form.sites.includes(s.handle)} onChange={(v) => setForm({ ...form, sites: v ? [...form.sites, s.handle] : form.sites.filter((h) => h !== s.handle) })} />
              ))}
            </div>
          )}

          {form && (
            <div className="space-y-3">
              <div><p className="text-sm font-medium text-zinc-800 dark:text-white">Action after save</p><Text className="mt-1">Where the values form goes after saving this global set's values.</Text></div>
              <Radio label="Go to listing" checked={form.saveAction === 'listing'} onChange={() => setForm({ ...form, saveAction: 'listing' })} />
              <Radio label="Continue editing" checked={form.saveAction === 'editing'} onChange={() => setForm({ ...form, saveAction: 'editing' })} />
            </div>
          )}

          <ModalActions>
            <Button variant="filled" onClick={() => setForm(null)}>Cancel</Button>
            <Button variant="primary" type="submit">Save</Button>
          </ModalActions>
        </form>
      </Modal>

      <Modal open={del !== null} onClose={() => setDel(null)} className="min-w-[24rem]">
        <div className="space-y-4">
          <Heading className="text-lg">Delete this global set?</Heading>
          <Text>Its values will be deleted too. This cannot be undone.</Text>
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
