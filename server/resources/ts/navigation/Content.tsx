// Padanan NavigationTreeEditor mode blueprint, tampil seperti form Globals di Draft CMS lama (global-set-form.blade.php):
// konten (tab → section → field) di kiri, panel Save/Cancel + pilihan bahasa + tab "sidebar" di kanan.
// Semua edit di memori; hanya Save (Ctrl/Cmd+S) yang menyimpan.
import { useEffect, useMemo, useRef, useState } from 'react';
import { type Asset } from '../media/api';
import { useToasts } from '../shared/flux';
import { Button, Input, Toasts, cn } from '../shared/ui';
import { AssetPicker, type PickerState } from './AssetPicker';
import { ApiError, type ContentData, type Field, client, slug as slugify, visit } from './api';
import { type Bag, type Env, FieldView, Locked, type Path, uid, visible } from './Fields';

export const put = (d: unknown, path: Path, v: unknown): unknown => {
  if (!path.length) return v;
  const [k, ...rest] = path;
  const copy: unknown = Array.isArray(d) ? [...d] : { ...(d as object) };
  (copy as Record<string | number, unknown>)[k] = put((d as Record<string | number, unknown>)?.[k], rest, v);
  return copy;
};

// Buang error path ini beserta turunannya (kuncinya memuat indeks baris, yang bergeser saat diurutkan/dihapus).
export const dropErrors = (e: Record<string, string>, path: Path) => {
  const p = path.join('.');
  return Object.fromEntries(Object.entries(e).filter(([k]) => k !== p && !k.startsWith(`${p}.`)));
};

// Setiap baris replicator butuh _uid (kunci React & editor Bard); dibuang lagi saat simpan, seperti stripRowUids.
export function withUids(fields: Field[], data: Bag): Bag {
  const out = { ...data };
  for (const f of fields) if (f.type === 'replicator') out[f.handle] = ((out[f.handle] as Bag[]) ?? []).map((r) => ({ _uid: uid(), ...withUids(f.replicator_fields ?? [], r) }));
  return out;
}
export const strip = (v: unknown): unknown => Array.isArray(v) ? v.map(strip) : v && typeof v === 'object' ? Object.fromEntries(Object.entries(v).filter(([k]) => k !== '_uid').map(([k, x]) => [k, strip(x)])) : v;

export function assetIds(fields: Field[], data: Bag, out: number[] = []): number[] {
  for (const f of fields) {
    const v = data[f.handle];
    if (f.type === 'assets' && Array.isArray(v)) out.push(...v.map(Number));
    if (f.type === 'replicator' && Array.isArray(v)) v.forEach((r) => assetIds(f.replicator_fields ?? [], r as Bag, out));
  }
  return out;
}

