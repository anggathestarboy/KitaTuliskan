// Klien JSON API modul Media (internal/modules/media/media.go). Error server (422/403) dilempar sebagai ApiError.
export interface Asset {
  id: number; basename: string; filename: string; extension: string; folder: string; path: string;
  size: number; mime_type: string; width: number; height: number;
  title: string; alt: string; loading: 'lazy' | 'eager'; url: string; copy_url: string; updated_at: string;
}
export interface Folder { name: string; path: string }
export interface Can { upload: boolean; editFolders: boolean; edit: boolean; move: boolean; rename: boolean; delete: boolean }
export interface Listing { containers: { handle: string; title: string }[]; container: string; path: string; folders: Folder[]; files: Asset[]; can: Can }
export interface Item { type: 'file' | 'folder'; key: string }

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
    // XHR, bukan fetch: hanya XHR yang punya event progress upload.
    upload: (container: string, path: string, files: File[], onProgress: (pct: number) => void) =>
      new Promise<{ failures: string[] }>((resolve, reject) => {
        const fd = new FormData();
        files.forEach((f) => fd.append('file', f));
        const x = new XMLHttpRequest();
        x.open('POST', `${base}/upload?${q({ container, path })}`);
        x.upload.onprogress = (e) => e.lengthComputable && onProgress(Math.round((e.loaded / e.total) * 100));
        x.onload = () => {
          const body = (() => { try { return JSON.parse(x.responseText); } catch { return {}; } })();
          x.status < 300 ? resolve(body) : reject(new ApiError(body.error || 'Upload failed.'));
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
