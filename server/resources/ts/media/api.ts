// Klien JSON API modul Media (internal/modules/media/media.go). Error server (422/403) dilempar sebagai ApiError.
export interface Asset {
  id: number; basename: string; filename: string; extension: string; folder: string; path: string;
  size: number; mime_type: string; width: number; height: number;
  title: string; alt: string; loading: 'lazy' | 'eager'; url: string; copy_url: string; updated_at: string;
}
export interface Folder { name: string; path: string }
export interface Can { upload: boolean; editFolders: boolean; edit: boolean; move: boolean; rename: boolean; delete: boolean }
export interface Compression { enabled: boolean; minKb: number; maxKb: number; minQuality: number; maxQuality: number }
export interface Listing { compression: Compression; containers: { handle: string; title: string }[]; container: string; path: string; folders: Folder[]; files: Asset[]; can: Can }
export interface Item { type: 'file' | 'folder'; key: string }

export type Progress = { phase: 'upload'; pct: number } | { phase: 'process'; done: number; total: number; name: string };

export class ApiError extends Error {}

const q = (o: Record<string, string>) => new URLSearchParams(o).toString();
const fail = async (r: Response): Promise<never> => { throw new ApiError((await r.json().catch(() => ({}))).error || 'Something went wrong.'); };

export function client(cp: string) {
  const base = `${cp}/media/api`;
  return {
    listing: async (container: string, path: string, search: string, signal?: AbortSignal): Promise<Listing> => {
      const r = await fetch(`${base}/listing?${q({ container, path, search })}`, { signal });
      return r.ok ? r.json() : fail(r);
    },
    post: async <T = unknown>(endpoint: string, container: string, body: unknown): Promise<T> => {
      const r = await fetch(`${base}/${endpoint}?${q({ container })}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
      return r.ok ? r.json() : fail(r);
    },
    // XHR, bukan fetch: hanya XHR yang punya event progress upload. Server membalas NDJSON (start, file, done) supaya fase
    // pemrosesan/konversi WebP setelah byte terkirim juga terlihat, bukan berhenti di 100%.
    upload: (container: string, path: string, files: File[], onProgress: (p: Progress) => void, level?: number) =>
      new Promise<{ failures: string[] }>((resolve, reject) => {
        const fd = new FormData();
        files.forEach((f) => fd.append('file', f));
        if (level !== undefined) fd.append('level', String(level));
        const x = new XMLHttpRequest();
        x.open('POST', `${base}/upload?${q({ container, path })}`);
        let seen = 0, failures: string[] = [];
        const drain = () => {
          const t = x.responseText, end = t.lastIndexOf('\n');
          if (end < seen) return;
          for (const line of t.slice(seen, end).split('\n')) {
            if (!line) continue;
            const ev = JSON.parse(line);
            if (ev.event === 'file') onProgress({ phase: 'process', done: ev.index, total: ev.total, name: ev.name });
            if (ev.event === 'done') failures = ev.failures;
          }
          seen = end + 1;
        };
        x.upload.onprogress = (e) => e.lengthComputable && onProgress({ phase: 'upload', pct: Math.round((e.loaded / e.total) * 100) });
        x.upload.onload = () => onProgress({ phase: 'process', done: 0, total: files.length, name: '' });
        x.onprogress = () => x.status < 300 && drain();
        x.onload = () => {
          if (x.status >= 300) return reject(new ApiError((() => { try { return JSON.parse(x.responseText).error; } catch { return ''; } })() || 'Upload failed.'));
          drain();
          resolve({ failures });
        };
        x.onerror = () => reject(new ApiError('Upload failed.'));
        x.send(fd);
      }),
    downloadUrl: (container: string, type: Item['type'], key: string) => `${cp}/media/download?${q({ container, type, key })}`,
  };
}

// Number::fileSize (presisi 0) dan diffForHumans di source.
export const fileSize = (b: number) => {
  const u = ['B', 'KB', 'MB', 'GB', 'TB'];
  let i = 0;
  while (b >= 1024 && i < u.length - 1) { b /= 1024; i++; }
  return `${Math.round(b)} ${u[i]}`;
};
export const ago = (iso: string) => {
  const s = Math.round((new Date(iso).getTime() - Date.now()) / 1000);
  const units: [Intl.RelativeTimeFormatUnit, number][] = [['year', 31536000], ['month', 2592000], ['week', 604800], ['day', 86400], ['hour', 3600], ['minute', 60], ['second', 1]];
  const [u, n] = units.find(([, n]) => Math.abs(s) >= n) ?? units[6];
  return new Intl.RelativeTimeFormat('en', { numeric: 'always' }).format(Math.trunc(s / n), u);
};

// Padanan WebpImageConverter::supports + partitionImagesBySize di source: gambar yang bisa dikonversi dan >800 KB menunggu slider.
export const HARD_MAX = 800 * 1024;
const convertibleMime = ['image/jpeg', 'image/pjpeg', 'image/png', 'image/gif', 'image/webp', 'image/bmp', 'image/x-ms-bmp'];
export const needsCompression = (f: File) => {
  const ext = f.name.split('.').pop()?.toLowerCase() ?? '';
  const ok = convertibleMime.includes(f.type.toLowerCase()) || (['', 'application/octet-stream'].includes(f.type) && ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp'].includes(ext));
  return ok && f.size > HARD_MAX;
};

// Rumus slider = compressionTargetKb()/compressionQuality() di source (0 = kualitas terbaik, 100 = file terkecil).
export const sliderTarget = (c: Compression, level: number) => Math.round(c.maxKb - (c.maxKb - c.minKb) * (level / 100));
export const sliderQuality = (c: Compression, level: number) => Math.round(c.maxQuality - (c.maxQuality - c.minQuality) * (level / 100));
export const formatBytes = (b: number) => (b >= 1048576 ? `${(b / 1048576).toFixed(1)} MB` : `${Math.max(1, Math.round(b / 1024))} KB`);
