// Padanan komponen Flux yang dipakai halaman Media. Kelas Tailwind disalin dari vendor/livewire/flux di source
// (button, input, select, modal, breadcrumbs, toast) supaya tampilan sama persis.
import { type ReactNode, useEffect, useRef } from 'react';
import { FOLDER_PATH, icons, PDF_PATHS } from './icons';

export const cn = (...c: (string | false | null | undefined)[]) => c.filter(Boolean).join(' ');

// Heroicons outline 24 (flux:icon default).
export const Icon = ({ name, className = 'size-4' }: { name: string; className?: string }) => (
  <svg className={cn('shrink-0', className)} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" aria-hidden="true">
    {icons[name].map((d, i) => <path key={i} strokeLinecap="round" strokeLinejoin="round" d={d} />)}
  </svg>
);

// Heroicons mini 20 (flux:icon variant="mini": ikon di dalam input & pemisah breadcrumb).
const mini = {
  'magnifying-glass': 'M9 3.5a5.5 5.5 0 1 0 0 11 5.5 5.5 0 0 0 0-11ZM2 9a7 7 0 1 1 12.452 4.391l3.328 3.329a.75.75 0 1 1-1.06 1.06l-3.329-3.328A7 7 0 0 1 2 9Z',
  'chevron-right': 'M8.22 5.22a.75.75 0 0 1 1.06 0l4.25 4.25a.75.75 0 0 1 0 1.06l-4.25 4.25a.75.75 0 0 1-1.06-1.06L11.94 10 8.22 6.28a.75.75 0 0 1 0-1.06Z',
};
export const MiniIcon = ({ name, className = 'size-5' }: { name: keyof typeof mini; className?: string }) => (
  <svg className={cn('shrink-0', className)} viewBox="0 0 20 20" fill="currentColor" aria-hidden="true"><path fillRule="evenodd" clipRule="evenodd" d={mini[name]} /></svg>
);

export const FolderIcon = ({ className }: { className: string }) => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 80 66" className={className}><path fill="currentColor" d={FOLDER_PATH} /></svg>
);

export const PdfIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 43 56" className="asset-thumbnail h-1/2 w-1/2 rounded-sm">
    {PDF_PATHS.map((p, i) => <path key={i} fill={p.fill} fillRule="evenodd" d={p.d} />)}
  </svg>
);

type Variant = 'outline' | 'primary' | 'filled' | 'ghost' | 'danger';
const variants: Record<Variant, string> = {
  outline: 'bg-white hover:bg-zinc-50 dark:bg-zinc-700 dark:hover:bg-zinc-600/75 text-zinc-800 dark:text-white border border-zinc-200 hover:border-zinc-200 border-b-zinc-300/80 dark:border-zinc-600 dark:hover:border-zinc-600 shadow-xs',
  primary: 'bg-[var(--color-accent)] hover:bg-[color-mix(in_oklab,_var(--color-accent),_transparent_10%)] text-[var(--color-accent-foreground)] border border-black/10 dark:border-0 shadow-[inset_0px_1px_--theme(--color-white/.2)]',
  filled: 'bg-zinc-800/5 hover:bg-zinc-800/10 dark:bg-white/10 dark:hover:bg-white/20 text-zinc-800 dark:text-white',
  ghost: 'bg-transparent hover:bg-zinc-800/5 dark:hover:bg-white/15 text-zinc-800 dark:text-white',
  danger: 'bg-red-500 hover:bg-red-600 dark:bg-red-600 dark:hover:bg-red-500 text-white shadow-[inset_0px_1px_var(--color-red-500),inset_0px_2px_--theme(--color-white/.15)] dark:shadow-none',
};

// flux:button size="sm": h-8 rounded-md; tanpa teks = tombol persegi w-8 dengan ikon size-5.
export function Button({ variant = 'outline', icon, children, className, ...p }: { variant?: Variant; icon?: string } & React.ButtonHTMLAttributes<HTMLButtonElement>) {
  const square = !children;
  return (
    <button type="button" {...p} className={cn(
      'relative inline-flex h-8 cursor-pointer items-center justify-center gap-2 rounded-md text-sm font-medium whitespace-nowrap disabled:pointer-events-none disabled:cursor-default disabled:opacity-50 disabled:shadow-none dark:disabled:opacity-50',
      square ? 'w-8' : cn(icon ? 'ps-2' : 'ps-3', 'pe-3'), variants[variant], className)}>
      {icon && <Icon name={icon} className={square ? 'size-5' : 'size-4'} />}{children}
    </button>
  );
}

// flux:button.group
export const ButtonGroup = ({ children }: { children: ReactNode }) => <div className="group/button flex [&>*:first-child:not(:last-child)]:rounded-e-none [&>*:last-child:not(:first-child)]:rounded-s-none">{children}</div>;

