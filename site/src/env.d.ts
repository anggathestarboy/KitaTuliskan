/// <reference types="astro/client" />
// Tipe Astro.locals: diisi pages/[...slug].astro dari cms.ts → context() + entry/data halaman yang sedang dirender.
type Ctx = Awaited<ReturnType<typeof import('./cms').context>>;
declare namespace App {
  interface Locals extends Ctx {
    entry: Record<string, any>;
    data: Record<string, any>;
    previewVersion?: number; // diisi pages/preview/[token].astro; ada = halaman live preview
  }
}

// Variabel template: disisipkan otomatis ke src/views/**/*.astro oleh plugins/template-scope.mjs (= Astro.locals). Deklarasi ini hanya untuk editor.
declare const data: App.Locals['data'];
declare const entry: App.Locals['entry'];
declare const site: App.Locals['site'];
declare const sites: App.Locals['sites'];
declare const global: Cms.Global;
declare const navigation: Cms.Navigation;
declare const collection: App.Locals['collection'] & Cms.Collection;
declare const taxonomy: App.Locals['taxonomy'] & Cms.Taxonomy;
