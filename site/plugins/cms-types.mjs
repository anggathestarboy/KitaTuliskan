// Autocomplete di editor: membaca blueprint dari CMS lalu menulis src/cms.generated.d.ts, jadi `navigation.` menyarankan handle
// (header, footer, ...), `navigation.header.` menyarankan field blueprint-nya, dan `menu_loop?.map((m) => m.` menyarankan field replicator.
// Dijalankan saat `astro dev` mulai lalu dicek ulang tiap 2 dtk (file hanya ditulis bila isinya berubah), jadi blueprint yang baru disimpan di admin
// (mis. field replicator baru) langsung dapat snippet tanpa restart dan tanpa membuka halaman. Gagal fetch (CMS belum/tidak jalan) = file lama dibiarkan, dicoba lagi. Setelah blueprint berubah, kalau editor belum update: "TypeScript: Restart TS Server".
// Snippet loop: untuk handle yang berupa list (taxonomy.x, collection.x, field replicator/assets-banyak) ditulis juga .vscode/cms.code-snippets,
// jadi memilih `taxonomy.breaking_news` di Ctrl+Space langsung jadi `taxonomy.breaking_news?.map((item) => (...))`; bukan list = tidak ada snippet.
// File snippet di-gitignore: setup awal hanya berisi loop taxonomy, collection (pages), dan navigation yang ada; snippet field replicator baru muncul setelah dibuat di blueprint.
// shortcut: select/radio bertipe string (bukan union opsi), fieldset bertipe Record<string, any>; rapikan bila perlu.
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { pathToFileURL } from 'node:url';

const OUT = new URL('../src/cms.generated.d.ts', import.meta.url);
const json = (v) => (typeof v === 'string' ? JSON.parse(v) : v ?? {});
const key = (h) => JSON.stringify(h);

function fieldType(f) {
  switch (f.type) {
    case 'number': return 'number';
    case 'toggle': return 'boolean';
    case 'checkbox': return 'string[]';
    case 'taxonomy': return f.max_items == null || f.max_items > 1 ? 'CmsTerm[]' : 'CmsTerm | null';
    case 'assets': return f.max_items == null || f.max_items > 1 ? 'CmsAsset[]' : 'CmsAsset | null';
    case 'replicator': return `Array<{ ${props(f.replicator_fields)} }>`;
    case 'fieldset': return 'Record<string, any>';
    default: return 'string'; // text, textarea, markdown, bard, date, select, radio, link, author
  }
}
const props = (fields = []) => fields.map((f) => `${key(f.handle)}?: ${fieldType(f)};`).join(' ');

// Satu objek per handle; field dari beberapa blueprint di namespace yang sama digabung (collection bisa punya banyak blueprint).
const fieldsOf = (blueprints, ns, handle) => blueprints.filter((b) => b.namespace === `${ns}.${handle}`).flatMap((b) => json(b.data).fields ?? []);
const shape = (blueprints, ns, handle) => `{ ${props(fieldsOf(blueprints, ns, handle))} }`;

const access = (h) => (/^[A-Za-z_$][\w$]*$/.test(h) ? `.${h}` : `[${key(h)}]`);
const isList = (f) => f.type === 'replicator' || (f.type === 'assets' && (f.max_items == null || f.max_items > 1));
const opt = (a) => (a.startsWith('[') ? `?.${a}` : `?${a}`); // .x → ?.x ; ["x-y"] → ?.["x-y"]
const loop = (prefix, body, item) => ({ prefix, scope: 'astro', body: [`${body}?.map((\${1:${item}}) => (`, '	$0', '))'] });

