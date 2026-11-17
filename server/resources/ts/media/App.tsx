// Media Library = padanan Livewire Index + index.blade.php di source (state komponen Livewire → state React, wire:click → API Go).
import { type MouseEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { type Asset, type Item, type Listing, client } from './api';
import { ContextMenu, Entry, type Ctx, type MenuAction, Thumbnail } from './items';
import { Button, ButtonGroup, Heading, Icon, Input, MiniIcon, Modal, ModalActions, Select, Text, Toasts, cn } from './ui';

type Toast = { id: number; text: string; danger: boolean };
const parse = (s: string): Item => { const i = s.indexOf(':'); return { type: s.slice(0, i) as Item['type'], key: s.slice(i + 1) }; };
const urlOf = (c: string, p: string) => { const u = new URLSearchParams(); c && u.set('container', c); p && u.set('path', p); return u.size ? `?${u}` : location.pathname; };

export function App({ cp }: { cp: string }) {
  const api = useMemo(() => client(cp), [cp]);
  const init = new URLSearchParams(location.search);
  const [container, setContainer] = useState(init.get('container') ?? '');
  const [path, setPath] = useState(init.get('path') ?? '');
  const [search, setSearch] = useState('');
  const [q, setQ] = useState('');
  const [data, setData] = useState<Listing | null>(null);
  const [mode, setMode] = useState<'grid' | 'list'>('grid');
  const [selected, setSelected] = useState<string[]>([]);
  const [clipboard, setClipboard] = useState<{ mode: 'cut' | 'copy'; items: Item[] } | null>(null);
  const [menu, setMenu] = useState<{ id: string; x: number; y: number } | null>(null);
  const [toasts, setToasts] = useState<Toast[]>([]);
  const [tick, setTick] = useState(0);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [progress, setProgress] = useState<number | null>(null);
  const [dragging, setDragging] = useState(false);
  const [newFolder, setNewFolder] = useState<string | null>(null);
  const [renFolder, setRenFolder] = useState<{ path: string; value: string } | null>(null);
  const [renFile, setRenFile] = useState<{ id: number; value: string; ext: string } | null>(null);
  const [delFolder, setDelFolder] = useState<{ path: string; hasContents: boolean } | null>(null);
  const [delFile, setDelFile] = useState<number | null>(null);
  const [bulkDelete, setBulkDelete] = useState(false);
  const [preview, setPreview] = useState<{ asset: Asset; title: string; alt: string; loading: 'lazy' | 'eager' } | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);

  const cur = data?.container ?? '';
  const can = data?.can;
  const reload = () => setTick((t) => t + 1);
  const toast = useCallback((text: string, danger = false) => {
    const id = Date.now() + Math.random();
    setToasts((t) => [...t, { id, text, danger }]);
    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), 4000);
  }, []);
  const fail = (e: unknown) => toast(e instanceof Error ? e.message : String(e), true);
  // run = try/catch + toast + reload; true kalau berhasil (untuk menutup modal).
  const run = async (fn: () => Promise<unknown>, ok?: string) => {
    try { await fn(); ok && toast(ok); return true; } catch (e) { fail(e); return false; } finally { reload(); }
  };

  useEffect(() => { const t = setTimeout(() => { setQ(search); setSelected([]); }, 400); return () => clearTimeout(t); }, [search]);
  useEffect(() => {
    const ac = new AbortController();
    api.listing(container, path, q, ac.signal).then((d) => { setData(d); setLoadError(null); }).catch((e) => { if (e.name !== 'AbortError') { setLoadError(e.message); fail(e); } });
    return () => ac.abort();
  }, [api, container, path, q, tick]);
  useEffect(() => { data && history.replaceState(history.state, '', urlOf(data.container, data.path)); }, [data?.container, data?.path]);

  const goTo = (p: string) => { setPath(p); setSearch(''); setQ(''); setSelected([]); };
  const items = selected.map(parse);
  const toggle = (type: 'file' | 'folder', key: string) => setSelected((s) => (s.includes(`${type}:${key}`) ? s.filter((x) => x !== `${type}:${key}`) : [...s, `${type}:${key}`]));
  const allowed = (file: boolean | undefined, folder: boolean | undefined) => items.length > 0 && items.every((i) => (i.type === 'folder' ? folder : file));
  const canPaste = !!clipboard && !!cur && !!can && (clipboard.mode === 'cut'
    ? can.move
    : (!clipboard.items.some((i) => i.type === 'file') || can.edit) && (!clipboard.items.some((i) => i.type === 'folder') || can.editFolders));

  const stash = (m: 'cut' | 'copy', list: Item[]) => { setClipboard({ mode: m, items: list }); setSelected([]); toast(`${m === 'cut' ? 'Cut' : 'Copied'}. Navigate to a destination folder and paste.`); };
  const duplicate = (list: Item[]) => run(() => api.post('duplicate', cur, { items: list }), 'Duplicated.');
  const remove = (list: Item[], msg = 'Deleted.') => run(() => api.post('delete', cur, { items: list }), msg);
  const download = (type: Item['type'], key: string) => { location.href = api.downloadUrl(cur, type, key); };
  const view = (a: Asset) => setPreview({ asset: a, title: a.title, alt: a.alt, loading: a.loading });
  const startRenameFile = (a: Asset) => setRenFile({ id: a.id, value: a.filename, ext: a.extension });

  const drop = async (type: string, key: string, target: string) => {
    if (type === 'folder' && (key === target || target.startsWith(key + '/'))) return;
    try {
      const r = await api.post<{ moved: number }>('paste', cur, { mode: 'cut', items: [{ type, key }], to: target });
      r.moved && toast('Moved.');
    } catch (e) { fail(e); }
    reload();
  };

  const upload = async (files: File[]) => {
    if (!files.length || !can?.upload) return;
    setProgress(0);
    try {
      const r = await api.upload(cur, data!.path, files, setProgress);
      r.failures.length ? toast(r.failures.join(' '), true) : toast('Upload complete.');
    } catch (e) { fail(e); }
    setProgress(null);
    reload();
  };

  const ctx: Ctx = {
    can: can!, selected, toggle, goTo, view, drop,
    menu: (id: string, e: MouseEvent, tgl = false) => setMenu((m) => (tgl && m?.id === id ? null : { id, x: e.clientX - 20, y: e.clientY + 10 })),
  };

  const menuActions = (id: string): MenuAction[] => {
    if (!can || !data) return [];
    const [kind, ...rest] = id.split('-'), key = rest.join('-');
    if (kind === 'folder') {
      const it: Item = { type: 'folder', key };
      return [
        can.editFolders && { label: 'Rename', run: () => setRenFolder({ path: key, value: key.split('/').pop()! }) },
        can.editFolders && { label: 'Duplicate', run: () => duplicate([it]) },
        can.move && { label: 'Cut', run: () => stash('cut', [it]) },
        { label: 'Download', run: () => download('folder', key) },
        can.editFolders && { label: 'Delete', danger: true, run: () => askDeleteFolder(key) },
      ].filter(Boolean) as MenuAction[];
    }
    const a = data.files.find((f) => String(f.id) === key);
    if (!a) return [];
    const it: Item = { type: 'file', key };
    return [
      { label: 'View', run: () => view(a) },
      { label: 'Copy URL', run: () => navigator.clipboard.writeText(a.copy_url) },
      can.rename && { label: 'Rename', run: () => startRenameFile(a) },
      can.edit && { label: 'Duplicate', run: () => duplicate([it]) },
      can.move && { label: 'Cut', run: () => stash('cut', [it]) },
      { label: 'Download', run: () => download('file', key) },
      can.delete && { label: 'Delete', danger: true, run: () => setDelFile(a.id) },
    ].filter(Boolean) as MenuAction[];
  };

  const askDeleteFolder = async (p: string) => {
    try {
      const l = await api.listing(cur, p, '');
      setDelFolder({ path: p, hasContents: l.folders.length > 0 || l.files.length > 0 });
    } catch (e) { fail(e); }
  };

  const savePreview = async () => {
    if (!preview) return;
    const { asset, title, alt, loading } = preview;
    if (await run(() => api.post('meta', cur, { id: String(asset.id), title, alt, loading }), 'Saved.')) setPreview(null);
  };

  if (!data) {
    // Jangan blank: tampilkan penyebab (mis. server Go masih versi lama yang belum punya /media/api).
    return loadError ? <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-200">Media Library failed to load: {loadError}. If you just updated the code, restart the Go server (make stop, make dev).</div> : null;
  }
  const crumbs = data.path ? data.path.split('/').map((name, i, a) => ({ name, path: a.slice(0, i + 1).join('/') })) : [];
  const empty = data.folders.length === 0 && data.files.length === 0;
  const rows = [
    ...data.folders.map((f) => <Entry key={`folder-${f.path}`} mode={mode} ctx={ctx} row={{ type: 'folder', name: f.name, path: f.path }} />),
    ...data.files.map((f) => <Entry key={`file-${f.id}`} mode={mode} ctx={ctx} row={{ type: 'file', asset: f }} />),
  ];

  return (
    <div onClick={() => setMenu(null)}>
      <div className="flex flex-col gap-4 rounded-xl px-4 py-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2">
          <svg className="h-6 w-6 text-gray-800 dark:text-white" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="24" height="24" fill="none" viewBox="0 0 24 24">
            <path stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="m3 16 5-7 6 6.5m6.5 2.5L16 13l-4.286 6M14 10h.01M4 19h16a1 1 0 0 0 1-1V6a1 1 0 0 0-1-1H4a1 1 0 0 0-1 1v12a1 1 0 0 0 1 1Z" />
          </svg>
          <h1 className="text-2xl font-medium text-zinc-800 dark:text-white">Media Library</h1>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <ButtonGroup>
            <Button variant={mode === 'grid' ? 'primary' : 'ghost'} icon="squares-2x2" aria-label="Grid" onClick={() => setMode('grid')} />
            <Button variant={mode === 'list' ? 'primary' : 'ghost'} icon="list-bullet" aria-label="List" onClick={() => setMode('list')} />
          </ButtonGroup>
          {data.containers.length > 0 && <>
            {canPaste && <Button variant="filled" icon="clipboard-document" onClick={() => paste()}>Paste</Button>}
            {can?.editFolders && <Button icon="folder-plus" onClick={() => setNewFolder('')}>New Folder</Button>}
            {can?.upload && <Button variant="primary" icon="arrow-up-tray" onClick={() => fileInput.current?.click()}>Upload</Button>}
          </>}
        </div>
      </div>

      <div className="mt-3 mb-7">
        {data.containers.length > 0 && (
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div className="w-full sm:w-72"><Input small icon="magnifying-glass" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search files & folders" /></div>
            <div className="w-full sm:w-56"><Select small className="cursor-pointer" value={cur} onChange={(e) => { setContainer(e.target.value); setPath(''); setSearch(''); setQ(''); setSelected([]); }}>
              {data.containers.map((c) => <option key={c.handle} value={c.handle}>{c.title}</option>)}
            </Select></div>
          </div>
        )}
      </div>

      {progress !== null && (
        <div className="my-3 space-y-3">
          <div className="flex items-center justify-between text-xs text-zinc-500"><span>Uploading...</span><span>{progress}%</span></div>
          <div className="h-1.5 w-full overflow-hidden rounded-full bg-zinc-200 dark:bg-white/10"><div className="h-full rounded-full bg-blue-500 transition-[width] duration-300 ease-out" style={{ width: `${progress}%` }} /></div>
        </div>
      )}

      <div className="my-3 space-y-3">
        {data.containers.length === 0 ? (
          <div className="rounded-lg border border-dashed border-zinc-300 p-12 text-center text-zinc-400 dark:border-zinc-700">
            No media containers yet. Run <code className="rounded bg-zinc-100 px-1.5 py-0.5 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300">make media</code> to create one.
          </div>
        ) : (
          <div>
            <div className="overflow-x-auto rounded-t-lg bg-zinc-100 px-4 py-2 dark:bg-zinc-800">
              <div className="flex w-max">
                {[{ name: 'Home', path: '' }, ...crumbs].map((c, i, all) => (
                  <div key={c.path} className="group/breadcrumb flex items-center text-sm font-medium">
                    <div onClick={() => goTo(c.path)} className="cursor-pointer text-gray-500 dark:text-white/80">{c.name}</div>
                    {i < all.length - 1 && <MiniIcon name="chevron-right" className="mx-1 size-5 text-zinc-300 dark:text-white/80" />}
                  </div>
                ))}
              </div>
            </div>
            <div
              className={cn('rounded-b-lg border border-t-0 border-dashed border-zinc-300 p-6 dark:border-zinc-700', dragging && 'border-blue-500 bg-blue-50 dark:bg-blue-950')}
              onDragOver={(e) => { if (can?.upload && e.dataTransfer.types.includes('Files')) { e.preventDefault(); setDragging(true); } }}
              onDragLeave={() => setDragging(false)}
              onDrop={(e) => { setDragging(false); if (can?.upload && e.dataTransfer.files.length) { e.preventDefault(); upload([...e.dataTransfer.files]); } }}
            >
              {mode === 'grid' ? (
                <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 xl:grid-cols-7 2xl:grid-cols-9">
                  {rows}
                  {empty && <div className="col-span-full py-12 text-center text-zinc-400">This folder is empty.</div>}
                </div>
              ) : (
                <div className="space-y-1">
                  {!empty && (
                    <div className="flex items-center gap-3 border-b border-zinc-200 px-2 pb-2 text-xs font-medium text-zinc-400 dark:border-zinc-700">
                      <span className="size-8 shrink-0" /><span className="min-w-0 flex-1">Name</span>
                      <span className="hidden w-20 shrink-0 text-right sm:block">Size</span>
                      <span className="hidden w-36 shrink-0 text-right md:block">Last Modified</span>
                      <span className="w-8 shrink-0" />
                    </div>
                  )}
                  {rows}
                  {empty && <div className="py-12 text-center text-zinc-400">This folder is empty.</div>}
                </div>
              )}
            </div>
          </div>
        )}

        <input ref={fileInput} type="file" multiple className="hidden" onChange={(e) => { upload([...(e.target.files ?? [])]); e.target.value = ''; }} />

        {selected.length > 0 && (
          <div className="fixed inset-x-0 bottom-4 z-20 flex justify-center px-4 sm:bottom-6">
            <div className="flex max-w-full flex-wrap items-center justify-center gap-2 rounded-2xl border border-zinc-200 bg-white px-4 py-2 shadow-lg dark:border-zinc-700 dark:bg-zinc-900">
              <Text>{selected.length} selected</Text>
              {selected.length === 1 && allowed(can?.rename, can?.editFolders) && <Button icon="pencil" onClick={() => renameSelected()}>Rename</Button>}
              {allowed(can?.edit, can?.editFolders) && <>
                <Button icon="clipboard-document" onClick={() => stash('copy', items)}>Copy</Button>
                <Button icon="document-duplicate" onClick={() => { duplicate(items); setSelected([]); }}>Duplicate</Button>
              </>}
              {allowed(can?.move, can?.move) && <Button icon="scissors" onClick={() => stash('cut', items)}>Cut</Button>}
              {allowed(can?.delete, can?.editFolders) && <Button variant="danger" icon="trash" onClick={() => setBulkDelete(true)}>Delete</Button>}
              <Button variant="ghost" icon="x-mark" aria-label="Clear" onClick={() => setSelected([])} />
            </div>
          </div>
        )}
      </div>

      {menu && <ContextMenu x={menu.x} y={menu.y} actions={menuActions(menu.id)} close={() => setMenu(null)} />}

      <Modal open={newFolder !== null} onClose={() => setNewFolder(null)}>
        <div className="space-y-6">
          <Heading>New Folder</Heading>
          <Input autoFocus label="Folder name" value={newFolder ?? ''} onChange={(e) => setNewFolder(e.target.value)} onEnter={createFolder} />
          <ModalActions><Button variant="ghost" onClick={() => setNewFolder(null)}>Cancel</Button><Button variant="primary" onClick={createFolder}>Create</Button></ModalActions>
        </div>
      </Modal>

      <Modal open={!!renFolder} onClose={() => setRenFolder(null)}>
        <div className="space-y-6">
          <Heading>Rename Folder</Heading>
          <Input autoFocus label="Folder name" value={renFolder?.value ?? ''} onChange={(e) => setRenFolder({ ...renFolder!, value: e.target.value })} onEnter={renameFolder} />
          <ModalActions><Button variant="ghost" onClick={() => setRenFolder(null)}>Cancel</Button><Button variant="primary" onClick={renameFolder}>Rename</Button></ModalActions>
        </div>
      </Modal>

      <Modal open={!!renFile} onClose={() => setRenFile(null)}>
        <div className="space-y-6">
          <Heading>Rename File</Heading>
          <Input autoFocus label="File name" description={renFile?.ext ? `.${renFile.ext} will be kept automatically` : undefined} value={renFile?.value ?? ''} onChange={(e) => setRenFile({ ...renFile!, value: e.target.value })} onEnter={renameFile} />
          <ModalActions><Button variant="ghost" onClick={() => setRenFile(null)}>Cancel</Button><Button variant="primary" onClick={renameFile}>Rename</Button></ModalActions>
        </div>
      </Modal>

      <Modal open={!!delFolder} onClose={() => setDelFolder(null)}>
        <div className="space-y-6">
          <Heading>Delete Folder</Heading>
          <Text>{delFolder?.hasContents ? 'This folder is not empty. Deleting it will permanently remove everything inside it. This cannot be undone.' : 'This folder will be permanently deleted. This cannot be undone.'}</Text>
          <ModalActions><Button variant="ghost" onClick={() => setDelFolder(null)}>Cancel</Button><Button variant="danger" onClick={async () => { const p = delFolder!.path; setDelFolder(null); await remove([{ type: 'folder', key: p }], 'Folder deleted.'); }}>Delete</Button></ModalActions>
        </div>
      </Modal>

      <Modal open={delFile !== null} onClose={() => setDelFile(null)}>
        <Heading>Delete File</Heading>
        <Text className="mt-3">This file will be permanently deleted. This cannot be undone.</Text>
        <div className="mt-6"><ModalActions><Button variant="ghost" onClick={() => setDelFile(null)}>Cancel</Button><Button variant="danger" onClick={async () => { const id = delFile!; setDelFile(null); await remove([{ type: 'file', key: String(id) }]); }}>Delete</Button></ModalActions></div>
      </Modal>

      <Modal open={bulkDelete} onClose={() => setBulkDelete(false)}>
        <div className="space-y-6">
          <Heading>Delete Selected</Heading>
          <Text>This will permanently delete the selected items, including the contents of any selected folders. This cannot be undone.</Text>
          <ModalActions><Button variant="ghost" onClick={() => setBulkDelete(false)}>Cancel</Button><Button variant="danger" onClick={async () => { setBulkDelete(false); await remove(items); setSelected([]); }}>Delete</Button></ModalActions>
        </div>
      </Modal>

      <Modal open={!!preview} onClose={() => setPreview(null)} className="w-[60%] max-w-none xl:w-[40%]">
        {preview && (
          <div className="mt-7 grid grid-cols-1 gap-6" onKeyDown={(e) => { if ((e.ctrlKey || e.metaKey) && e.key === 's') { e.preventDefault(); can?.edit && savePreview(); } }}>
            <div className="flex aspect-square h-[300px] w-full items-center justify-center rounded-lg bg-zinc-100 p-3 dark:bg-zinc-800"><Thumbnail asset={preview.asset} fit="object-contain" interactive /></div>
            <div className="min-w-0 space-y-4">
              <Heading className="pr-8 break-words">{preview.asset.basename}</Heading>
              {(preview.asset.extension === 'pdf' || preview.asset.mime_type.startsWith('video/')) && (
                <a href={preview.asset.url} target="_blank" rel="noreferrer" className="inline-flex h-8 items-center gap-2 rounded-lg border border-zinc-200 bg-white px-3 text-sm font-medium text-zinc-800 shadow-xs hover:bg-zinc-50 dark:border-zinc-600 dark:bg-zinc-700 dark:text-white dark:hover:bg-zinc-600">
                  <Icon name="arrow-top-right-on-square" />Open in new tab
                </a>
              )}
              {can?.edit ? <>
                <Input label="Title" value={preview.title} onChange={(e) => setPreview({ ...preview, title: e.target.value })} onEnter={savePreview} />
                <Input label="Alt text" value={preview.alt} onChange={(e) => setPreview({ ...preview, alt: e.target.value })} onEnter={savePreview} />
                <Select label="Loading" value={preview.loading} onChange={(e) => setPreview({ ...preview, loading: e.target.value as 'lazy' | 'eager' })}>
                  <option value="lazy">Lazy</option><option value="eager">Eager</option>
                </Select>
                <ModalActions><Button variant="ghost" onClick={() => setPreview(null)}>Close</Button><Button variant="primary" onClick={savePreview}>Save</Button></ModalActions>
              </> : <ModalActions><Button variant="ghost" onClick={() => setPreview(null)}>Close</Button></ModalActions>}
            </div>
          </div>
        )}
      </Modal>

      <Toasts toasts={toasts} />
    </div>
  );

  // --- aksi yang butuh closure atas state di atas ---
  function createFolder() {
    const name = (newFolder ?? '').trim();
    if (!name) return toast('The new folder name field is required.', true);
    run(() => api.post('folder', cur, { op: 'create', path: data!.path, name }), 'Folder created.').then((ok) => ok && setNewFolder(null));
  }
  function renameFolder() {
    if (!renFolder?.value.trim()) return toast('The rename folder value field is required.', true);
    run(() => api.post('folder', cur, { op: 'rename', path: renFolder.path, name: renFolder.value }), 'Folder renamed.').then((ok) => { if (ok) { setRenFolder(null); setSelected([]); } });
  }
  function renameFile() {
    if (!renFile?.value.trim()) return toast('The rename file value field is required.', true);
    run(() => api.post('rename', cur, { id: String(renFile.id), name: renFile.value }), 'File renamed.').then((ok) => { if (ok) { setRenFile(null); setSelected([]); } });
  }
  function renameSelected() {
    const it = items[0], a = it.type === 'file' ? data!.files.find((f) => String(f.id) === it.key) : undefined;
    if (a) startRenameFile(a);
    else setRenFolder({ path: it.key, value: it.key.split('/').pop()! });
  }
  function paste() {
    if (!clipboard) return;
    run(() => api.post('paste', cur, { mode: clipboard.mode, items: clipboard.items, to: data!.path }), 'Done.').then((ok) => ok && setClipboard(null));
  }
}