// flux:input (+ label/description via flux:field). sm = h-8 rounded-md.
export function Input({ label, description, onEnter, icon, small, className, ...p }: { label?: string; description?: string; onEnter?: () => void; icon?: 'magnifying-glass'; small?: boolean } & React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <div className="block min-w-0">
      {label && <label className={cn('inline-flex items-center text-sm font-medium text-zinc-800 dark:text-white', description ? 'mb-2' : 'mb-3')}>{label}</label>}
      {description && <p className="mb-3 text-sm text-zinc-500 dark:text-white/60">{description}</p>}
      <div className={cn('group/input relative block w-full', className)}>
        {icon && <div className="pointer-events-none absolute start-0 top-0 bottom-0 flex items-center justify-center border-s border-transparent ps-3 text-xs text-zinc-400/75 dark:text-white/60"><MiniIcon name={icon} /></div>}
        <input {...p} onKeyDown={(e) => e.key === 'Enter' && onEnter?.()}
          className={cn('block w-full appearance-none border bg-white shadow-xs placeholder-zinc-400 border-zinc-200 border-b-zinc-300/80 text-zinc-700 dark:border-white/10 dark:bg-white/10 dark:text-zinc-300 dark:placeholder-zinc-400 dark:shadow-none pe-3',
            small ? 'h-8 rounded-md py-1.5 text-sm leading-[1.125rem]' : 'h-10 rounded-lg py-2 text-base leading-[1.375rem] sm:text-sm', icon ? 'ps-10' : 'ps-3')} />
      </div>
    </div>
  );
}

// flux:select (native): panah dari select[data-flux-select-native] di app.css.
export function Select({ label, small, className, children, ...p }: { label?: string; small?: boolean } & React.SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <div className="block min-w-0">
      {label && <label className="mb-3 inline-flex items-center text-sm font-medium text-zinc-800 dark:text-white">{label}</label>}
      <select {...p} data-flux-select-native
        className={cn('block w-full appearance-none border ps-3 pe-10 shadow-xs bg-white text-zinc-700 border-zinc-200 border-b-zinc-300/80 dark:border-white/10 dark:bg-white/10 dark:text-zinc-300 dark:shadow-none dark:[&_option]:bg-zinc-700 dark:[&_option]:text-white',
          small ? 'h-8 rounded-md py-1.5 text-sm leading-[1.125rem]' : 'h-10 rounded-lg py-2 text-base leading-[1.375rem] sm:text-sm', className)}>{children}</select>
    </div>
  );
}

// flux:modal: <dialog> di dalam [data-flux-modal] (animasi + backdrop dari app.css), tombol close = ghost sm x-mark.
export function Modal({ open, onClose, className, children }: { open: boolean; onClose: () => void; className?: string; children: ReactNode }) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const d = ref.current!;
    if (open && !d.open) d.showModal();
    if (!open && d.open) d.close();
  }, [open]);
  return (
    <div data-flux-modal className="contents">
      <dialog ref={ref} onClose={onClose} onClick={(e) => e.target === ref.current && onClose()}
        className={cn('m-auto p-6 min-w-xs max-w-xl rounded-xl bg-white shadow-lg ring ring-black/5 dark:bg-zinc-800 dark:ring-zinc-700', className)}>
        {open && children}
        <div className="absolute end-0 top-0 me-4 mt-4">
          <Button variant="ghost" icon="x-mark" aria-label="Close modal" onClick={onClose} className="text-zinc-400! hover:text-zinc-800! dark:text-zinc-500! dark:hover:text-white!" />
        </div>
      </dialog>
    </div>
  );
}

export const ModalActions = ({ children }: { children: ReactNode }) => <div className="flex justify-end gap-2">{children}</div>;
export const Heading = ({ children, className }: { children: ReactNode; className?: string }) => <h2 className={cn('text-base font-medium text-zinc-800 dark:text-white', className)}>{children}</h2>;
export const Text = ({ children, className }: { children: ReactNode; className?: string }) => <p className={cn('text-sm font-normal text-zinc-500 dark:text-white/70', className)}>{children}</p>;

// flux:toast (bottom-right). Ikon 16px solid dari toast/index.blade.php.
const toastIcon = {
  success: 'M8 15A7 7 0 1 0 8 1a7 7 0 0 0 0 14Zm3.844-8.791a.75.75 0 0 0-1.188-.918l-3.7 4.79-1.649-1.833a.75.75 0 1 0-1.114 1.004l2.25 2.5a.75.75 0 0 0 1.15-.043l4.25-5.5Z',
  danger: 'M8 15A7 7 0 1 0 8 1a7 7 0 0 0 0 14ZM8 4a.75.75 0 0 1 .75.75v3a.75.75 0 0 1-1.5 0v-3A.75.75 0 0 1 8 4Zm0 8a1 1 0 1 0 0-2 1 1 0 0 0 0 2Z',
};
export function Toasts({ toasts }: { toasts: { id: number; text: string; danger: boolean }[] }) {
  return (
    <div className="fixed right-0 bottom-0 z-[60] flex flex-col gap-3 p-4 sm:p-6">
      {toasts.map((t) => (
        <div key={t.id} className="w-xs max-w-sm sm:w-sm">
          <div className="flex rounded-xl border border-zinc-200 border-b-zinc-300/80 bg-white p-2 shadow-lg dark:border-zinc-600 dark:bg-zinc-700">
            <div className="flex flex-1 gap-2 py-1.5 ps-2.5">
              <svg viewBox="0 0 16 16" fill="currentColor" className={cn('mt-0.5 size-4 shrink-0', t.danger ? 'text-rose-500 dark:text-rose-400' : 'text-lime-600 dark:text-lime-400')}>
                <path fillRule="evenodd" clipRule="evenodd" d={t.danger ? toastIcon.danger : toastIcon.success} />
              </svg>
              <div className="text-sm font-medium text-zinc-800 dark:text-white">{t.text}</div>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
