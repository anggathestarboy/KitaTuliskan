// Padanan TermManager + term-manager.blade.php: daftar term satu taxonomy (cari slug/judul, filter site, hapus).
import { useEffect, useState } from 'react';
import { ago } from '../media/api';
import { call, visit } from '../navigation/api';
import { Cell, Column, ContextMenu, Menu, MenuItem, Table, useToasts, type MenuAction } from '../shared/flux';
import { Button, Heading, Input, ModalActions, Modal, Select, Text, Toasts } from '../shared/ui';

interface Term { id: number; site: string; title: string; slug: string; updatedAt: string }
interface Data {
  taxonomy: { handle: string; title: string }; terms: Term[]; sites: { handle: string; name: string }[]; createSite: string;
  hasBlueprint: boolean; canCreate: boolean; canEdit: boolean; canDelete: boolean;
}

// = App\Support\SiteColors: titik warna per bahasa/negara di tabel.
const COLORS: Record<string, string> = {
  indonesia: 'bg-red-500', english: 'bg-blue-500', arabic: 'bg-emerald-500', mandarin: 'bg-amber-500', chinese: 'bg-amber-500', japanese: 'bg-pink-500',
  korean: 'bg-violet-500', thai: 'bg-cyan-500', vietnamese: 'bg-lime-500', malay: 'bg-indigo-500', tagalog: 'bg-orange-500', filipino: 'bg-orange-500',
  hindi: 'bg-fuchsia-500', german: 'bg-slate-500', french: 'bg-sky-500', spanish: 'bg-yellow-500', portuguese: 'bg-teal-500', dutch: 'bg-orange-600',
  russian: 'bg-rose-500', korea: 'bg-violet-500', japan: 'bg-pink-500', china: 'bg-amber-500', malaysia: 'bg-indigo-500', singapore: 'bg-red-600',
  thailand: 'bg-cyan-500', vietnam: 'bg-lime-500', philippines: 'bg-orange-500', india: 'bg-fuchsia-500',
};

