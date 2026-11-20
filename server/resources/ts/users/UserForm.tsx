// Padanan UserForm + user-form.blade.php (tambah & ubah user).
import { useEffect, useState } from 'react';
import { Checkbox, Radio, useToasts } from '../shared/flux';
import { Button, Input, Text, Toasts } from '../shared/ui';
import { ApiError, call, visit } from '../navigation/api';
import { FieldError, FormHeader, icons, saveShortcut } from './parts';

interface Option { handle: string; title: string }
interface Data { user?: { id: string; name: string; email: string }; roles: Option[]; groups: Option[]; roleHandles: string[]; groupHandles: string[]; canChangePassword: boolean }

const toggle = (list: string[], v: string, on: boolean) => (on ? [...list, v] : list.filter((x) => x !== v));

export function UserForm({ cp, id }: { cp: string; id: string }) {
  const { toasts, toast } = useToasts();
  const [d, setD] = useState<Data | null>(null);
  const [f, setF] = useState({ name: '', email: '', password: '', passwordMode: 'admin', roleHandles: [] as string[], groupHandles: [] as string[] });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const set = (p: Partial<typeof f>) => setF((x) => ({ ...x, ...p }));

  useEffect(() => {
    call<Data>(`${cp}/users/api/form?id=${encodeURIComponent(id)}`).then((r) => {
      setD(r);
      set({ name: r.user?.name ?? '', email: r.user?.email ?? '', roleHandles: r.roleHandles, groupHandles: r.groupHandles });
    }).catch((e) => toast(e.message, true));
  }, []);

  const save = async () => {
    setErrors({});
    try { await call(`${cp}/users/api/save`, { id, ...f }); visit(`${cp}/users`); } // pesan sukses dikirim server sebagai flash
    catch (e) {
      if (e instanceof ApiError && Object.keys(e.errors ?? {}).length) setErrors(e.errors);
      else toast(e instanceof Error ? e.message : String(e), true);
    }
  };
  useEffect(() => saveShortcut(save));

  if (!d) return <Toasts toasts={toasts} />;
  const editing = !!id;
  return (
    <section className="w-full max-w-3xl space-y-6">
      <FormHeader icon={icons.userBack} back={`${cp}/users`} title={editing ? 'Edit user' : 'Add user'} />

      <form className="space-y-6" onSubmit={(e) => { e.preventDefault(); save(); }}>
        <div className="grid gap-4">
          <div><Input label="Name" placeholder="Jane Doe" required autoFocus value={f.name} onChange={(e) => set({ name: e.target.value })} /><FieldError msg={errors.name} /></div>
          <div><Input label="Email" type="email" placeholder="jane@example.com" required value={f.email} onChange={(e) => set({ email: e.target.value })} /><FieldError msg={errors.email} /></div>
        </div>

        {!editing ? (
          <>
            <div className="space-y-3">
              <p className="text-sm font-medium text-zinc-800 dark:text-white">Password</p>
              <Radio label="Set password now" checked={f.passwordMode === 'admin'} onChange={() => set({ passwordMode: 'admin' })} />
              <Radio label="Email the user a link to set their own password" checked={f.passwordMode === 'reset_link'} onChange={() => set({ passwordMode: 'reset_link' })} />
            </div>
            {f.passwordMode === 'admin'
              ? <div><Input label="Password" type="password" placeholder="••••••••" required value={f.password} onChange={(e) => set({ password: e.target.value })} /><FieldError msg={errors.password} /></div>
              : <Text>A password-reset link will be emailed to them once saved — they set their own password from there.</Text>}
          </>
        ) : d.canChangePassword && (
          <div><Input label="Password" type="password" description="Leave blank to keep the current password." placeholder="••••••••" value={f.password} onChange={(e) => set({ password: e.target.value })} /><FieldError msg={errors.password} /></div>
        )}

        <div className="space-y-3">
          <p className="text-sm font-medium text-zinc-800 dark:text-white">Roles</p>
          {d.roles.length === 0 && <Text>No roles yet.</Text>}
          {d.roles.map((r) => <Checkbox key={r.handle} label={r.title} checked={f.roleHandles.includes(r.handle)} onChange={(v) => set({ roleHandles: toggle(f.roleHandles, r.handle, v) })} />)}
          <FieldError msg={errors.roleHandles} />
        </div>

        <div className="space-y-3">
          <p className="text-sm font-medium text-zinc-800 dark:text-white">User groups</p>
          {d.groups.length === 0 && <Text>No user groups yet.</Text>}
          {d.groups.map((g) => <Checkbox key={g.handle} label={g.title} checked={f.groupHandles.includes(g.handle)} onChange={(v) => set({ groupHandles: toggle(f.groupHandles, g.handle, v) })} />)}
          <FieldError msg={errors.groupHandles} />
        </div>

        <div className="flex justify-end gap-2">
          <a href={`${cp}/users`}><Button variant="filled">Cancel</Button></a>
          <Button variant="primary" type="submit">Save</Button>
        </div>
      </form>

      <Toasts toasts={toasts} />
    </section>
  );
}