// scope = awalan API/URL ("navigations" | "globals" | "taxonomies"); noun = nama item di toast.
// term = mode TermForm: satu baris per bahasa (bukan satu objek per item) + judul, slug, dan pindah bahasa lewat URL term saudara.
export function Content({ cp, handle, scope = 'navigations', noun = 'Navigation', term }: { cp: string; handle: string; scope?: string; noun?: string; term?: { id: number; site: string } }) {
  const api = useMemo(() => client(cp, scope), [cp, scope]);
  const { toasts, toast } = useToasts();
  const [cd, setCd] = useState<ContentData | null>(null);
  const [data, setData] = useState<Bag>({});
  const [tab, setTab] = useState('main');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [assets, setAssets] = useState<Record<number, Asset>>({});
  const [picker, setPicker] = useState<PickerState | null>(null);
  const [loadError, setLoadError] = useState('');
  const [saving, setSaving] = useState(false);
  const [title, setTitle] = useState('');
  const [slug, setSlug] = useState('');

  const load = (site: string) => (term
    ? api.term(handle, new URLSearchParams({ id: String(term.id), site: term.site, origin: new URLSearchParams(location.search).get('origin') ?? '' }))
    : api.content(handle, site)
  ).then((d) => {
    setCd(d); setData(withUids(d.blueprint.fields, d.data)); setErrors({});
    if (d.term) { setTitle(d.term.title); setSlug(d.term.slug); }
    if (!term && d.sites.length > 1) history.replaceState(history.state, '', `?site=${encodeURIComponent(d.activeSite)}`);
  }).catch((e) => setLoadError(e.message));
  useEffect(() => { load(new URLSearchParams(location.search).get('site') ?? ''); }, [api, handle]);
  const back = term ? `${cp}/${scope}/${handle}` : `${cp}/${scope}`;

  // Asset yang dipilih perlu info (thumbnail); ambil yang belum ada di cache.
  const missing = useMemo(() => (cd ? [...new Set(assetIds(cd.blueprint.fields, data))].filter((id) => !assets[id]) : []), [cd, data, assets]);
  useEffect(() => {
    if (!missing.length) return;
    fetch(`${cp}/media/api/assets?ids=${missing.join(',')}`).then((r) => r.json()).then((d: { assets?: Asset[] }) => setAssets((a) => ({ ...a, ...Object.fromEntries((d.assets ?? []).map((x) => [x.id, x])) }))).catch(() => {});
  }, [missing.join(','), cp]);

  const save = async () => {
    if (!cd?.canEdit || saving) return;
    setSaving(true);
    try {
      if (term) {
        const r = await api.saveTerm(handle, { id: term.id, site: cd.activeSite, origin: cd.term!.origin, title, slug, data: strip(data) });
        // pesan "Term saved." dikirim server sebagai flash; tujuan menurut "Action after save" taxonomy.
        visit(r.saveAction === 'editing' ? `${back}/terms/${r.id}` : r.saveAction === 'create_another' ? `${back}/terms/create/${r.site}` : back);
        return;
      }
      await api.saveContent(handle, cd.activeSite, strip(data));
      setErrors({});
      if (cd.saveAction === 'listing') { visit(back); return; } // pesan "saved" dikirim server sebagai flash
      toast(`${noun} saved.`);
    } catch (e) {
      if (e instanceof ApiError) setErrors(e.errors);
      toast(e instanceof Error ? e.message : String(e), true);
    }
    setSaving(false);
  };
  const saveRef = useRef(save);
  saveRef.current = save;
  useEffect(() => {
    const k = (e: KeyboardEvent) => { if ((e.ctrlKey || e.metaKey) && e.key === 's') { e.preventDefault(); saveRef.current(); } };
    addEventListener('keydown', k);
    return () => removeEventListener('keydown', k);
  }, []);

  if (loadError) return <p className="text-sm font-medium text-red-500 dark:text-red-400">{loadError}</p>;
  if (!cd) return null;

  const env: Env = {
    cd, errors, assets,
    set: (path, v) => { setData((d) => put(d, path, v) as Bag); setErrors((e) => dropErrors(e, path)); },
    pickAssets: (f, path, ids) => setPicker({
      container: String(f.options ?? ''), max: f.max_items ?? null, validation: f.validation ?? '', selected: ids.map((id) => assets[id]).filter(Boolean),
      ok: (picked, got) => { setAssets((a) => ({ ...a, ...Object.fromEntries(got.map((x) => [x.id, x])) })); env.set(path, picked); },
    }),
    pickMedia: (container, single, ok) => setPicker({ container, max: single ? 1 : null, validation: '', selected: [], ok: (_, got) => ok(got) }),
  };

  const { blueprint } = cd;
  const mainTabs = blueprint.tabs.filter((t) => t.handle !== 'sidebar');
  const sidebarTab = blueprint.tabs.find((t) => t.handle === 'sidebar');
  const sidebarFields = blueprint.fields.filter((f) => f.tab === 'sidebar');
  const tabFields = blueprint.fields.filter((f) => (f.tab ?? 'main') === tab);
  const sections = (blueprint.tabs.find((t) => t.handle === tab)?.sections ?? []);

  const one = (f: Field) => visible(f, data) && (
    <div key={f.handle}>
      {!f.localizable && !cd.isOrigin
        ? <Locked f={f} value={data[f.handle]} />
        : <FieldView f={f} path={[f.handle]} value={data[f.handle]} env={env} />}
    </div>
  );
  // Bagian ber-section: kartu abu dengan judul, isi di panel putih.
  const grouped = (fields: Field[], secs: { handle: string; display: string }[]) => secs.map((sec) => {
    const inSec = fields.filter((f) => f.section === sec.handle);
    if (!inSec.length) return null;
    return (
      <div key={sec.handle} className="rounded-2xl bg-gray-200/80 px-2 py-2.5 dark:bg-zinc-950">
        <div className="px-2 font-semibold text-black"><p className="text-sm text-zinc-500 dark:text-white/70">{sec.display}</p></div>
        <div className="mt-3 space-y-6 rounded-lg bg-white p-3 dark:bg-zinc-800">{inSec.map(one)}</div>
      </div>
    );
  });
  const ungrouped = tabFields.filter((f) => (f.section ?? '') === '');

  return (
    <section key={cd.activeSite} className="max-w-8xl w-full space-y-6">
      <div className="flex items-center gap-3 rounded-xl border border-gray-200 px-3.5 py-2 dark:border-zinc-500/80">
        <div className="h-6 w-2.5 rounded-full bg-sky-500" />
        <h1 className="text-2xl font-medium text-zinc-800 dark:text-white">{term ? (term.id ? 'Edit term' : 'New term') : cd.item.title}</h1>
      </div>

      <form className="flex flex-col items-start gap-7 md:flex-row" onSubmit={(e) => { e.preventDefault(); save(); }}>
        <div className="w-full space-y-6 lg:w-[60%] xl:w-[70%]">
          {mainTabs.length > 1 && (
            <div className="flex gap-1 border-b border-zinc-200 dark:border-zinc-700">
              {mainTabs.map((t) => (
                <button key={t.handle} type="button" onClick={() => setTab(t.handle)}
                  className={cn('border-b-2 px-3 py-2 text-sm', tab === t.handle ? 'border-zinc-800 font-medium text-zinc-900 dark:border-zinc-100 dark:text-zinc-100' : 'border-transparent text-zinc-500 hover:text-zinc-700 dark:hover:text-zinc-300')}>{t.display}</button>
              ))}
            </div>
          )}
          {ungrouped.map(one)}
          {blueprint.fields.length === 0 && (
            <div className="rounded-lg border border-dashed border-zinc-300 p-6 text-center dark:border-zinc-600">
              <p className="text-sm text-zinc-500 dark:text-white/70">This blueprint has no fields yet.</p>
              <div className="mt-3"><Button onClick={() => visit(`${cp}/blueprints`)}>Add fields in Blueprints</Button></div>
            </div>
          )}
          {blueprint.fields.length > 0 && tabFields.length === 0 && <p className="text-sm text-zinc-500 dark:text-white/70">No fields in this tab.</p>}
          {grouped(tabFields, sections)}
        </div>

        <div className="h-max w-full space-y-6 rounded-xl border border-zinc-200 p-4 lg:w-[40%] xl:w-[30%] dark:border-zinc-600">
          <div className="mb-6 flex justify-end gap-2 dark:border-zinc-700">
            {cd.canEdit && <Button variant="primary" type="submit" className="w-full">Save</Button>}
            <Button variant="filled" onClick={() => visit(back)}>Cancel</Button>
          </div>

          {cd.sites.length > 1 && (
            <div className="space-y-1 rounded-xl border border-gray-300 p-2 md:col-span-1 dark:border-zinc-500">
              <div className="flex flex-col gap-1 dark:border-zinc-700">
                {cd.sites.map((s) => s.handle === cd.activeSite
                  ? <Button key={s.handle} variant="primary" disabled>{s.name}</Button>
                  : <Button key={s.handle} variant="filled" onClick={() => (s.href ? visit(s.href) : load(s.handle))}>{s.name}</Button>)}
              </div>
            </div>
          )}

          {term && <Input small label="Title" required maxLength={300} placeholder="Term title" value={title} onChange={(e) => setTitle(e.target.value)} />}

          {term && (
            <div className="flex items-end gap-2">
              <div className="min-w-0 flex-1"><Input small label="Slug" required placeholder="my-term-slug" value={slug} onChange={(e) => setSlug(e.target.value)} /></div>
              <Button icon="arrow-path" title="Regenerate from title" aria-label="Regenerate from title" onClick={() => setSlug(slugify(title))} />
            </div>
          )}

          {sidebarFields.length > 0 && (
            <div className="space-y-6 border-t border-zinc-200 pt-5 dark:border-zinc-700">
              {sidebarFields.filter((f) => (f.section ?? '') === '').map(one)}
              {grouped(sidebarFields, sidebarTab?.sections ?? [])}
            </div>
          )}
        </div>
      </form>

      <AssetPicker cp={cp} state={picker} onClose={() => setPicker(null)} />
      <Toasts toasts={toasts} />
    </section>
  );
}
