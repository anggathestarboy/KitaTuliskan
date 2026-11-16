// Entry admin panel. CSS adalah entry Vite terpisah (lihat vite.config.ts) supaya dimuat sebagai <link>, bukan lewat JS.
import * as Turbo from '@hotwired/turbo'; // = wire:navigate di source: prefetch saat hover, swap body, progress bar
import Alpine from 'alpinejs';

declare global { interface Window { Alpine: typeof Alpine; Turbo: typeof Turbo } }
window.Alpine = Alpine;
window.Turbo = Turbo;
Alpine.start();

// Tema (light/dark/system) — dijalankan ulang tiap navigasi Turbo
const applyTheme = (t: string) => {
  localStorage.theme = t;
  document.documentElement.classList.toggle('dark', t === 'dark' || (t === 'system' && matchMedia('(prefers-color-scheme: dark)').matches));
  document.querySelectorAll<HTMLElement>('[data-theme]').forEach((b) => ['bg-white', 'shadow-xs', 'dark:bg-zinc-700'].forEach((c) => b.classList.toggle(c, b.dataset.theme === t)));
};
document.addEventListener('turbo:load', () => {
  document.querySelectorAll<HTMLElement>('[data-theme]').forEach((b) => (b.onclick = () => applyTheme(b.dataset.theme!)));
  applyTheme(localStorage.theme || 'dark');
});
