// Potongan tampilan yang dipakai bersama halaman Users / Roles / User Groups (header kartu + ikon dari source).
import { type ReactNode } from 'react';
import { Button } from '../shared/ui';

const svg = (d: ReactNode) => (
  <svg className="h-6 w-6 text-gray-800 dark:text-white" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="24" height="24" fill="none" viewBox="0 0 24 24">{d}</svg>
);
const stroke = (d: string, cap: 'round' | 'square' = 'round', join = true) => (
  <path stroke="currentColor" strokeLinecap={cap} strokeLinejoin={join ? 'round' : undefined} strokeWidth="2" d={d} />
);

export const icons = {
  users: svg(stroke('M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Zm0 0a8.949 8.949 0 0 0 4.951-1.488A3.987 3.987 0 0 0 13 16h-2a3.987 3.987 0 0 0-3.951 3.512A8.948 8.948 0 0 0 12 21Zm3-11a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z')),
  roles: svg(stroke('M7 19H5a1 1 0 0 1-1-1v-1a3 3 0 0 1 3-3h1m4-6a3 3 0 1 1-6 0 3 3 0 0 1 6 0Zm7.441 1.559a1.907 1.907 0 0 1 0 2.698l-6.069 6.069L10 19l.674-3.372 6.07-6.07a1.907 1.907 0 0 1 2.697 0Z', 'square')),
  groups: svg(stroke('M4.5 17H4a1 1 0 0 1-1-1 3 3 0 0 1 3-3h1m0-3.05A2.5 2.5 0 1 1 9 5.5M19.5 17h.5a1 1 0 0 0 1-1 3 3 0 0 0-3-3h-1m0-3.05a2.5 2.5 0 1 0-2-4.45m.5 13.5h-7a1 1 0 0 1-1-1 3 3 0 0 1 3-3h3a3 3 0 0 1 3 3 1 1 0 0 1-1 1Zm-1-9.5a2.5 2.5 0 1 1-5 0 2.5 2.5 0 0 1 5 0Z')),
  userBack: svg(stroke('M10 19H5a1 1 0 0 1-1-1v-1a3 3 0 0 1 3-3h2m10 1a3 3 0 0 1-3 3m3-3a3 3 0 0 0-3-3m3 3h1m-4 3a3 3 0 0 1-3-3m3 3v1m-3-4a3 3 0 0 1 3-3m-3 3h-1m4-3v-1m-2.121 1.879-.707-.707m5.656 5.656-.707-.707m-4.242 0-.707.707m5.656-5.656-.707.707M12 8a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z', 'square')),
  roleBack: svg(<>
    <path fill="currentColor" d="M6.94318 11h-.85227l.96023-2.90909h1.07954L9.09091 11h-.85227l-.63637-2.10795h-.02272L6.94318 11Zm-.15909-1.14773h1.60227v.59093H6.78409v-.59093ZM9.37109 11V8.09091h1.25571c.2159 0 .4048.04261.5667.12784.162.08523.2879.20502.3779.35937.0899.15436.1349.33476.1349.5412 0 .20833-.0464.38873-.1392.54119-.0918.15246-.2211.26989-.3878.35229-.1657.0824-.3593.1236-.5809.1236h-.75003v-.61367h.59093c.0928 0 .1719-.0161.2372-.0483.0663-.03314.1169-.08002.152-.14062.036-.06061.054-.13211.054-.21449 0-.08334-.018-.15436-.054-.21307-.0351-.05966-.0857-.10511-.152-.13636-.0653-.0322-.1444-.0483-.2372-.0483h-.2784V11h-.78981Zm3.41481-2.90909V11h-.7898V8.09091h.7898Z" />
    <path stroke="currentColor" strokeLinejoin="round" strokeWidth="2" d="M8.31818 2c-.55228 0-1 .44772-1 1v.72878c-.06079.0236-.12113.04809-.18098.07346l-.55228-.53789c-.38828-.37817-1.00715-.37817-1.39543 0L3.30923 5.09564c-.19327.18824-.30229.44659-.30229.71638 0 .26979.10902.52813.30229.71637l.52844.51468c-.01982.04526-.03911.0908-.05785.13662H3c-.55228 0-1 .44771-1 1v2.58981c0 .5523.44772 1 1 1h.77982c.01873.0458.03802.0914.05783.1366l-.52847.5147c-.19327.1883-.30228.4466-.30228.7164 0 .2698.10901.5281.30228.7164l1.88026 1.8313c.38828.3781 1.00715.3781 1.39544 0l.55228-.5379c.05987.0253.12021.0498.18102.0734v.7288c0 .5523.44772 1 1 1h2.65912c.5523 0 1-.4477 1-1v-.7288c.1316-.0511.2612-.1064.3883-.1657l.5435.2614v.4339c0 .5523.4477 1 1 1H14v.0625c0 .5523.4477 1 1 1h.0909v.0625c0 .5523.4477 1 1 1h.6844l.4952.4823c1.1648 1.1345 3.0214 1.1345 4.1863 0l.2409-.2347c.1961-.191.3053-.454.3022-.7277-.0031-.2737-.1183-.5342-.3187-.7207l-6.2162-5.7847c.0173-.0398.0342-.0798.0506-.12h.7799c.5522 0 1-.4477 1-1V8.17969c0-.55229-.4478-1-1-1h-.7799c-.0187-.04583-.038-.09139-.0578-.13666l.5284-.51464c.1933-.18824.3023-.44659.3023-.71638 0-.26979-.109-.52813-.3023-.71637l-1.8803-1.8313c-.3883-.37816-1.0071-.37816-1.3954 0l-.5523.53788c-.0598-.02536-.1201-.04985-.1809-.07344V3c0-.55228-.4477-1-1-1H8.31818Z" />
  </>),
};

