// Padanan partials/seo-fields.blade.php: tab "SEO" entry. Meta Featured Image & Meta Author bersifat non-localizable
// (hanya diedit dari bahasa default), sisanya per bahasa — lihat EntryForm::SEO_LOCALIZABLE.
import { type Asset } from '../media/api';
import { area } from '../navigation/Fields';
import { Checkbox } from '../shared/flux';
import { Icon, Input, Select } from '../shared/ui';
import { FL } from './Field';

export interface SeoOptions { robots: Record<string, string>; priorities: string[]; changefreqs: string[] }
export type Seo = { meta_title: string; meta_description: string; meta_canonical: string; robots: string[]; meta_featured_image: number[]; meta_author: string; sitemap_priority: string; sitemap_changefreq: string };

const TITLE_LIMIT = 60, DESCRIPTION_LIMIT = 160; // batas teks yang dipotong Google di hasil pencarian

const Locked = () => (
  <>
    <div className="rounded-md border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-500 dark:border-zinc-700 dark:bg-zinc-800">Set in the default language</div>
    <p className="text-sm text-zinc-500 dark:text-white/60">Same for every language — edit it from the default language.</p>
  </>
);

export function SeoTab({ seo, set, errors, isOrigin, options, authors, assets, canPick, pick }: {
  seo: Seo; set: (p: Partial<Seo>) => void; errors: Record<string, string>; isOrigin: boolean; options: SeoOptions; authors: { id: string; name: string }[];
  assets: Record<number, Asset>; canPick: boolean; pick: () => void;
}) {
  const e = (k: string) => errors[`seo.${k}`];
  const counter = (v: string, limit: number) => <div className="flex justify-end"><p className={`text-xs ${v.length > limit ? 'text-red-500' : 'text-zinc-400'}`}>{v.length} / {limit}</p></div>;
  return (
    <div className="space-y-6">
      <FL label="Meta Title" required err={e('meta_title')}>
        <Input maxLength={TITLE_LIMIT} value={seo.meta_title} onChange={(x) => set({ meta_title: x.target.value })} />
        {counter(seo.meta_title, TITLE_LIMIT)}
      </FL>
      <FL label="Meta Description" required err={e('meta_description')}>
        <textarea rows={3} maxLength={DESCRIPTION_LIMIT} className={area} value={seo.meta_description} onChange={(x) => set({ meta_description: x.target.value })} />
        {counter(seo.meta_description, DESCRIPTION_LIMIT)}
      </FL>
      <FL label="Meta Canonical" description="Meta canonical can be automatic if it is empty." err={e('meta_canonical')}>
        <Input placeholder="https://example.com/page" value={seo.meta_canonical} onChange={(x) => set({ meta_canonical: x.target.value })} />
      </FL>
      <FL label="Robots" required err={e('robots')} description={'If left blank then the default is "noindex, nofollow". Checking "No Index" also excludes this entry from sitemap.xml.'}>
        <div className="space-y-3">
          {Object.entries(options.robots).map(([v, l]) => (
            <Checkbox key={v} label={l} checked={seo.robots.includes(v)} onChange={(on) => set({ robots: on ? [...seo.robots, v] : seo.robots.filter((x) => x !== v) })} />
          ))}
        </div>
      </FL>
      <FL label="Meta Featured Image" required err={e('meta_featured_image')}>
        {!isOrigin ? <Locked /> : (
          <div className="mt-2 flex flex-wrap gap-2">
            {seo.meta_featured_image.map((id) => assets[id] && (
              <div key={id} className="group relative size-16 overflow-hidden rounded border border-zinc-200 dark:border-zinc-700">
                {assets[id].mime_type.startsWith('image/')
                  ? <img src={assets[id].url} className="h-full w-full object-cover" alt={assets[id].title} />
                  : <div className="flex h-full w-full items-center justify-center bg-zinc-100 text-[10px] uppercase dark:bg-zinc-800">{assets[id].extension}</div>}
                <button type="button" onClick={() => set({ meta_featured_image: seo.meta_featured_image.filter((x) => x !== id) })} className="absolute top-0 right-0 rounded-bl bg-black/60 px-1 text-xs text-white opacity-0 group-hover:opacity-100">✕</button>
              </div>
            ))}
            {seo.meta_featured_image.length === 0 && (
              <button type="button" disabled={!canPick} onClick={pick} className="flex size-16 cursor-pointer items-center justify-center rounded border border-dashed border-zinc-300 text-zinc-400 hover:border-zinc-400 hover:text-zinc-500 disabled:cursor-default disabled:opacity-50 dark:border-zinc-600"><Icon name="plus" className="size-5" /></button>
            )}
          </div>
        )}
      </FL>
      <FL label="Meta Author" required err={e('meta_author')}>
        {!isOrigin ? <Locked /> : (
          <Select value={seo.meta_author} onChange={(x) => set({ meta_author: x.target.value })}>
            <option value="">Select an author</option>
            {authors.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
          </Select>
        )}
      </FL>
      <div className="grid grid-cols-2 gap-4">
        <FL label="Sitemap Priority"><Select value={seo.sitemap_priority} onChange={(x) => set({ sitemap_priority: x.target.value })}>{options.priorities.map((p) => <option key={p} value={p}>{p}</option>)}</Select></FL>
        <FL label="Sitemap Changefreq"><Select value={seo.sitemap_changefreq} onChange={(x) => set({ sitemap_changefreq: x.target.value })}>{options.changefreqs.map((p) => <option key={p} value={p}>{p.charAt(0).toUpperCase() + p.slice(1)}</option>)}</Select></FL>
      </div>
    </div>
  );
}
