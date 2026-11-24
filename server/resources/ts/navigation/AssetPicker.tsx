// Padanan media::asset-picker (modal "Select assets"): breadcrumb, cari, grid folder/file, strip terpilih, "Use selected".
// Upload langsung di dalam picker (alur sama dengan Media Library: >800 KB menunggu slider kompresi).
import { useEffect, useRef, useState } from 'react';
import { type Asset, type Compression, type Folder, type Listing, type Progress, client, needsCompression } from '../media/api';
import { Compress } from '../media/Compress';
import { UploadProgress } from '../media/Progress';
import { Thumbnail } from '../media/items';
import { Button, FolderIcon, Icon, Input, MiniIcon, Modal, cn } from '../shared/ui';

export interface PickerState { container: string; selected: Asset[]; max: number | null; validation: string; ok: (ids: number[], assets: Asset[]) => void }

// isAssetAllowed: subset rule file Laravel yang dipakai field assets (image, mimes, mimetypes).
const allowed = (a: Asset, rules: string) => rules.split('|').every((r) => {
  const [name, arg = ''] = r.trim().split(':');
  if (name === 'image') return a.mime_type.startsWith('image/') && a.extension !== 'svg';
  if (name === 'mimes' || name === 'extensions') return arg.split(',').includes(a.extension.toLowerCase());
  if (name === 'mimetypes') return arg.split(',').includes(a.mime_type);
  return true;
});