// taxonomy.x / collection.x = path penuh. Field list (replicator, assets banyak) = prefix nama field saja, tiap kedalaman (menu_loop, menu_child_menu, ...):
// ketik `navigation.header.menu_` atau `menu.menu_child` lalu pilih snippet-nya, yang tertulis hanya `menu_loop?.map(...)` setelah titik yang sudah diketik.
function snippets({ navigations, globals, collections, taxonomies, blueprints }) {
  const out = {};
  for (const r of taxonomies) out[`taxonomy${access(r.handle)}`] = loop(`taxonomy${access(r.handle)}`, `taxonomy${access(r.handle)}`, 'item');
  for (const r of collections) out[`collection${access(r.handle)}`] = loop(`collection${access(r.handle)}`, `collection${access(r.handle)}`, 'item');
  // Navigation tanpa blueprint = daftar item {title,url,children}, jadi handle-nya sendiri sudah list. sidebar-admin = menu panel admin, bukan untuk site.
  for (const r of navigations) if (r.handle !== 'sidebar-admin' && !fieldsOf(blueprints, 'navigations', r.handle).length) out[`navigation${access(r.handle)}`] = loop(`navigation${access(r.handle)}`, `navigation${access(r.handle)}`, 'item');
  // Path penuh per modul (cocok saat seluruh `global.global_sidebar.menu_loop` diketik): navigation/global per handle, data.* untuk field entry halaman ini.
  const full = (path, safe) => { out[path] = loop(path, safe, 'item'); };
  const lists = (ns, handle) => fieldsOf(blueprints, ns, handle).filter(isList);
  for (const [root, rows, ns] of [['navigation', navigations, 'navigations'], ['global', globals, 'globals']])
    for (const r of rows) for (const f of lists(ns, r.handle)) full(`${root}${access(r.handle)}${access(f.handle)}`, `${root}${access(r.handle)}${opt(access(f.handle))}`);
  for (const r of collections) for (const f of lists('collections', r.handle)) full(`data${access(f.handle)}`, `data${opt(access(f.handle))}`);
  const single = new Set(); // handle yang di blueprint lain bukan list (mis. site_image) tidak dapat snippet, supaya tidak menyesatkan
  const walk = (fields, depth) => fields.forEach((f) => {
    if (isList(f)) out[f.handle] ??= loop(f.handle, f.handle, depth ? 'child' : 'item');
    else single.add(f.handle);
    if (f.type === 'replicator') walk(f.replicator_fields ?? [], depth + 1);
  });
  for (const b of blueprints) if (!/^forms\./.test(b.namespace ?? '')) walk(json(b.data).fields ?? [], 0);
  single.forEach((h) => delete out[h]); // hanya snippet nama-field-saja yang dihapus; path penuh tidak ambigu (tidak ada di `single` karena berprefix)
  return `${JSON.stringify(out, null, 2)}\n`;
}

async function build(base) {
  const get = (t) => fetch(`${base}/api/${t}`).then((r) => r.json()).then((v) => v ?? []); // tabel kosong = JSON null dari Go
  const [blueprints, navigations, globals, collections, taxonomies] = await Promise.all(['blueprints', 'navigations', 'global_sets', 'collections', 'taxonomies'].map(get));
  const map = (rows, ns, wrap = (s) => s) => `{ ${rows.map((r) => `${key(r.handle)}: ${wrap(shape(blueprints, ns, r.handle))};`).join(' ')} }`;
  const navs = `{ ${navigations.map((r) => `${key(r.handle)}: ${fieldsOf(blueprints, 'navigations', r.handle).length ? shape(blueprints, 'navigations', r.handle) : 'any'};`).join(' ')} }`; // tanpa blueprint = daftar item {title,url,children}
  const code = `// DIGENERATE oleh plugins/cms-types.mjs dari blueprint CMS. Jangan diedit.
interface CmsTerm { id: string; title?: string; slug?: string; taxonomy: string; data: Record<string, any>; [k: string]: any }
interface CmsAsset { url: string; alt?: string; width?: number; height?: number; loading?: string; title?: string; mime_type?: string; [k: string]: any }
type CmsEntry<T> = import('./cms').Entry & T & { url: string; data: T };
declare namespace Cms {
  type Navigation = ${navs};
  type Global = ${map(globals, 'globals')};
  type Collection = ${map(collections, 'collections', (s) => `CmsEntry<${s}>[]`)};
  type Taxonomy = ${map(taxonomies, 'taxonomies', (s) => `Array<Record<string, any> & ${s}>`)};
}
`;
  return { code, snippets: snippets({ navigations, globals, collections, taxonomies, blueprints }) };
}

const SNIPPETS = [new URL('../.vscode/cms.code-snippets', import.meta.url), new URL('../../.vscode/cms.code-snippets', import.meta.url)];
const put = async (f, text) => { if (text !== (await readFile(f, 'utf8').catch(() => ''))) await writeFile(f, text); }; // tulis hanya bila berubah: tidak memicu reload TS

export default function cmsTypes(base = process.env.CMS_URL ?? 'http://localhost:8000') {
  let busy = false, failed = false;
  const sync = async () => {
    if (busy) return;
    busy = true;
    try {
      const { code, snippets } = await build(base);
      await put(OUT, code);
      for (const f of SNIPPETS) { await mkdir(new URL('./', f), { recursive: true }); await put(f, snippets); } // folder .vscode: yang dibuka editor (site/ atau root repo) yang terpakai
      failed = false;
    } catch (e) {
      if (!failed) console.warn(`[cms-types] gagal ambil blueprint dari ${base}: ${e.message} (dicoba lagi tiap 2 dtk)`); // sekali per rentetan gagal, bukan tiap polling
      failed = true;
    } finally { busy = false; }
  };
  return {
    name: 'cms-types',
    apply: 'serve',
    buildStart: sync,
    // Polling, bukan hook request: simpan blueprint di admin tidak menyentuh Astro. Blok tanpa return (nilai kembalian configureServer dianggap post-hook).
    configureServer(s) {
      const t = setInterval(sync, 2000);
      s.httpServer?.on('close', () => clearInterval(t));
    },
  };
}

// `npm run types` = generate sekali tanpa menjalankan astro dev (CMS Go harus aktif).
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await cmsTypes((await readFile(new URL('../.env', import.meta.url), 'utf8')).match(/^CMS_URL=(.+)$/m)?.[1]).buildStart();
}
