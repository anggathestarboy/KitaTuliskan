// Padanan EntryForm + entry-form.blade.php (+ partials/sidebar-fields): konten (tab → section → field, SEO, Schema) di kiri,
// panel Save/Cancel + Visit + bahasa + Title/Slug/Parent/Order/Date/Status + tab "sidebar" blueprint di kanan.
// Semua edit di memori; hanya Save (Ctrl/Cmd+S) yang menyimpan. Live Preview (render template Blade) ada di Astro, bukan di sini.
import { useEffect, useMemo, useRef, useState } from 'react';
import { type Asset } from '../media/api';
import { AssetPicker, type PickerState } from '../navigation/AssetPicker';
import { ApiError, type ContentData, type Field, call, slug as slugify, visit } from '../navigation/api';
import { assetIds, put, strip, withUids } from '../navigation/Content';
import { type Bag, type Env, FieldView, Locked, visible } from '../navigation/Fields';
import { Radio, useToasts } from '../shared/flux';
import { Button, Icon, Input, Select, Text, Toasts, cn } from '../shared/ui';
import { FL } from './Field';
import { type Block, SchemaTab } from './Schema';
import { type Seo, type SeoOptions, SeoTab } from './Seo';

interface EntryData extends ContentData {
  collection: { handle: string; title: string; structured: boolean; visibleOrder: boolean; livePreview: boolean; schemaEnabled: boolean; seoEnabled: boolean; saveAction: string };
  entry: { id: string; title: string; slug: string; status: string; date: string; order: number | null; parentId: string | null; origin: string | null; blueprint: string; url?: string; previewUrl?: string };
  statuses: Record<string, string>; canEditStatus: boolean; parents: { id: string; title: string }[];
  seo: Seo; schemaBlocks: Block[]; schemaTypes: string[]; seoOptions: SeoOptions;
}

const BAR: Record<string, string> = { published: 'bg-emerald-500', scheduled: 'bg-blue-500', need_review: 'bg-amber-500', draft: 'bg-gray-500' };
// datetime-local tanpa detik (setEntryDatePreset): "Yesterday" / "Now" / "Tomorrow".
const stamp = (days: number) => { const d = new Date(Date.now() + days * 864e5); return new Date(d.getTime() - d.getTimezoneOffset() * 6e4).toISOString().slice(0, 16); };