export function AssetPicker({ cp, state, onClose }: { cp: string; state: PickerState | null; onClose: () => void }) {
  const [picked, setPicked] = useState<Asset[]>([]);
  const [path, setPath] = useState('');
  const [search, setSearch] = useState('');
  const [list, setList] = useState<{ folders: Folder[]; files: Asset[] }>({ folders: [], files: [] });
  const [msg, setMsg] = useState('');
  const [meta, setMeta] = useState<{ compression?: Compression; canUpload: boolean }>({ canUpload: false });
  const [reloadKey, setReloadKey] = useState(0);
  const [pending, setPending] = useState<File[]>([]);
  const [busy, setBusy] = useState(false);
  const [progress, setProgress] = useState<Progress | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);

  useEffect(() => { if (state) { setPicked(state.selected); setPath(''); setSearch(''); setMsg(''); } }, [state]);
  useEffect(() => {
    if (!state) return;
    const t = setTimeout(() => {
      const q = new URLSearchParams({ container: state.container, path, search });
      fetch(`${cp}/media/api/listing?${q}`).then((r) => r.json()).then((d: Listing & { error?: string }) => (d.error ? setMsg(d.error) : (setList({ folders: d.folders ?? [], files: d.files ?? [] }), setMeta({ compression: d.compression, canUpload: !!d.can?.upload })))).catch(() => setMsg('Something went wrong.'));
    }, search ? 300 : 0);
    return () => clearTimeout(t);
  }, [state, path, search, cp, reloadKey]);

  // send = satu kali kirim; level kosong = batas default 800 KB. Kegagalan tampil di msg picker.
  const send = async (files: File[], level?: number) => {
    setProgress({ phase: 'upload', pct: 0 });
    try {
      const r = await client(cp).upload(state!.container, path, files, setProgress, level);
      setMsg(r.failures.join(' '));
    } catch (e) { setMsg(e instanceof Error ? e.message : 'Upload failed.'); }
    setProgress(null);
    setReloadKey((k) => k + 1);
  };
  const upload = async (files: File[]) => {
    if (!files.length) return;
    const big = meta.compression?.enabled ? files.filter(needsCompression) : [];
    const now = files.filter((f) => !big.includes(f));
    if (now.length) await send(now);
    if (big.length) setPending(big);
  };
  const confirmCompress = async (level: number) => {
    setBusy(true);
    await send(pending, level);
    setBusy(false);
    setPending([]);
  };

  const sel = (id: number) => picked.some((a) => a.id === id);
  const toggle = (a: Asset) => {
    if (sel(a.id)) return setPicked(picked.filter((x) => x.id !== a.id));
    if (state!.max !== null && picked.length >= state!.max) return setMsg(`You can only select up to ${state!.max}.`);
    if (state!.validation && !allowed(a, state!.validation)) return setMsg('This file type is not allowed for this field.');
    setMsg(''); setPicked([...picked, a]);
  };
  const crumbs = path ? path.split('/').map((name, i, all) => ({ name, path: all.slice(0, i + 1).join('/') })) : [];

  return (
    <Modal open={!!state} onClose={onClose} className="max-w-4xl space-y-6">
      <div className="pr-8"><h2 className="text-lg font-medium text-zinc-800 dark:text-white">Select assets</h2></div>

      {picked.length > 0 && (
        <div className="space-y-1.5 rounded-lg border border-zinc-200 p-2 dark:border-zinc-700">
          <p className="text-xs font-medium text-zinc-500">Selected — counts toward the limit even from other folders</p>
          <div className="flex flex-wrap gap-2">
            {picked.map((a) => (
              <button type="button" key={a.id} onClick={() => toggle(a)} title={a.basename} className="group flex w-16 shrink-0 flex-col gap-1">
                <div className="relative size-16 overflow-hidden rounded border-2 border-blue-500">
                  <div className="flex h-full w-full items-center justify-center overflow-hidden bg-zinc-100 dark:bg-zinc-800"><Thumbnail asset={a} /></div>
                  <span className="absolute inset-0 flex items-center justify-center bg-black/50 text-white opacity-0 group-hover:opacity-100">✕</span>
                </div>
                <span className="w-full truncate text-center text-[11px] text-zinc-500">{a.basename}</span>
              </button>
            ))}
          </div>
        </div>
      )}

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        {search === '' ? (
          <nav className="flex flex-wrap items-center gap-2 text-sm">
            <button type="button" className={cn('cursor-pointer', path === '' ? 'font-medium text-zinc-800 dark:text-white' : 'text-zinc-500 dark:text-white/70')} onClick={() => setPath('')}>Home</button>
            {crumbs.map((c) => (
              <span key={c.path} className="flex items-center gap-2">
                <MiniIcon name="chevron-right" className="size-4 text-zinc-300 dark:text-white/40" />
                <button type="button" className={cn('cursor-pointer', c.path === path ? 'font-medium text-zinc-800 dark:text-white' : 'text-zinc-500 dark:text-white/70')} onClick={() => setPath(c.path)}>{c.name}</button>
              </span>
            ))}
          </nav>
        ) : <div />}
        <Input small icon="magnifying-glass" placeholder="Search..." className="w-full sm:w-64" value={search} onChange={(e) => setSearch(e.target.value)} />
      </div>

      <div className="max-h-96 overflow-y-auto px-1">
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-7">
          {list.folders.map((f) => (
            <button type="button" key={f.path} onClick={() => { setPath(f.path); setSearch(''); }} className="group flex flex-col items-center gap-1 rounded-lg p-1">
              <div className="flex aspect-square w-full items-center justify-center rounded-lg p-2 hover:bg-zinc-100 dark:hover:bg-zinc-800"><FolderIcon className="size-full text-(--color-accent)/90 group-hover:text-(--color-accent)" /></div>
              <span className="w-full truncate text-center text-xs">{f.name}</span>
            </button>
          ))}
          {list.files.map((a) => {
            const ok = !state?.validation || allowed(a, state.validation);
            return (
              <button type="button" key={a.id} disabled={!ok} title={ok ? undefined : 'This file type is not allowed for this field.'} onClick={() => toggle(a)}
                className={cn('relative flex flex-col gap-1 rounded-lg border p-1.5 text-left', sel(a.id) ? 'border-blue-500 ring-2 ring-blue-500' : 'border-zinc-200 hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800', !ok && 'cursor-not-allowed opacity-40')}>
                <div className="flex aspect-square w-full items-center justify-center overflow-hidden rounded bg-zinc-100 dark:bg-zinc-800"><Thumbnail asset={a} /></div>
                <span className="w-full truncate text-center text-xs">{a.basename}</span>
                {sel(a.id) && <Icon name="check-circle" className="absolute top-1 right-1 size-4 rounded-full bg-white text-blue-500 dark:bg-zinc-900" />}
              </button>
            );
          })}
          {list.folders.length === 0 && list.files.length === 0 && <div className="col-span-full py-12 text-center text-zinc-400">Nothing here yet.</div>}
        </div>
      </div>

      {progress && !busy && <UploadProgress p={progress} />}
      {msg && <p className="text-sm text-red-500">{msg}</p>}

      <div className="flex items-center justify-between gap-2 border-t border-zinc-200 pt-4 dark:border-zinc-700">
        <div>
          {meta.canUpload && (
            <>
              <input ref={fileInput} type="file" multiple className="hidden" onChange={(e) => { upload([...(e.target.files ?? [])]); e.target.value = ''; }} />
              <Button icon="arrow-up-tray" disabled={!!progress} onClick={() => fileInput.current?.click()}>{progress ? 'Uploading...' : 'Upload'}</Button>
            </>
          )}
        </div>
        <div className="flex items-center gap-2">
          <span className="text-sm text-zinc-500 dark:text-white/70">{picked.length} selected</span>
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button variant="primary" onClick={() => { state!.ok(picked.map((a) => a.id), picked); onClose(); }}>Use selected</Button>
        </div>
      </div>
      {pending.length > 0 && meta.compression && <Compress files={pending} cfg={meta.compression} busy={busy} progress={progress} onCancel={() => setPending([])} onConfirm={confirmCompress} />}
    </Modal>
  );
}