export function Terms({ cp, handle }: { cp: string; handle: string }) {
  const api = `${cp}/taxonomies/api`;
  const { toasts, toast } = useToasts();
  const [d, setD] = useState<Data | null>(null);
  const [search, setSearch] = useState('');
  const [site, setSite] = useState<string | null>(null); // null = belum dipilih → site default taxonomy
  const [del, setDel] = useState<number | null>(null);
  const [menu, setMenu] = useState<{ id: number; x: number; y: number } | null>(null);

  const load = () => call<Data>(`${api}/term-list/${encodeURIComponent(handle)}`).then(setD).catch((e) => toast(e.message, true));
  useEffect(() => { load(); }, [handle]);

  const remove = async () => {
    const id = del!; setDel(null);
    try { await call(`${api}/term-delete/${encodeURIComponent(handle)}`, { id }); toast('Term deleted.'); } catch (e) { toast(e instanceof Error ? e.message : String(e), true); }
    load();
  };

  if (!d) return null;
  const filter = site ?? d.createSite;
  const q = search.toLowerCase();
  const terms = d.terms.filter((t) => (!q || t.slug.toLowerCase().includes(q) || t.title.toLowerCase().includes(q)) && (!filter || t.site === filter));
  const base = `${cp}/taxonomies/${handle}`;
  const canNew = d.hasBlueprint && d.canCreate && d.createSite !== '';
  const operate = d.canEdit || d.canDelete;
  const actions = (t: Term): MenuAction[] => [
    d.canEdit && { label: 'Edit', run: () => visit(`${base}/terms/${t.id}`) },
    d.canDelete && { label: 'Delete', danger: true, run: () => setDel(t.id) },
  ].filter(Boolean) as MenuAction[];

  return (
    <section className="w-full" onClick={() => setMenu(null)}>
      <div className="flex flex-wrap items-center justify-between gap-3 overflow-hidden rounded-xl border border-zinc-200 px-2 py-2 md:px-4 dark:border-zinc-500">
        <div className="flex items-center gap-2">
          <svg className="h-6 w-6 text-gray-800 dark:text-white" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="24" height="24" fill="none" viewBox="0 0 24 24">
            <path stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M15.583 8.445h.01M10.86 19.71l-6.573-6.63a.993.993 0 0 1 0-1.4l7.329-7.394A.98.98 0 0 1 12.31 4l5.734.007A1.968 1.968 0 0 1 20 5.983v5.5a.992.992 0 0 1-.316.727l-7.44 7.5a.974.974 0 0 1-1.384.001Z" />
          </svg>
          <h1 className="text-2xl font-medium text-zinc-800 dark:text-white">{d.taxonomy.title}</h1>
        </div>
        <Button variant="primary" disabled={!canNew} onClick={() => visit(`${base}/terms/create/${d.createSite}`)}>New term</Button>
      </div>

      {!d.hasBlueprint && <Text className="mt-3">This taxonomy has no blueprint linked yet — link one in the Taxonomies module before creating terms.</Text>}

      <div className="mt-3 mb-7 flex flex-wrap items-end gap-3">
        <div className="min-w-64 flex-1"><Input small icon="magnifying-glass" placeholder="Search by slug or title…" value={search} onChange={(e) => setSearch(e.target.value)} /></div>
        {d.sites.length > 1 && (
          <div className="w-48">
            <Select small value={filter} onChange={(e) => setSite(e.target.value)}>
              <option value="">All sites</option>
              {d.sites.map((s) => <option key={s.handle} value={s.handle}>{s.name}</option>)}
            </Select>
          </div>
        )}
      </div>

      <div className="rounded-xl bg-zinc-100 p-1.5 dark:bg-zinc-900">
        <div className="rounded-xl bg-white dark:bg-zinc-800">
          <Table>
            <thead className="bg-zinc-100 dark:bg-zinc-900">
              <tr><Column className="w-25">Site</Column><Column>Title</Column><Column>Slug</Column><Column>Updated</Column><Column className="w-32" /></tr>
            </thead>
            <tbody>
              {terms.length === 0 && <tr><Cell colSpan={5}>{search || filter ? 'No terms match your filters.' : 'No terms yet.'}</Cell></tr>}
              {terms.map((t) => (
                <tr key={t.id} onContextMenu={(e) => { if (!operate) return; e.preventDefault(); e.stopPropagation(); setMenu({ id: t.id, x: e.clientX - 20, y: e.clientY + 10 }); }}>
                  <Cell className="w-[13%]">
                    <div className="flex items-center gap-2"><span className={`size-2 rounded-full ${COLORS[t.site.toLowerCase()] ?? 'bg-gray-400'}`} /><span>{t.site}</span></div>
                  </Cell>
                  <Cell>{d.canEdit ? <a className="hover:text-orange-700" href={`${base}/terms/${t.id}`}>{t.title || '—'}</a> : <span>{t.title || '—'}</span>}</Cell>
                  <Cell>{t.slug || '—'}</Cell>
                  <Cell className="w-[10%]">{t.updatedAt ? ago(t.updatedAt) : ''}</Cell>
                  <Cell className="w-32">
                    <div className="flex justify-end gap-2">
                      {operate && (
                        <Menu trigger={<Button variant="ghost" icon="ellipsis-vertical" aria-label="Actions" />}>
                          {d.canEdit && <MenuItem icon="pencil" onClick={() => visit(`${base}/terms/${t.id}`)}>Edit</MenuItem>}
                          {d.canDelete && <MenuItem icon="trash" danger onClick={() => setDel(t.id)}>Delete</MenuItem>}
                        </Menu>
                      )}
                    </div>
                  </Cell>
                </tr>
              ))}
            </tbody>
          </Table>
        </div>
      </div>

      {menu && <ContextMenu x={menu.x} y={menu.y} actions={actions(d.terms.find((x) => x.id === menu.id)!)} close={() => setMenu(null)} />}

      <Modal open={del !== null} onClose={() => setDel(null)} className="min-w-[24rem]">
        <div className="space-y-4">
          <Heading className="text-lg">Delete this term?</Heading>
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
