// Bar progres dua fase: "Uploading..." (byte terkirim) lalu "Compressing..." (server mengonversi ke WebP, per file).
import { type Progress } from './api';

export function UploadProgress({ p }: { p: Progress }) {
  const upload = p.phase === 'upload';
  // Fase proses: file yang sedang dikerjakan (index = done) ditampilkan berdenyut karena durasinya tidak bisa diprediksi.
  const done = upload ? 0 : Math.min(p.done, p.total), total = upload ? 1 : Math.max(1, p.total);
  const label = upload ? 'Uploading...' : `Compressing & converting${p.name ? ` ${p.name}` : ''}...`;
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-4 text-xs text-zinc-500">
        <span className="min-w-0 truncate">{label}</span>
        <span className="shrink-0">{upload ? `${p.pct}%` : `${Math.min(done + 1, total)} of ${total}`}</span>
      </div>
      <div className="relative h-1.5 w-full overflow-hidden rounded-full bg-zinc-200 dark:bg-white/10">
        {upload ? (
          <div className="h-full rounded-full bg-blue-500 transition-[width] duration-300 ease-out" style={{ width: `${p.pct}%` }} />
        ) : (
          <>
            <div className="absolute inset-y-0 left-0 bg-blue-500 transition-[width] duration-300 ease-out" style={{ width: `${(done / total) * 100}%` }} />
            <div className="absolute inset-y-0 animate-pulse bg-blue-400/60 transition-[left] duration-300 ease-out" style={{ left: `${(done / total) * 100}%`, width: `${100 / total}%` }} />
          </>
        )}
      </div>
    </div>
  );
}
