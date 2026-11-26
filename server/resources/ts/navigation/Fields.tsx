// Padanan collections::partials.entry-field (cabang yang dipakai form Navigation) + EvaluatesFieldConditions.
import { arrayMove, useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { type ReactNode, useEffect, useState } from 'react';
import { type Asset } from '../media/api';
import { Checkbox } from '../shared/flux';
import { Button, Icon, Input, Select, cn } from '../shared/ui';
import { type ContentData, type Field } from './api';
import { Bard, PROSE } from './Bard';
import { SortList } from './SortList';

export type Path = (string | number)[];
export type Bag = Record<string, unknown>;

export interface Env {
  cd: ContentData;
  errors: Record<string, string>;
  assets: Record<number, Asset>;
  set: (path: Path, value: unknown) => void;
  pickAssets: (f: Field, path: Path, ids: number[]) => void;
  pickMedia: (container: string, single: boolean, ok: (assets: Asset[]) => void) => void;
}

export const uid = () => crypto.randomUUID();

// = fillFieldsData($fields, null): baris baru berisi nilai bawaan tiap field.
export function blank(fields: Field[]): Bag {
  const o: Bag = {};
  for (const f of fields) o[f.handle] = f.type === 'replicator' || f.type === 'assets' || f.type === 'taxonomy' ? [] : f.default ?? (f.type === 'toggle' ? false : '');
  return o;
}

/* ---------- kondisi tampil ---------- */

const s = (v: unknown) => (v === null || v === undefined ? '' : typeof v === 'boolean' ? (v ? '1' : '') : String(v));

function condEquals(actual: unknown, expected: string) {
  if (typeof actual === 'boolean') return actual === ['1', 'true', 'on', 'yes'].includes(expected.trim().toLowerCase());
  if (Array.isArray(actual)) return actual.some((x) => s(x) === expected);
  return s(actual) === expected;
}

export function visible(f: Field, scope: Bag): boolean {
  const c = f.conditions;
  const mode = c?.mode ?? 'always';
  if (!c || mode === 'always') return true;
  const actual = scope[c.field ?? ''];
  const expected = c.value ?? '';
  let m: boolean;
  switch (c.operator ?? 'equals') {
    case 'not': m = !condEquals(actual, expected); break;
    case 'strict': m = typeof actual === 'string' && actual === expected; break;
    case 'contains': m = Array.isArray(actual) ? actual.some((x) => s(x) === expected) : s(actual).includes(expected); break;
    case 'contains_any': m = Array.isArray(actual) && expected.split(',').map((w) => w.trim()).filter(Boolean).some((w) => actual.some((x) => s(x) === w)); break;
    default: m = condEquals(actual, expected);
  }
  return mode === 'show' ? m : !m;
}

/* ---------- potongan umum ---------- */

export const desc = 'mt-0 text-xs text-zinc-500 dark:text-white/60';
export const errCls = 'text-sm font-normal text-red-600 dark:text-red-400';

function Label({ f }: { f: Field }) {
  return <label title={f.handle} className="mb-3 inline-flex items-center text-sm font-medium text-zinc-800 dark:text-white">{f.display ?? f.handle}{f.required && <>&nbsp;<span className="text-red-500">*</span></>}</label>;
}

// flux:text.text-sm.font-medium dipakai field yang bukan flux:field (link, assets).
export function PlainLabel({ f }: { f: Field }) {
  return <p title={f.handle} className="text-sm font-medium text-zinc-500 dark:text-white/70">{f.display ?? f.handle}{f.required && <>&nbsp;<span className="text-red-500">*</span></>}</p>;
}

function Wrap({ f, err, extra, children }: { f: Field; err?: string; extra?: string; children: ReactNode }) {
  const d = [f.instructions, extra].filter(Boolean).join(' ').trim();
  return (
    <div>
      <Label f={f} />
      {children}
      {d && <p className={desc}>{d}</p>}
      {err && <p className={errCls}>{err}</p>}
    </div>
  );
}

const range = (f: Field, unit: string) => {
  const mn = f.min_items ?? null, mx = f.max_items ?? null;
  if (mn !== null && mx !== null) return `${mn}–${mx} ${unit}`;
  if (mn !== null) return `At least ${mn} ${unit}`;
  if (mx !== null) return `Up to ${mx} ${unit}`;
  return '';
};

const opts = (f: Field) => (Array.isArray(f.options) ? f.options : []);

export const area = 'block w-full resize-y rounded-lg border border-zinc-200 border-b-zinc-300/80 bg-white p-3 text-base text-zinc-700 shadow-xs placeholder-zinc-400 sm:text-sm dark:border-white/10 dark:bg-white/10 dark:text-zinc-300 dark:placeholder-zinc-400 dark:shadow-none';

function Switch({ on, onChange, title }: { on: boolean; onChange: (v: boolean) => void; title?: string }) {
  return (
    <button type="button" role="switch" aria-checked={on} title={title} onClick={() => onChange(!on)}
      className={cn('relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full transition-colors', on ? 'bg-[var(--color-accent)]' : 'bg-zinc-200 dark:bg-white/15')}>
      <span className={cn('inline-block size-4 rounded-full bg-white shadow-sm transition-transform dark:bg-zinc-800', on ? 'translate-x-[1.1rem] dark:bg-[var(--color-accent-foreground)]' : 'translate-x-0.5 dark:bg-white')} />
    </button>
  );
}

/* ---------- field ---------- */

export function FieldView({ f, path, value, env }: { f: Field; path: Path; value: unknown; env: Env }) {
  const { cd, set } = env;
  const err = env.errors[path.join('.')];
  const put = (v: unknown) => set(path, v);
  const ph = f.placeholder || undefined;
  const list = opts(f);

  switch (f.type) {
    case 'toggle':
      return <div><Label f={f} /><div><Switch on={value === true || value === 1 || value === '1'} onChange={put} title={f.handle} /></div>{f.instructions && <p className={desc}>{f.instructions}</p>}{err && <p className={errCls}>{err}</p>}</div>;

    case 'select':
      return <Wrap f={f} err={err}><Select value={s(value)} onChange={(e) => put(e.target.value)}><option value="">{ph ?? 'Select an option'}</option>{list.map((o) => <option key={o} value={o}>{o}</option>)}</Select></Wrap>;

    case 'author':
      return <Wrap f={f} err={err}><Select value={s(value)} onChange={(e) => put(e.target.value)}><option value="">Select an author</option>{cd.authors.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}</Select></Wrap>;

    case 'radio':
      return (
        <Wrap f={f} err={err}>
          <div className="space-y-3">{list.map((o) => (
            <label key={o} className="flex cursor-pointer items-center gap-3 text-sm font-medium text-zinc-800 dark:text-white">
              <input type="radio" className="size-4 accent-zinc-800 dark:accent-white" checked={s(value) === o} onChange={() => put(o)} />{o}
            </label>
          ))}</div>
        </Wrap>
      );

    case 'checkbox': {
      const sel = Array.isArray(value) ? (value as unknown[]).map(s) : [];
      return (
        <Wrap f={f} err={err} extra={range(f, 'selections')}>
          <div className="space-y-3">{list.map((o) => <Checkbox key={o} label={o} checked={sel.includes(o)} onChange={(on) => put(on ? [...sel, o] : sel.filter((x) => x !== o))} />)}</div>
        </Wrap>
      );
    }

    case 'date': return <Wrap f={f} err={err}><Input type="date" value={s(value)} onChange={(e) => put(e.target.value)} /></Wrap>;
    case 'number': return <Wrap f={f} err={err}><Input type="number" placeholder={ph} value={s(value)} onChange={(e) => put(e.target.value === '' ? '' : Number(e.target.value))} /></Wrap>;
    case 'textarea':
    case 'markdown': return <Wrap f={f} err={err}><textarea rows={4} placeholder={ph} className={area} value={s(value)} onChange={(e) => put(e.target.value)} /></Wrap>;
    case 'bard': return <Wrap f={f} err={err}><Bard f={f} value={s(value)} onChange={put} container={cd.bardContainer} pickMedia={env.pickMedia} /></Wrap>;
    case 'replicator': return <ReplicatorField f={f} path={path} rows={Array.isArray(value) ? (value as Bag[]) : []} err={err} env={env} />;
    case 'link': return <LinkField f={f} value={s(value)} err={err} put={put} env={env} />;
    case 'taxonomy': return <TaxonomyField f={f} value={value} err={err} put={put} env={env} />;
    case 'assets': return <AssetsField f={f} path={path} value={value} err={err} env={env} />;
    default: return <Wrap f={f} err={err}><Input placeholder={ph} value={s(value)} onChange={(e) => put(e.target.value)} /></Wrap>;
  }
}

/* replicator: kartu baris (drag urut/aktif/duplikat/hapus), tombol Add + batas max_items */

const xs = 'h-6! w-6!';

// Komponen level atas supaya isi baris (editor Bard, input) tidak di-mount ulang saat urutan berubah.
function ReplicatorRow({ f, path, rows, i, shut, toggle, limit, env }: { f: Field; path: Path; rows: Bag[]; i: number; shut: boolean; toggle: () => void; limit: boolean; env: Env }) {
  const row = rows[i];
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({ id: String(row._uid) });
  return (
    <div ref={setNodeRef} style={{ transform: CSS.Translate.toString(transform), transition }}
      className={cn('relative rounded-xl border border-zinc-200 bg-gray-200 p-1 dark:border-zinc-700 dark:bg-zinc-900', row._enabled === false && 'opacity-60', isDragging && 'z-20 border-blue-400 shadow-xl ring-2 ring-blue-400/40 dark:border-blue-500')}>
      <div className="flex items-center justify-between gap-4 px-3 py-1">
        <div className="flex min-w-0 items-center gap-2">
          <span ref={setActivatorNodeRef} {...attributes} {...listeners} title="Drag to reorder" aria-label="Drag to reorder" className={cn('shrink-0 touch-none text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300', isDragging ? 'cursor-grabbing' : 'cursor-grab')}><Icon name="bars-3" className="size-4" /></span>
          <button type="button" onClick={toggle} className="flex min-w-0 cursor-pointer items-center gap-1 text-sm font-medium text-zinc-600 hover:text-orange-700 dark:text-zinc-300">
            <Icon name={shut ? 'chevron-right' : 'chevron-down'} className="size-4 shrink-0" />
            <span className="truncate">{f.display} #{i + 1}</span>
          </button>
        </div>
        <div className="flex shrink-0 items-center gap-1">
          <Switch on={row._enabled !== false} title="Enabled" onChange={(v) => env.set([...path, i, '_enabled'], v)} />
          <Button variant="ghost" icon="document-duplicate" title="Duplicate" aria-label="Duplicate" disabled={limit} className={xs}
            onClick={() => env.set(path, [...rows.slice(0, i + 1), { ...structuredClone(row), _uid: uid() }, ...rows.slice(i + 1)])} />
          <Button variant="ghost" icon="trash" title="Delete" aria-label="Delete" className={xs}
            onClick={() => { if (confirm('Delete this item? This cannot be undone.')) env.set(path, rows.filter((_, x) => x !== i)); }} />
        </div>
      </div>
      {!shut && (
        <div className="mt-1 grid gap-3 rounded-xl bg-white px-2 py-3 dark:bg-zinc-800">
          {(f.replicator_fields ?? []).filter((x) => visible(x, row)).map((x) => <div key={x.handle}><FieldView f={x} path={[...path, i, x.handle]} value={row[x.handle]} env={env} /></div>)}
        </div>
      )}
    </div>
  );
}

function ReplicatorField({ f, path, rows, err, env }: { f: Field; path: Path; rows: Bag[]; err?: string; env: Env }) {
  const [shut, setShut] = useState<Record<string, boolean>>({});
  const max = f.max_items ?? null;
  const limit = max !== null && rows.length >= max;
  const isShut = (r: Bag) => shut[String(r._uid)] ?? !!f.collapsed_by_default;
  const all = (v: boolean) => setShut(Object.fromEntries(rows.map((r) => [String(r._uid), v])));
  return (
    <div>
      <div className="flex items-center justify-between gap-2">
        <div>
          <PlainLabel f={f} />
          {f.instructions && <p className="block text-xs text-zinc-500">{f.instructions}</p>}
        </div>
        {rows.length > 0 && (
          <div className="flex shrink-0 gap-1">
            <Button variant="ghost" icon="chevron-double-down" title="Expand all" aria-label="Expand all" className={xs} onClick={() => all(false)} />
            <Button variant="ghost" icon="chevron-double-up" title="Collapse all" aria-label="Collapse all" className={xs} onClick={() => all(true)} />
          </div>
        )}
      </div>
      <div className="mt-2 space-y-3">
        {rows.length === 0 && <p className="text-sm text-zinc-500 dark:text-white/70">No entries yet.</p>}
        <SortList ids={rows.map((r) => String(r._uid))} onSort={(id, to) => env.set(path, arrayMove(rows, rows.findIndex((r) => String(r._uid) === id), to))}>
          {rows.map((row, i) => {
            const id = String(row._uid), shutNow = isShut(row);
            return <ReplicatorRow key={id} f={f} path={path} rows={rows} i={i} shut={shutNow} toggle={() => setShut({ ...shut, [id]: !shutNow })} limit={limit} env={env} />;
          })}
        </SortList>
        <div className="flex items-center gap-2">
          <Button variant="filled" disabled={limit} onClick={() => env.set(path, [...rows, { _uid: uid(), _enabled: true, ...blank(f.replicator_fields ?? []) }])}>Add {f.display}</Button>
          {max !== null && <span className="text-xs text-zinc-500 dark:text-white/70">{rows.length} / {max}</span>}
        </div>
      </div>
      {err && <p className={errCls}>{err}</p>}
    </div>
  );
}

// Field non-localizable di bahasa non-default: hanya tampil (nilai milik bahasa default), tidak bisa diedit.
export function Locked({ f, value }: { f: Field; value: unknown }) {
  const scalar = value === null || ['string', 'number', 'boolean'].includes(typeof value);
  return (
    <div>
      <Label f={f} />
      {f.type === 'bard' && scalar && s(value) !== ''
        ? <div className={cn('max-w-none rounded-md border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-800 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-200', PROSE)} dangerouslySetInnerHTML={{ __html: s(value) }} />
        : <div className="rounded-md border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-500 dark:border-zinc-700 dark:bg-zinc-800">{scalar ? s(value) || '—' : 'Set in the default language'}</div>}
      <p className="text-sm text-zinc-500 dark:text-white/60">Same for every language — edit it from the default language.</p>
    </div>
  );
}

/* link: None / URL / Entry */

function LinkField({ f, value, err, put, env }: { f: Field; value: string; err?: string; put: (v: unknown) => void; env: Env }) {
  const [mode, setMode] = useState<'none' | 'url' | 'entry'>(value.startsWith('entry::') ? 'entry' : f.required || value ? 'url' : 'none');
  const site = env.cd.activeSite;
  const allowed = opts(f);
  const entries = env.cd.entries.filter((e) => (!allowed.length || allowed.includes(e.group)) && (site === '' || e.site === site));
  const tab = (m: typeof mode, label: string, reset: boolean) => (
    <button type="button" onClick={() => { if (mode !== m || m === 'none') { setMode(m); if (reset) put(''); } }}
      className={cn('px-2 py-1', mode === m && 'bg-zinc-800 text-white dark:bg-zinc-100 dark:text-zinc-900')}>{label}</button>
  );
  return (
    <div>
      <PlainLabel f={f} />
      {f.instructions && <p className="block text-xs text-zinc-500">{f.instructions}</p>}
      <div className="mt-1 mb-2 inline-flex overflow-hidden rounded border border-zinc-200 text-xs dark:border-zinc-700">
        {!f.required && tab('none', 'None', true)}{tab('url', 'URL', true)}{tab('entry', 'Entry', true)}
      </div>
      {mode === 'url' && <Input small placeholder={f.placeholder || 'https://...'} value={value} onChange={(e) => put(e.target.value)} />}
      {mode === 'entry' && (
        <Select small value={value} onChange={(e) => put(e.target.value)}>
          <option value="">Select an entry</option>
          {entries.map((e) => <option key={e.id} value={`entry::${e.id}`}>{e.group}: {e.label}</option>)}
        </Select>
      )}
      {err && <p className={errCls}>{err}</p>}
    </div>
  );
}

/* taxonomy: multi-pilih (dropdown centang) atau select tunggal bila max_items = 1 */

function TaxonomyField({ f, value, err, put, env }: { f: Field; value: unknown; err?: string; put: (v: unknown) => void; env: Env }) {
  const [open, setOpen] = useState(false);
  const site = env.cd.activeSite;
  const terms = env.cd.terms.filter((t) => t.group === f.options && (site === '' || t.site === site));
  const sel = (Array.isArray(value) ? value : []).map(String);
  const multi = f.max_items == null || f.max_items > 1;
  const hint = range(f, 'selections');
  useEffect(() => {
    if (!open) return;
    const out = () => setOpen(false);
    setTimeout(() => addEventListener('click', out));
    return () => removeEventListener('click', out);
  }, [open]);
  if (!multi) {
    return <Wrap f={f} err={err} extra={hint}><Select value={sel[0] ?? ''} onChange={(e) => put(e.target.value ? [Number(e.target.value)] : [])}><option value="">Select a term</option>{terms.map((t) => <option key={t.id} value={t.id}>{t.label}</option>)}</Select></Wrap>;
  }
  const toggle = (id: string) => {
    if (sel.includes(id)) return put(sel.filter((x) => x !== id).map(Number));
    put([...sel, id].map(Number));
    if (f.max_items && sel.length + 1 >= f.max_items) setOpen(false);
  };
  const label = sel.length ? sel.map((id) => terms.find((t) => t.id === id)?.label).join(', ') : 'Select terms';
  return (
    <Wrap f={f} err={err} extra={hint}>
      <div className="relative" onClick={(e) => e.stopPropagation()}>
        <button type="button" onClick={() => setOpen(!open)} className="flex w-full items-center justify-between rounded-lg border border-zinc-200 bg-white px-3 py-2 text-left text-sm shadow-xs dark:border-white/10 dark:bg-white/10 dark:text-zinc-300">
          <span className="truncate">{label}</span><Icon name="chevron-down" className="size-4 shrink-0 text-zinc-400" />
        </button>
        {open && (
          <div className="absolute z-10 mt-1 max-h-56 w-full overflow-y-auto rounded-lg border border-zinc-200 bg-white py-1 shadow-lg dark:border-white/10 dark:bg-zinc-800">
            {terms.length === 0 && <p className="block px-3 py-1.5 text-sm text-zinc-500">No terms yet.</p>}
            {terms.map((t) => (
              <label key={t.id} className="flex cursor-pointer items-center justify-between px-3 py-1.5 text-sm hover:bg-zinc-50 dark:hover:bg-zinc-700">
                <span>{t.label}</span>
                <input type="checkbox" className="peer sr-only" checked={sel.includes(t.id)} onChange={() => toggle(t.id)} />
                <Icon name="check" className="hidden size-4 shrink-0 text-zinc-800 peer-checked:block dark:text-zinc-100" />
              </label>
            ))}
          </div>
        )}
      </div>
    </Wrap>
  );
}

/* assets: thumbnail terpilih + tombol "+" yang membuka picker */

function AssetsField({ f, path, value, err, env }: { f: Field; path: Path; value: unknown; err?: string; env: Env }) {
  const ids = (Array.isArray(value) ? value : []).map(Number);
  const valid = ids.filter((id) => env.assets[id]); // id yang asset-nya sudah dihapus tidak dihitung
  const maxReached = f.max_items != null && valid.length >= f.max_items;
  const hint = range(f, 'items');
  return (
    <div>
      <PlainLabel f={f} />
      {f.instructions && <p className="block text-xs text-zinc-500">{f.instructions}</p>}
      {hint && <p className="block text-xs text-zinc-500">{hint}</p>}
      <div className="mt-2 flex flex-wrap gap-2">
        {valid.map((id) => {
          const a = env.assets[id];
          return (
            <div key={id} className="group relative size-16 overflow-hidden rounded border border-zinc-200 dark:border-zinc-700">
              {a.mime_type.startsWith('image/')
                ? <img src={a.url} className="h-full w-full object-cover" alt={a.title} />
                : <div className="flex h-full w-full items-center justify-center bg-zinc-100 text-[10px] uppercase dark:bg-zinc-800">{a.extension}</div>}
              <button type="button" onClick={() => env.set(path, valid.filter((x) => x !== id))} className="absolute top-0 right-0 rounded-bl bg-black/60 px-1 text-xs text-white opacity-0 group-hover:opacity-100">✕</button>
            </div>
          );
        })}
        {!maxReached && (
          <button type="button" onClick={() => env.pickAssets(f, path, valid)} className="flex size-16 items-center justify-center rounded border border-dashed border-zinc-300 text-zinc-400 hover:border-zinc-400 hover:text-zinc-600 dark:border-zinc-600"><Icon name="plus" className="size-5" /></button>
        )}
      </div>
      {err && <p className={errCls}>{err}</p>}
    </div>
  );
}