export function EntryForm({ cp, handle, id, site }: { cp: string; handle: string; id: string; site: string }) {
  const base = `${cp}/collections`;
  const { toasts, toast } = useToasts();
  const [cd, setCd] = useState<EntryData | null>(null);
  const [data, setData] = useState<Bag>({});
  const [tab, setTab] = useState(new URLSearchParams(location.search).get('q') ?? 'main');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [messages, setMessages] = useState<string[]>([]);
  const [assets, setAssets] = useState<Record<number, Asset>>({});
  const [picker, setPicker] = useState<PickerState | null>(null);
  const [loadError, setLoadError] = useState('');
  const [saving, setSaving] = useState(false);
  const [title, setTitle] = useState('');
  const [slug, setSlug] = useState('');
  const [status, setStatus] = useState('draft');
  const [date, setDate] = useState('');
  const [order, setOrder] = useState('');
  const [parentId, setParentId] = useState('');
  const [seo, setSeo] = useState<Seo | null>(null);
  const [blocks, setBlocks] = useState<Block[]>([]);

  useEffect(() => {
    const q = new URLSearchParams(location.search);
    const url = id ? `${base}/api/entry/${encodeURIComponent(handle)}?id=${encodeURIComponent(id)}`
      : `${base}/api/entry/${encodeURIComponent(handle)}?${new URLSearchParams({ site, origin: q.get('origin') ?? '', blueprint: q.get('blueprint') ?? '' })}`;
    call<EntryData>(url).then((d) => {
      setCd(d); setData(withUids(d.blueprint.fields, d.data)); setSeo(d.seo); setBlocks(d.schemaBlocks ?? []);
      setTitle(d.entry.title); setSlug(d.entry.slug); setStatus(d.entry.status); setDate(d.entry.date); setOrder(d.entry.order === null ? '' : String(d.entry.order)); setParentId(d.entry.parentId ?? '');
    }).catch((e) => setLoadError(e.message));
  }, [handle, id, site]);

  // Tab aktif disimpan di ?q= seperti #[Url('q')].
  const goTab = (t: string) => { setTab(t); const q = new URLSearchParams(location.search); q.set('q', t); history.replaceState(history.state, '', `?${q}`); };

  // Asset yang dipilih (field assets + gambar SEO) perlu info/thumbnail; ambil yang belum ada di cache.
  const missing = useMemo(() => (cd ? [...new Set([...assetIds(cd.blueprint.fields, data), ...(seo?.meta_featured_image ?? []).map(Number)])].filter((x) => !assets[x]) : []), [cd, data, seo, assets]);
  useEffect(() => {
    if (!missing.length) return;
    fetch(`${cp}/media/api/assets?ids=${missing.join(',')}`).then((r) => r.json()).then((d: { assets?: Asset[] }) => setAssets((a) => ({ ...a, ...Object.fromEntries((d.assets ?? []).map((x) => [x.id, x])) }))).catch(() => {});
  }, [missing.join(','), cp]);

  const save = async () => {
    if (!cd || saving) return;
    setSaving(true);
    try {
      const r = await call<{ id: string; site: string; blueprint: string; saveAction: string }>(`${base}/api/entry/${encodeURIComponent(handle)}`, {
        id, site: cd.activeSite, origin: cd.entry.origin ?? '', blueprint: cd.entry.blueprint, title, slug, status, date, order: order === '' ? null : Number(order), parentId: parentId || null,
        data: strip(data), ...(cd.collection.seoEnabled ? { seo } : {}), ...(cd.collection.schemaEnabled ? { schemaBlocks: blocks } : {}),
      });
      // pesan "Entry saved." dikirim server sebagai flash; tujuan menurut "Action after save" collection.
      visit(r.saveAction === 'editing' ? `${base}/${handle}/entries/${r.id}` : r.saveAction === 'create_another' ? `${base}/${handle}/entries/create/${r.site}?blueprint=${encodeURIComponent(r.blueprint)}` : `${base}/${handle}`);
      return;
    } catch (e) {
      const errs = e instanceof ApiError ? e.errors : {};
      setErrors(errs);
      setMessages(Object.keys(errs).length ? [...new Set(Object.values(errs))] : [e instanceof Error ? e.message : String(e)]);
      scrollTo({ top: 0, behavior: 'smooth' });
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
  if (!cd || !seo) return null;

  const { collection: col, blueprint } = cd;
  const clear = (k: string) => setErrors((e) => { const { [k]: _, ...rest } = e; return rest; });
  const env: Env = {
    cd, errors, assets,
    set: (path, v) => { setData((d) => put(d, path, v) as Bag); clear(path.join('.')); },
    pickAssets: (f, path, ids) => setPicker({
      container: String(f.options ?? ''), max: f.max_items ?? null, validation: f.validation ?? '', selected: ids.map((x) => assets[x]).filter(Boolean),
      ok: (picked, got) => { setAssets((a) => ({ ...a, ...Object.fromEntries(got.map((x) => [x.id, x])) })); env.set(path, picked); },
    }),
    pickMedia: (container, single, ok) => setPicker({ container, max: single ? 1 : null, validation: '', selected: [], ok: (_, got) => ok(got) }),
  };
  const pickSeoImage = () => setPicker({
    container: cd.bardContainer, max: 1, validation: 'image', selected: [],
    ok: (picked, got) => { setAssets((a) => ({ ...a, ...Object.fromEntries(got.map((x) => [x.id, x])) })); setSeo((s) => ({ ...s!, meta_featured_image: picked })); clear('seo.meta_featured_image'); },
  });
  const pickLogo = (i: number) => setPicker({ container: cd.bardContainer, max: 1, validation: 'image', selected: [], ok: (_, got) => got[0] && setBlocks((bs) => bs.map((b, x) => (x === i ? { ...b, logo: got[0].url } : b))) });
  const field = <T,>(k: string, set: (v: T) => void) => (v: T) => { set(v); clear(k); };

  const mainTabs = blueprint.tabs.filter((t) => t.handle !== 'sidebar');
  const sidebarTab = blueprint.tabs.find((t) => t.handle === 'sidebar');
  const sidebarFields = blueprint.fields.filter((f) => f.tab === 'sidebar');
  const tabFields = blueprint.fields.filter((f) => (f.tab ?? 'main') === tab);
  const sections = blueprint.tabs.find((t) => t.handle === tab)?.sections ?? [];
  const tabs = [...mainTabs.map((t) => ({ handle: t.handle, display: t.display })), ...(col.seoEnabled ? [{ handle: 'seo', display: 'SEO' }] : []), ...(col.schemaEnabled ? [{ handle: 'schema', display: 'Schema' }] : [])];

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
  const url = cd.entry.url;

  return (
    <section key={cd.activeSite} className="max-w-8xl w-full space-y-6">
      <div className="flex items-center gap-3 rounded-xl border border-gray-200 px-3.5 py-2 dark:border-zinc-500/80">
        {BAR[cd.entry.status] && <div className={cn('h-6 w-2.5 rounded-full', BAR[cd.entry.status])} />}
        <h1 className="text-2xl font-medium text-zinc-800 dark:text-white">{id ? cd.entry.title : 'New Entry'}</h1>
      </div>

      {messages.length > 0 && (
        <div className="flex gap-3 rounded-lg border border-red-200 bg-red-50 p-4 dark:border-red-400/30 dark:bg-red-400/20">
          <Icon name="exclamation-triangle" className="mt-0.5 size-5 text-red-500 dark:text-red-400" />
          <div>
            <p className="text-sm font-medium text-red-800 dark:text-red-200">Please fix the following before saving:</p>
            <ul className="mt-2 list-disc space-y-1 pl-4 text-sm text-red-800 dark:text-red-200">{messages.map((m) => <li key={m}>{m}</li>)}</ul>
          </div>
        </div>
      )}

      <form className="flex flex-col items-start gap-7 md:flex-row" onSubmit={(e) => { e.preventDefault(); save(); }}>
        <div className="w-full space-y-6 lg:w-[60%] xl:w-[70%]">
          {(mainTabs.length > 1 || col.schemaEnabled || col.seoEnabled) && (
            <div className="flex gap-1 border-b border-zinc-200 dark:border-zinc-700">
              {tabs.map((t) => (
                <button key={t.handle} type="button" onClick={() => goTab(t.handle)}
                  className={cn('border-b-2 px-3 py-2 text-sm', tab === t.handle ? 'border-zinc-800 font-medium text-zinc-900 dark:border-zinc-100 dark:text-zinc-100' : 'border-transparent text-zinc-500 hover:text-zinc-700 dark:hover:text-zinc-300')}>{t.display}</button>
              ))}
            </div>
          )}

          {col.schemaEnabled && tab === 'schema'
            ? <SchemaTab blocks={blocks} setBlocks={setBlocks} types={cd.schemaTypes} errors={errors} canPick={!!cd.bardContainer} pickLogo={pickLogo} />
            : col.seoEnabled && tab === 'seo'
              ? <SeoTab seo={seo} set={(p) => { setSeo({ ...seo, ...p }); Object.keys(p).forEach((k) => clear(`seo.${k}`)); }} errors={errors} isOrigin={cd.isOrigin} options={cd.seoOptions} authors={cd.authors} assets={assets} canPick={!!cd.bardContainer} pick={pickSeoImage} />
              : (
                <>
                  {ungrouped.map(one)}
                  {tabFields.length === 0 && <Text>{blueprint.fields.length === 0 ? 'This blueprint has no fields yet.' : 'No fields in this tab.'}</Text>}
                  {grouped(tabFields, sections)}
                </>
              )}
        </div>

        <div className="col-span-1 h-max w-full space-y-5 rounded-lg border border-zinc-200 p-4 lg:w-[40%] xl:w-[30%] dark:border-zinc-700">
          <div>
            <div className="flex justify-between gap-2 pb-2">
              <Button variant="primary" type="submit" className="w-full">Save</Button>
              <Button variant="filled" onClick={() => visit(`${base}/${handle}`)}>Cancel</Button>
            </div>
            {((col.livePreview && cd.entry.previewUrl) || url) && (
              <div className="flex gap-2">
                {col.livePreview && cd.entry.previewUrl && <Button icon="eye" className="w-full" onClick={() => window.open(cd.entry.previewUrl, '_blank')}>Live Preview</Button>}
                {url && <Button icon="arrow-top-right-on-square" className="w-full" disabled={cd.entry.status !== 'published'} onClick={() => window.open(url, '_blank')}>Visit</Button>}
              </div>
            )}
          </div>

          {cd.sites.length > 1 && (
            <div className="rounded-xl border border-gray-300 p-2 dark:border-zinc-500">
              <div className="mt-1 flex flex-col gap-1">
                {cd.sites.map((s) => s.handle === cd.activeSite
                  ? <Button key={s.handle} variant="primary" disabled>{s.name}</Button>
                  : <Button key={s.handle} variant="filled" onClick={() => s.href && visit(s.href)}>{s.name}</Button>)}
              </div>
            </div>
          )}

          <FL label="Title" required err={errors.title}>
            <Input small placeholder="Entry title" maxLength={300} required value={title} onChange={(e) => field('title', setTitle)(e.target.value)} />
          </FL>
          <FL label="Slug" required err={errors.slug}>
            <div className="grid grid-cols-[1fr_auto] items-end gap-2">
              <Input small className="w-full min-w-0" placeholder="enter-slug" maxLength={255} required value={slug} onChange={(e) => field('slug', setSlug)(e.target.value)} />
              <Button icon="arrow-path" title="Regenerate from title" aria-label="Regenerate from title" onClick={() => setSlug(slugify(title))} />
            </div>
          </FL>

          {col.structured && (
            <FL label="Parent" err={errors.parentId}>
              <Select small value={parentId} onChange={(e) => field('parentId', setParentId)(e.target.value)}>
                <option value="">None — top level</option>
                {cd.parents.map((p) => <option key={p.id} value={p.id}>{p.title}</option>)}
              </Select>
            </FL>
          )}

          {col.visibleOrder && <FL label="Order" err={errors.order}><Input small type="number" placeholder="1" value={order} onChange={(e) => field('order', setOrder)(e.target.value)} /></FL>}

          <FL label="Date" required err={errors.date}>
            <div className="space-y-3">
              <Input small type="datetime-local" required value={date} onChange={(e) => field('date', setDate)(e.target.value)} />
              <div className="flex gap-1">
                <Button onClick={() => setDate(stamp(-1))}>Yesterday</Button>
                <Button onClick={() => setDate(stamp(0))}>Now</Button>
                <Button onClick={() => setDate(stamp(1))}>Tomorrow</Button>
              </div>
            </div>
          </FL>

          {/* "Published" dengan Date di masa depan tetap "Published". */}
          <FL label="Status" required err={errors.status}>
            <div className="space-y-3">
              {Object.entries(cd.statuses).map(([v, l]) => <Radio key={v} label={l} disabled={!cd.canEditStatus} checked={status === v} onChange={() => field('status', setStatus)(v)} />)}
            </div>
          </FL>

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
