// Entry admin panel. CSS adalah entry Vite terpisah (lihat vite.config.ts) supaya dimuat sebagai <link>, bukan lewat JS.
import * as Turbo from '@hotwired/turbo'; // = wire:navigate di source: prefetch saat hover, swap body, progress bar
import Alpine from 'alpinejs';

declare global { interface Window { Alpine: typeof Alpine; Turbo: typeof Turbo; pointerSort: typeof pointerSort } }
window.Alpine = Alpine;
window.Turbo = Turbo;

// Drag berbasis pointer: item mengikuti kursor, item lain bergeser (lihat x-flip), lepas = settle mulus. Reorder data dilakukan
// handler @dragover milik Alpine; di sini hanya mengirim event sintetis ke elemen di bawah kursor.
function pointerSort(e: PointerEvent, el: HTMLElement, done: () => void) {
  if (e.button) return done();
  const y0 = e.clientY, top0 = el.getBoundingClientRect().top;
  let x = e.clientX, y = e.clientY, ty = 0, started = false, live = true;
  const follow = () => { ty = top0 + (y - y0) - (el.getBoundingClientRect().top - ty); el.style.transform = `translateY(${ty}px)`; };
  const tick = () => {
    if (!live) return;
    if (y < 70) scrollBy(0, -12); else if (y > innerHeight - 70) scrollBy(0, 12);
    const t = document.elementFromPoint(x, y);
    const list = t && !t.closest('[data-field]') ? t.closest('[x-flip]') : null;
    const kids = list ? [...list.querySelectorAll(':scope > [data-field]:not([data-dragging])')] : [];
    // celah antar item bukan "akhir list": hanya kirim ke container kalau kursor di bawah item terakhir
    if (t && !(kids.length && y < kids[kids.length - 1].getBoundingClientRect().bottom)) t.dispatchEvent(new DragEvent('dragover', { bubbles: true, cancelable: true, clientX: x, clientY: y }));
    window.Alpine.nextTick(() => { if (live) { follow(); requestAnimationFrame(tick); } });
  };
  const move = (ev: PointerEvent) => {
    x = ev.clientX; y = ev.clientY;
    if (started || Math.abs(y - y0) < 4) return;
    started = true;
    el.dataset.dragging = '';
    Object.assign(el.style, { position: 'relative', zIndex: '20', pointerEvents: 'none', boxShadow: '0 10px 25px rgb(0 0 0 / .25)' });
    document.body.style.cursor = 'grabbing';
    tick();
  };
  const up = () => {
    live = false;
    removeEventListener('pointermove', move); removeEventListener('pointerup', up); removeEventListener('pointercancel', up);
    document.body.style.cursor = '';
    const fin = () => { delete el.dataset.dragging; el.removeAttribute('style'); };
    if (started) { el.style.transition = 'transform .2s cubic-bezier(.2,.8,.3,1), box-shadow .2s'; el.style.transform = ''; el.style.boxShadow = ''; setTimeout(fin, 220); }
    done();
  };
  addEventListener('pointermove', move); addEventListener('pointerup', up); addEventListener('pointercancel', up);
}
window.pointerSort = pointerSort;

// x-flip: anak langsung yang berpindah posisi (reorder x-for saat drag) digeser mulus, seperti dnd-kit di navigations.
Alpine.directive('flip', (el) => {
  let prev = new Map<Element, number>();
  const snap = () => (prev = new Map([...el.children].map((c) => [c, (c as HTMLElement).offsetTop])));
  const mo = new MutationObserver(() => {
    mo.disconnect();
    for (const c of el.children as HTMLCollectionOf<HTMLElement>) {
      if (c.dataset.dragging !== undefined) continue; // item yang di-drag diposisikan pointerSort
      const d = (prev.get(c) ?? c.offsetTop) - c.offsetTop;
      if (d) c.animate([{ transform: `translateY(${d}px)` }, { transform: 'none' }], { duration: 200, easing: 'cubic-bezier(.2,.8,.3,1)' });
    }
    snap();
    mo.observe(el, { childList: true });
  });
  snap();
  mo.observe(el, { childList: true });
  el.addEventListener('dragstart', snap); // snapshot segar: layout bisa berubah karena collapse/expand
});
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
