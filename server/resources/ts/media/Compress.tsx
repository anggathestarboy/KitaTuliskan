// Padanan partials/_compression-modal.blade.php: pilih level kompresi dulu, baru upload. Rumus = api.ts (sama dengan server).
import { useState } from 'react';
import { type Compression, type Progress, formatBytes, sliderQuality, sliderTarget } from './api';
import { UploadProgress } from './Progress';
import { Button, Heading, Modal, ModalActions, Text } from '../shared/ui';

export function Compress({ files, cfg, busy, progress, onCancel, onConfirm }: { files: File[]; cfg: Compression; busy: boolean; progress: Progress | null; onCancel: () => void; onConfirm: (level: number) => void }) {
  const [level, setLevel] = useState(50);
  const target = sliderTarget(cfg, level), quality = sliderQuality(cfg, level);
  return (
    <Modal open={files.length > 0} onClose={() => !busy && onCancel()} className="max-w-2xl">
      <div className="space-y-6">
        <div className="space-y-2">
          <Heading>Compress images</Heading>
          <Text>Choose the compression level first. Nothing is compressed until you click Compress &amp; Upload, and the final file will never exceed the selected target (hard cap {cfg.maxKb} KB).</Text>
        </div>

        <div className="space-y-3">
          <div className="flex items-end justify-between gap-4">
            <div>
              <Text>Target maximum size</Text>
              <p className="text-3xl font-semibold tracking-tight text-zinc-800 dark:text-zinc-100">≤ <span>{target}</span> KB</p>
              <p className="mt-1 text-xs text-zinc-500">Original total: {formatBytes(files.reduce((n, f) => n + f.size, 0))}</p>
            </div>
            <div className="text-right">
              <span className="inline-block rounded-full bg-zinc-100 px-3 py-1 text-xs font-medium text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300">Max {cfg.maxKb} KB per image</span>
              <p className="mt-1 text-xs text-zinc-500">Quality: <span>{quality}</span>%</p>
            </div>
          </div>
          <input type="range" min={0} max={100} step={1} value={level} onChange={(e) => setLevel(parseInt(e.target.value))} className="w-full cursor-pointer accent-blue-600 dark:accent-blue-400" />
          <div className="flex justify-between text-xs text-zinc-500"><span>Best quality</span><span>Smallest size</span></div>
        </div>

        <div className="max-h-64 space-y-2 overflow-y-auto pr-1">
          {files.map((f, i) => (
            <div key={i} className="flex items-center justify-between gap-4 rounded-lg border border-zinc-200 px-3 py-2 dark:border-zinc-700">
              <div className="min-w-0">
                <p className="truncate text-sm font-medium text-zinc-800 dark:text-zinc-100">{f.name}</p>
                <p className="text-xs text-zinc-500">Original: {formatBytes(f.size)}</p>
              </div>
              <div className="shrink-0 text-right">
                <p className="text-sm font-semibold text-zinc-800 dark:text-zinc-100">≤ <span>{target}</span> KB</p>
                <p className="text-[11px] text-zinc-500">after compression · Q<span>{quality}</span></p>
              </div>
            </div>
          ))}
        </div>

        <Text className="text-xs">The final size can be smaller than the target depending on the image. Images are converted to WebP. Animated GIF and WebP files are kept as-is to preserve their animation.</Text>

        {busy && progress && <UploadProgress p={progress} />}

        <ModalActions>
          <Button variant="ghost" disabled={busy} onClick={onCancel}>Cancel</Button>
          <Button variant="primary" icon="arrow-up-tray" disabled={busy} onClick={() => onConfirm(level)}>{busy ? 'Compressing...' : 'Compress & Upload'}</Button>
        </ModalActions>
      </div>
    </Modal>
  );
}
