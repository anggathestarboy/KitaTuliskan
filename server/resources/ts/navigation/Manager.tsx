// Padanan NavigationManager + navigation-manager.blade.php. Bagian blueprint (checkbox "Create a blueprint", select Blueprint)
// belum ada karena modul Blueprints belum dibuat; kolom Blueprint tampil "—".
import { useEffect, useMemo, useState } from 'react';
import { Cell, Column, Menu, MenuItem, Table, useToasts, ContextMenu, type MenuAction } from '../shared/flux';
import { Button, Heading, Input, Modal, ModalActions, Text, Toasts } from '../shared/ui';
import { type Nav, client, slug, visit } from './api';


const tip = "Add fields to this navigation's blueprint first.";

export function Manager({ cp }: { cp: string }) {
  const api = useMemo(() => client(cp), [cp]);
  const { toasts, toast } = useToasts();
  const [navs, setNavs] = useState<Nav[] | null>(null);
  const [canCreate, setCanCreate] = useState(false);
  const [form, setForm] = useState<{ id: number; title: string; handle: string } | null>(null);
  const [error, setError] = useState('');
  const [del, setDel] = useState<number | null>(null);
  const [menu, setMenu] = useState<{ id: number; x: number; y: number } | null>(null);

  const load = () => api.list().then((d) => { setNavs(d.navigations); setCanCreate(d.canCreate); }).catch((e) => toast(e.message, true));
  useEffect(() => { load(); }, [api]);

  const fail = (e: unknown) => toast(e instanceof Error ? e.message : String(e), true);
  const save = async () => {
    if (!form) return;
    try {
      await api.save(form);
      setForm(null); setError('');
      toast('Navigation saved.');
      load();
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
  };
  const remove = async () => {
    const id = del!; setDel(null);
    try { await api.remove(id); toast('Navigation deleted.'); } catch (e) { fail(e); }
    load();
  };
  const edit = (n: Nav) => { if (n.protected) return toast('This navigation is protected and cannot be edited.', true); setError(''); setForm({ id: n.id, title: n.title, handle: n.handle }); };

  // Terproteksi: Edit/Delete tampil tapi nonaktif (seperti di source), bukan hilang.
  const menuActions = (n: Nav): MenuAction[] => [
    (n.protected || n.canEdit) && { label: 'Edit', disabled: n.protected, run: () => edit(n) },
    (n.protected || n.canDelete) && { label: 'Delete', danger: true, disabled: n.protected, run: () => setDel(n.id) },
  ].filter(Boolean) as MenuAction[];

  if (!navs) return null;
  return (
    <section className="w-full space-y-6" onClick={() => setMenu(null)}>
      <div className="flex flex-wrap items-center justify-between gap-3 overflow-hidden rounded-xl border border-zinc-200 px-2 py-2 md:px-4 dark:border-zinc-700">
        <div className="flex items-center gap-2">
          <svg className="h-6 w-6 text-gray-800 dark:text-white" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="24" height="24" fill="none" viewBox="0 0 24 24">
            <path stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M11 6.5h2M11 18h2m-7-5v-2m12 2v-2M5 8h2a1 1 0 0 0 1-1V5a1 1 0 0 0-1-1H5a1 1 0 0 0-1 1v2a1 1 0 0 0 1 1Zm0 12h2a1 1 0 0 0 1-1v-2a1 1 0 0 0-1-1H5a1 1 0 0 0-1 1v2a1 1 0 0 0 1 1Zm12 0h2a1 1 0 0 0 1-1v-2a1 1 0 0 0-1-1h-2a1 1 0 0 0-1 1v2a1 1 0 0 0 1 1Zm0-12h2a1 1 0 0 0 1-1V5a1 1 0 0 0-1-1h-2a1 1 0 0 0-1 1v2a1 1 0 0 0 1 1Z" />
          </svg>
          <h1 className="text-2xl font-medium text-zinc-800 dark:text-white">Navigation</h1>
        </div>
        {canCreate && <Button variant="primary" onClick={() => { setError(''); setForm({ id: 0, title: '', handle: '' }); }}>Add navigation</Button>}
      </div>

      <div className="rounded-xl bg-zinc-100 p-1.5 dark:bg-zinc-900">
        <div className="rounded-xl bg-white dark:bg-zinc-800">
          <Table>
            <thead className="bg-zinc-100 dark:bg-zinc-900">
              <tr><Column>Title</Column><Column>Handle</Column><Column>Blueprint</Column><Column /></tr>
            </thead>
            <tbody>
              {navs.length === 0 && (
                <tr><Cell colSpan={4}>No navigations yet. Run <code className="rounded bg-zinc-100 px-1.5 py-0.5 font-mono text-xs text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300"> make sidebar </code> to create the default admin navigation.</Cell></tr>
              )}
              {navs.map((n) => {
                const operate = n.canEdit || n.canDelete;
                const url = `${cp}/navigations/${n.handle}`;
                const noContent = !n.protected && !n.hasBlueprint; // "Edit content" butuh field blueprint
                return (
                  <tr key={n.id} onContextMenu={(e) => { if (!operate) return; e.preventDefault(); e.stopPropagation(); setMenu({ id: n.id, x: e.clientX - 20, y: e.clientY + 10 }); }}>
                    <Cell>{n.canEdit ? (noContent ? <a>{n.title} </a> : <a className="hover:text-orange-700" href={url}>{n.title} </a>) : <span>{n.title}</span>}</Cell>
                    <Cell>{n.handle}</Cell>
                    <Cell>{n.blueprint || '—'}</Cell>
                    <Cell>
                      <div className="flex justify-end gap-1">
                        {n.canEdit && (noContent
                          ? <Button variant="primary" disabled title={tip}>Edit content</Button>
                          : <Button variant="primary" onClick={() => visit(url)}>{n.protected ? 'Edit tree' : 'Edit content'}</Button>)}
                        {operate && (
                          <Menu trigger={<Button variant="ghost" icon="ellipsis-vertical" aria-label="Actions" />}>
                            {n.protected ? <MenuItem icon="pencil" disabled>Edit</MenuItem> : n.canEdit && <MenuItem icon="pencil" onClick={() => edit(n)}>Edit</MenuItem>}
                            {n.protected ? <MenuItem icon="trash" danger disabled>Delete</MenuItem> : n.canDelete && <MenuItem icon="trash" danger onClick={() => setDel(n.id)}>Delete</MenuItem>}
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

      {menu && (() => { const n = navs.find((x) => x.id === menu.id)!; return <ContextMenu x={menu.x} y={menu.y} actions={menuActions(n)} close={() => setMenu(null)} />; })()}

      <Modal open={!!form} onClose={() => setForm(null)} className="max-w-lg">
        <form className="space-y-6" onSubmit={(e) => { e.preventDefault(); save(); }}>
          <Heading>{form?.id ? 'Edit navigation' : 'Add navigation'}</Heading>
          <div className="space-y-4">
            <Input small label="Title" placeholder="Main menu" required autoFocus value={form?.title ?? ''} onChange={(e) => setForm({ ...form!, title: e.target.value })} />
            <div className="flex items-end gap-2">
              <div className="flex-1"><Input small label="Handle" placeholder="main" required value={form?.handle ?? ''} onChange={(e) => setForm({ ...form!, handle: e.target.value })} /></div>
              <Button icon="arrow-path" title="Regenerate from title" aria-label="Regenerate from title" onClick={() => setForm({ ...form!, handle: slug(form!.title) })} />
            </div>
            {error && <p className="text-sm font-medium text-red-500 dark:text-red-400">{error}</p>}
          </div>
          <ModalActions>
            <Button variant="filled" onClick={() => setForm(null)}>Cancel</Button>
            <Button variant="primary" type="submit">Save</Button>
          </ModalActions>
        </form>
      </Modal>

      <Modal open={del !== null} onClose={() => setDel(null)} className="min-w-[24rem]">
        <div className="space-y-4">
          <Heading>Delete this navigation?</Heading>
          <Text>This cannot be undone.</Text>
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