const Title = ({ children }: { children: ReactNode }) => <h1 className="text-2xl font-medium text-zinc-800 dark:text-white">{children}</h1>;

// Header halaman daftar: ikon + judul di kiri, tombol utama (opsional) di kanan.
export function ListHeader({ icon, title, action }: { icon: ReactNode; title: string; action?: { label: string; href?: string; onClick?: () => void } }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 overflow-hidden rounded-xl border border-zinc-200 px-2 py-2 md:px-4 dark:border-zinc-700">
      <div className="flex items-center gap-2">{icon}<Title>{title}</Title></div>
      {action && (action.href
        ? <a href={action.href}><Button variant="primary">{action.label}</Button></a>
        : <Button variant="primary" onClick={action.onClick}>{action.label}</Button>)}
    </div>
  );
}

// Header halaman form: link kembali (ikon) + judul.
export const FormHeader = ({ icon, back, title }: { icon: ReactNode; back: string; title: string }) => (
  <div className="flex items-center justify-start gap-2 rounded-xl border border-zinc-200 px-4 py-2 dark:border-zinc-700">
    <a href={back} className="text-sm text-zinc-500 hover:text-zinc-800 dark:text-white/60 dark:hover:text-white">{icon}</a>
    <Title>{title}</Title>
  </div>
);

export const FieldError = ({ msg }: { msg?: string }) => (msg ? <p className="mt-2 text-sm font-medium text-red-500 dark:text-red-400">{msg}</p> : null);

// Kartu tabel (bg-zinc-100 p-1.5 + putih di dalam) dipakai semua daftar.
export const TableCard = ({ children }: { children: ReactNode }) => (
  <div className="rounded-xl bg-zinc-100 p-1.5 dark:bg-zinc-900"><div className="overflow-hidden rounded-xl bg-white dark:bg-zinc-800">{children}</div></div>
);

// Ctrl/Cmd+S menyimpan form (x-on:keydown.window.ctrl.s di source).
export const saveShortcut = (save: () => void) => {
  const h = (e: KeyboardEvent) => { if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') { e.preventDefault(); save(); } };
  addEventListener('keydown', h);
  return () => removeEventListener('keydown', h);
};

export const dash = <span className="text-zinc-400">—</span>;
