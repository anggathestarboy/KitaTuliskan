// Padanan partials/_grid-item, _list-item, _thumbnail di source + context menu.
import { type DragEvent, type MouseEvent, useEffect, useState } from 'react';
import { type Asset, type Can, ago, fileSize } from './api';
import { FolderIcon, Icon, PdfIcon, cn } from './ui';

export interface Ctx {
  can: Can;
  selected: string[];
  toggle: (type: 'file' | 'folder', key: string) => void;
  menu: (id: string, e: MouseEvent, toggle?: boolean) => void;
  view: (a: Asset) => void;
  goTo: (path: string) => void;
  drop: (type: string, key: string, target: string) => void;
}

export function Thumbnail({ asset, fit = 'object-cover', interactive = false }: { asset: Asset; fit?: string; interactive?: boolean }) {
  const cls = `h-full w-full ${fit}`;
  if (asset.mime_type.startsWith('image/')) return <img src={asset.url} alt={asset.alt} loading={asset.loading} draggable={false} className={cls} />;
  if (asset.mime_type.startsWith('video/')) {
    if (interactive) return <video src={asset.url} controls preload="metadata" className={cls} />;
    return (
      <div className="relative h-full w-full">
        <video src={asset.url} preload="metadata" muted playsInline draggable={false} className={cls} />
        <span className="absolute inset-0 flex items-center justify-center"><Icon name="play-circle" className="size-1/3 text-white drop-shadow" /></span>
      </div>
    );
  }
  if (asset.extension === 'pdf') return <PdfIcon />;
  return <Icon name="document" className="size-1/2 text-zinc-400" />;
}

const kebab = 'rounded p-1 hover:bg-zinc-200 dark:hover:bg-zinc-700';

type Row = { type: 'folder'; name: string; path: string } | { type: 'file'; asset: Asset };

// Satu komponen untuk grid & list: bedanya hanya tata letak. Perilaku (pilih, dblclick, drag, context menu) identik dengan source.
export function Entry({ row, mode, ctx }: { row: Row; mode: 'grid' | 'list'; ctx: Ctx }) {
  const [over, setOver] = useState(false);
  useEffect(() => {
    const off = () => setOver(false);
    addEventListener('dragend', off);
    addEventListener('drop', off);
    return () => { removeEventListener('dragend', off); removeEventListener('drop', off); };
  }, []);

  const folder = row.type === 'folder';
  const key = folder ? row.path : String(row.asset.id);
  const id = `${row.type}-${key}`;
  const sel = ctx.selected.includes(`${row.type}:${key}`);
  const dnd = folder && ctx.can.move ? {
    onDragOver: (e: DragEvent) => { if (e.dataTransfer.types.includes('Files')) return; e.preventDefault(); e.stopPropagation(); setOver(true); },
    onDragLeave: (e: DragEvent) => { e.stopPropagation(); if (!e.currentTarget.contains(e.relatedTarget as Node)) setOver(false); },
    onDrop: (e: DragEvent) => {
      if (e.dataTransfer.types.includes('Files')) return;
      e.preventDefault(); e.stopPropagation(); setOver(false);
      const raw = e.dataTransfer.getData('text/plain'), i = raw.indexOf('|');
      ctx.drop(raw.slice(0, i), raw.slice(i + 1), row.path);
    },
  } : {};
  const common = {
    draggable: ctx.can.move,
    onDragStart: (e: DragEvent) => e.dataTransfer.setData('text/plain', `${row.type}|${key}`),
    onClick: () => ctx.toggle(row.type, key),
    onDoubleClick: () => (folder ? ctx.goTo(row.path) : ctx.view(row.asset)),
    onContextMenu: (e: MouseEvent) => { e.preventDefault(); e.stopPropagation(); ctx.menu(id, e); },
    ...dnd,
  };
  const label = folder ? row.name : row.asset.basename;
  const kebabBtn = (
    <button type="button" onClick={(e) => { e.stopPropagation(); ctx.menu(id, e, true); }} className={cn(kebab, mode === 'grid' ? 'absolute top-1 right-1 hidden bg-white/80 group-hover:block dark:bg-zinc-900/80' : 'hidden group-hover:block')}>
      <Icon name="ellipsis-vertical" />
    </button>
  );
  const folderSvg = (c: string) => <FolderIcon className={cn('size-full text-[var(--color-accent)]/90 group-hover:text-[var(--color-accent)]', c)} />;

  if (mode === 'grid') {
    return folder ? (
      <div {...common} className={cn('group relative flex flex-col items-center gap-1 overflow-hidden rounded-lg p-1', sel && 'ring-2 ring-blue-500', over && 'bg-blue-50 ring-2 ring-blue-400 dark:bg-blue-950')}>
        <div className="flex aspect-square w-full items-center justify-center rounded-lg p-2">{folderSvg('')}</div>
        <span className="w-full truncate text-center text-xs">{label}</span>
        {kebabBtn}
      </div>
    ) : (
      <div {...common} className={cn('group relative flex flex-col gap-1 rounded-lg border border-zinc-200 p-1.5 hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800', sel && 'ring-2 ring-blue-500')}>
        <div className="flex aspect-square w-full items-center justify-center overflow-hidden rounded bg-zinc-100 dark:bg-zinc-800"><Thumbnail asset={row.asset} /></div>
        <span className="w-full truncate text-center text-xs">{label}</span>
        {kebabBtn}
      </div>
    );
  }
  return (
    <div {...common} className={cn('group flex items-center gap-3 px-2 py-2 hover:bg-zinc-50 dark:hover:bg-zinc-800', sel && 'bg-blue-50 dark:bg-blue-950/40', over && 'bg-blue-50 ring-2 ring-blue-400 dark:bg-blue-950')}>
      {folder
        ? <div className="flex size-8 shrink-0 items-center justify-center">{folderSvg('')}</div>
        : <div className="flex size-8 shrink-0 items-center justify-center overflow-hidden rounded-sm bg-zinc-100 dark:bg-zinc-800"><Thumbnail asset={row.asset} /></div>}
      <span className="min-w-0 flex-1 truncate text-sm">{label}</span>
      <span className="hidden w-20 shrink-0 text-right text-sm text-zinc-400 sm:block">{!folder && row.asset.size ? fileSize(row.asset.size) : '—'}</span>
      <span className="hidden w-36 shrink-0 text-right text-sm text-zinc-400 md:block">{!folder && row.asset.updated_at ? ago(row.asset.updated_at) : '—'}</span>
      <div className="relative w-8 shrink-0">{kebabBtn}</div>
    </div>
  );
}

export interface MenuAction { label: string; danger?: boolean; run: () => void }

export function ContextMenu({ x, y, actions, close }: { x: number; y: number; actions: MenuAction[]; close: () => void }) {
  useEffect(() => { addEventListener('click', close); return () => removeEventListener('click', close); }, [close]);
  return (
    <div onClick={(e) => e.stopPropagation()} style={{ left: x, top: y }} className="fixed z-50 w-40 rounded-md border border-zinc-200 bg-white py-1 shadow-lg dark:border-zinc-700 dark:bg-zinc-900">
      {actions.map((a) => (
        <button key={a.label} type="button" onClick={() => { close(); a.run(); }} className={cn('block w-full cursor-pointer px-3 py-1.5 text-left text-sm hover:bg-zinc-100 dark:hover:bg-zinc-800', a.danger && 'text-red-600')}>{a.label}</button>
      ))}
    </div>
  );
}
