// Komponen Flux tambahan (selain ui.tsx): table, dropdown/menu, badge, checkbox, toast hook.
// Kelas Tailwind disalin dari vendor/livewire/flux di source.
import { type ReactNode, useCallback, useEffect, useRef, useState } from 'react';
import { Icon, cn } from './ui';

// flux:badge size="sm" color="zinc"
export const Badge = ({ children }: { children: ReactNode }) => (
  <span className="inline-flex items-center rounded-md bg-zinc-400/15 px-2 py-1 text-xs font-medium whitespace-nowrap text-zinc-700 [print-color-adjust:exact] dark:bg-zinc-400/40 dark:text-zinc-200">{children}</span>
);

// flux:checkbox: kotak 1.125rem, centang pakai warna accent.
export function Checkbox({ label, checked, onChange }: { label: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="flex cursor-pointer items-start gap-3 text-sm">
      <input type="checkbox" className="peer sr-only" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      <span className={cn('mt-px flex size-[1.125rem] shrink-0 items-center justify-center rounded-[.3rem] border shadow-xs peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-zinc-400',
        checked ? 'border-black/10 bg-[var(--color-accent)] text-[var(--color-accent-foreground)] dark:border-transparent' : 'border-zinc-300 bg-white dark:border-white/10 dark:bg-white/10')}>
        {checked && <svg viewBox="0 0 16 16" fill="currentColor" className="size-4"><path fillRule="evenodd" clipRule="evenodd" d="M12.416 3.376a.75.75 0 0 1 .208 1.04l-5 7.5a.75.75 0 0 1-1.154.114l-3-3a.75.75 0 0 1 1.06-1.06l2.353 2.353 4.493-6.74a.75.75 0 0 1 1.04-.207Z" /></svg>}
      </span>
      <span className="font-medium text-zinc-800 dark:text-white">{label}</span>
    </label>
  );
}

// flux:table
export const Table = ({ children }: { children: ReactNode }) => (
  <div className="flex flex-col"><div className="overflow-auto"><table className="isolate min-w-full table-fixed border-separate border-spacing-0 whitespace-nowrap text-zinc-800">{children}</table></div></div>
);
export const Column = ({ children, className }: { children?: ReactNode; className?: string }) => (
  <th className={cn('border-b border-zinc-800/10 px-3 py-3 text-start text-sm font-medium text-zinc-800 first:ps-4 last:pe-4 dark:border-white/20 dark:text-white', className)}><div className="flex">{children}</div></th>
);
export const Cell = ({ children, className, colSpan, strong }: { children?: ReactNode; className?: string; colSpan?: number; strong?: boolean }) => (
  <td colSpan={colSpan} className={cn('px-3 py-3 text-sm first:ps-4 last:pe-4 not-in-[tr:first-child]:border-t border-zinc-800/10 dark:border-white/20', strong ? 'font-medium text-zinc-800 dark:text-white' : 'text-zinc-500 dark:text-zinc-300', className)}>{children}</td>
);

// flux:dropdown + flux:menu: menu muncul tepat di bawah tombol, rata kanan; tertutup saat klik di luar / scroll / resize / Esc.
export function Menu({ trigger, children }: { trigger: ReactNode; children: ReactNode }) {
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!pos) return;
    const close = () => setPos(null);
    const key = (e: KeyboardEvent) => e.key === 'Escape' && close();
    const out = (e: MouseEvent) => !ref.current?.contains(e.target as Node) && close();
    addEventListener('click', out); addEventListener('scroll', close, true); addEventListener('resize', close); addEventListener('keydown', key);
    return () => { removeEventListener('click', out); removeEventListener('scroll', close, true); removeEventListener('resize', close); removeEventListener('keydown', key); };
  }, [pos]);
  return (
    <div ref={ref} className="inline-flex">
      <div onClick={(e) => { e.stopPropagation(); if (pos) return setPos(null); const r = e.currentTarget.getBoundingClientRect(); setPos({ top: r.bottom + 4, left: Math.max(8, r.right - 192) }); }}>{trigger}</div>
      {pos && <div style={pos} onClick={() => setPos(null)} className="fixed z-50 min-w-48 rounded-lg border border-zinc-200 bg-white p-[.3125rem] shadow-xs focus:outline-hidden dark:border-zinc-600 dark:bg-zinc-700">{children}</div>}
    </div>
  );
}
export function MenuItem({ icon, danger, disabled, onClick, children }: { icon?: string; danger?: boolean; disabled?: boolean; onClick?: () => void; children: ReactNode }) {
  return (
    <button type="button" disabled={disabled} onClick={(e) => { if (disabled) return e.stopPropagation(); onClick?.(); }}
      className={cn('group/item flex w-full cursor-pointer items-center rounded-md px-2 py-1.5 text-start text-sm font-medium select-none focus:outline-hidden disabled:cursor-default disabled:opacity-50',
        danger ? 'text-zinc-800 hover:bg-red-50 hover:text-red-600 enabled:focus-visible:bg-red-50 dark:text-white dark:hover:bg-red-400/20 dark:hover:text-red-400' : 'text-zinc-800 hover:bg-zinc-50 dark:text-white dark:hover:bg-zinc-600')}>
      {icon && <Icon name={icon} className="me-2 size-4 text-zinc-400 group-hover/item:text-current dark:text-white/60" />}{children}
    </button>
  );
}

// Toast ringan: tampil 4 detik, dirender oleh <Toasts/> di ui.tsx.
export function useToasts() {
  const [toasts, setToasts] = useState<{ id: number; text: string; danger: boolean }[]>([]);
  const toast = useCallback((text: string, danger = false) => {
    const id = Date.now() + Math.random();
    setToasts((t) => [...t, { id, text, danger }]);
    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), 4000);
  }, []);
  return { toasts, toast };
}
export interface MenuAction { label: string; danger?: boolean; disabled?: boolean; run: () => void }

export function ContextMenu({ x, y, actions, close }: { x: number; y: number; actions: MenuAction[]; close: () => void }) {
  useEffect(() => { addEventListener('click', close); return () => removeEventListener('click', close); }, [close]);
  return (
    <div onClick={(e) => e.stopPropagation()} style={{ left: x, top: y }} className="fixed z-50 w-40 rounded-md border border-zinc-200 bg-white py-1 shadow-lg dark:border-zinc-700 dark:bg-zinc-900">
      {actions.map((a) => (
        <button key={a.label} type="button" disabled={a.disabled} onClick={() => { close(); a.run(); }} className={cn('block w-full px-3 py-1.5 text-left text-sm', a.disabled ? 'opacity-50' : 'cursor-pointer hover:bg-zinc-100 dark:hover:bg-zinc-800', a.danger && 'text-red-600')}>{a.label}</button>
      ))}
    </div>
  );
}
