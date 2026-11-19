// Padanan RoleForm + role-form.blade.php: title/handle, switch Super Admin, pohon permission per grup (Check all / Uncheck all).
import { useEffect, useState } from 'react';
import { Checkbox, Switch, useToasts } from '../shared/flux';
import { Button, Input, Toasts } from '../shared/ui';
import { ApiError, call, slug, visit } from '../navigation/api';
import { FieldError, FormHeader, icons, saveShortcut } from './parts';

interface Perm { value: string; label: string; description: string | null; children: Perm[] }
interface Group { group: string; permissions: Perm[] }
interface Data { groups: Group[]; role?: { id: number; title: string; handle: string; superAdmin: boolean; permissions: string[] } }

// Semua nilai di sebuah pohon (induk + anak), untuk "Check all".
const flat = (ps: Perm[]): string[] => ps.flatMap((p) => [p.value, ...flat(p.children)]);

export function RoleForm({ cp, handle }: { cp: string; handle: string }) {
  const { toasts, toast } = useToasts();
  const [d, setD] = useState<Data | null>(null);
  const [f, setF] = useState({ id: 0, title: '', handle: '', superAdmin: false, permissions: [] as string[] });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const set = (p: Partial<typeof f>) => setF((x) => ({ ...x, ...p }));

  useEffect(() => {
    call<Data>(`${cp}/roles/api/form?handle=${encodeURIComponent(handle)}`).then((r) => { setD(r); if (r.role) setF(r.role); }).catch((e) => toast(e.message, true));
  }, []);

  const save = async () => {
    setErrors({});
    try { await call(`${cp}/roles/api/save`, f); visit(`${cp}/roles`); } // pesan sukses dikirim server sebagai flash
    catch (e) {
      if (e instanceof ApiError && Object.keys(e.errors ?? {}).length) setErrors(e.errors);
      else toast(e instanceof Error ? e.message : String(e), true);
    }
  };
  useEffect(() => saveShortcut(save));

  if (!d) return <Toasts toasts={toasts} />;
  const has = (v: string) => f.permissions.includes(v);
  const toggle = (v: string, on: boolean) => set({ permissions: on ? [...f.permissions, v] : f.permissions.filter((x) => x !== v) });
  const checkAll = (vs: string[]) => set({ permissions: [...new Set([...f.permissions, ...vs])] });
  const uncheckAll = (vs: string[]) => set({ permissions: f.permissions.filter((x) => !vs.includes(x)) });
  // Super Admin membuat semua permission lain tak berarti: kosongkan pohon saat dinyalakan.
  const setSuper = (v: boolean) => set({ superAdmin: v, permissions: v ? [] : f.permissions });

  const row = (p: Perm) => <Checkbox key={p.value} label={p.label} description={p.description ?? undefined} checked={has(p.value)} onChange={(v) => toggle(p.value, v)} />;

  return (
    <section className="w-full max-w-4xl space-y-6">
      <FormHeader icon={icons.roleBack} back={`${cp}/roles`} title={f.id ? 'Edit role' : 'Add role'} />

      <form className="space-y-6" onSubmit={(e) => { e.preventDefault(); save(); }}>
        <div className="grid gap-4">
          <div><Input small label="Title" placeholder="Editor" required autoFocus value={f.title} onChange={(e) => set({ title: e.target.value })} /><FieldError msg={errors.title} /></div>
          <div>
            <div className="flex items-end gap-2">
              <div className="min-w-0 flex-1"><Input small label="Handle" required value={f.handle} onChange={(e) => set({ handle: e.target.value })} /></div>
              <Button icon="arrow-path" title="Regenerate from title" aria-label="Regenerate from title" onClick={() => set({ handle: slug(f.title, '_') })} />
            </div>
            <FieldError msg={errors.handle} />
          </div>
        </div>

        <div className="rounded-lg border border-red-200 bg-red-50 p-4 dark:border-red-900 dark:bg-red-950/30">
          <Switch label="Super Admin" description="Full access, bypasses every permission check below. When on, this role can do everything, including things granted to no other role." checked={f.superAdmin} onChange={setSuper} />
        </div>

        {!f.superAdmin && (
          <div className="space-y-4">
            <h3 className="text-base font-medium text-zinc-800 dark:text-white">Permissions</h3>
            {d.groups.map((g) => {
              const values = flat(g.permissions);
              return (
                <div key={g.group} className="rounded-lg border border-zinc-200 p-4 dark:border-white/10">
                  <div className="mb-2 flex items-center justify-between">
                    <p className="text-sm font-medium text-zinc-500 dark:text-white/70">{g.group}</p>
                    <div className="flex gap-2">
                      <Button className="h-6 px-2 text-xs" onClick={() => checkAll(values)}>Check all</Button>
                      <Button variant="ghost" className="h-6 px-2 text-xs" onClick={() => uncheckAll(values)}>Uncheck all</Button>
                    </div>
                  </div>
                  <div className="flex flex-col gap-4">
                    {g.permissions.map((p) => (
                      <div key={p.value} className="contents">
                        {row(p)}
                        {p.children.length > 0 && <div className="mb-10 ms-6 space-y-3 last:mb-0 dark:border-white/10">{p.children.map(row)}</div>}
                      </div>
                    ))}
                  </div>
                </div>
              );
            })}
          </div>
        )}

        <div className="flex justify-end gap-2">
          <a href={`${cp}/roles`}><Button variant="filled">Cancel</Button></a>
          <Button variant="primary" type="submit">Save</Button>
        </div>
      </form>

      <Toasts toasts={toasts} />
    </section>
  );
}
