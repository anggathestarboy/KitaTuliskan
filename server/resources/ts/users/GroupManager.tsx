// Padanan UserGroupManager + user-group-manager.blade.php (daftar + modal tambah/ubah).
import { useEffect, useState } from 'react';
import { Cell, Checkbox, Column, ContextMenu, Menu, MenuItem, Table, useToasts, Badge, type MenuAction } from '../shared/flux';
import { Button, Heading, Input, Modal, ModalActions, Text, Toasts } from '../shared/ui';
import { ApiError, call, slug } from '../navigation/api';
import { dash, FieldError, icons, ListHeader, saveShortcut, TableCard } from './parts';

interface Role { handle: string; title: string }
interface Group { id: number; handle: string; title: string; roles: Role[]; roleHandles: string[]; users: number }
interface Form { id: number; title: string; handle: string; roleHandles: string[] }

export function GroupManager({ cp }: { cp: string }) {
  const api = `${cp}/user-groups/api`;
  const { toasts, toast } = useToasts();
  const [groups, setGroups] = useState<Group[] | null>(null);
  const [roles, setRoles] = useState<Role[]>([]);
  const [form, setForm] = useState<Form | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [del, setDel] = useState<number | null>(null);
  const [menu, setMenu] = useState<{ id: number; x: number; y: number } | null>(null);

  const load = () => call<{ groups: Group[]; roles: Role[] }>(`${api}/list`).then((d) => { setGroups(d.groups); setRoles(d.roles); }).catch((e) => toast(e.message, true));
  useEffect(() => { load(); }, []);

  const open = (f: Form) => { setErrors({}); setForm(f); };
  const edit = (g: Group) => open({ id: g.id, title: g.title, handle: g.handle, roleHandles: g.roleHandles });

  const save = async () => {
    if (!form) return;
    setErrors({});
    try { await call(`${api}/save`, form); setForm(null); toast('User group saved.'); load(); }
    catch (e) {
      if (e instanceof ApiError && Object.keys(e.errors ?? {}).length) setErrors(e.errors);
      else toast(e instanceof Error ? e.message : String(e), true);
    }
  };
  useEffect(() => (form ? saveShortcut(save) : undefined)); // Ctrl+S hanya saat modal terbuka
  const remove = async () => {
    const id = del!; setDel(null);
    try { await call(`${api}/delete`, { id }); toast('User group deleted.'); } catch (e) { toast(e instanceof Error ? e.message : String(e), true); }
    load();
  };
  const actions = (g: Group): MenuAction[] => [{ label: 'Edit', run: () => edit(g) }, { label: 'Delete', danger: true, run: () => setDel(g.id) }];

  if (!groups) return null;
  return (
    <section className="w-full space-y-6" onClick={() => setMenu(null)}>
      <ListHeader icon={icons.groups} title="User Groups" action={{ label: 'Add group', onClick: () => open({ id: 0, title: '', handle: '', roleHandles: [] }) }} />

      <TableCard>
        <Table>
          <thead className="bg-zinc-100 dark:bg-zinc-900">
            <tr><Column>Title</Column><Column>Handle</Column><Column>Roles</Column><Column>Users</Column><Column className="w-32" /></tr>
          </thead>
          <tbody>
            {groups.length === 0 && <tr><Cell colSpan={5}>No user groups yet.</Cell></tr>}
            {groups.map((g) => (
              <tr key={g.id} onContextMenu={(e) => { e.preventDefault(); e.stopPropagation(); setMenu({ id: g.id, x: e.clientX - 20, y: e.clientY + 10 }); }}>
                <Cell><button type="button" className="cursor-pointer hover:text-orange-700" onClick={(e) => { e.stopPropagation(); edit(g); }}>{g.title}</button></Cell>
                <Cell>{g.handle}</Cell>
                <Cell><div className="flex flex-wrap gap-1">{g.roles.length ? g.roles.map((r) => <Badge key={r.handle}>{r.title}</Badge>) : dash}</div></Cell>
                <Cell>{g.users}</Cell>
                <Cell>
                  <div className="flex justify-end gap-2">
                    <Menu trigger={<Button variant="ghost" icon="ellipsis-vertical" aria-label="Actions" />}>
                      <MenuItem icon="pencil" onClick={() => edit(g)}>Edit</MenuItem>
                      <MenuItem icon="trash" danger onClick={() => setDel(g.id)}>Delete</MenuItem>
                    </Menu>
                  </div>
                </Cell>
              </tr>
            ))}
          </tbody>
        </Table>
      </TableCard>

      {menu && <ContextMenu x={menu.x} y={menu.y} actions={actions(groups.find((x) => x.id === menu.id)!)} close={() => setMenu(null)} />}

      <Modal open={!!form} onClose={() => setForm(null)} className="max-w-lg">
        <form className="space-y-6" onSubmit={(e) => { e.preventDefault(); save(); }}>
          <Heading className="text-lg">{form?.id ? 'Edit group' : 'Add group'}</Heading>
          <div><Input small label="Title" placeholder="Marketing" required autoFocus value={form?.title ?? ''} onChange={(e) => setForm({ ...form!, title: e.target.value })} /><FieldError msg={errors.title} /></div>
          <div>
            <div className="flex items-end gap-2">
              <div className="flex-1"><Input small label="Handle" required value={form?.handle ?? ''} onChange={(e) => setForm({ ...form!, handle: e.target.value })} /></div>
              <Button icon="arrow-path" title="Regenerate from title" aria-label="Regenerate from title" onClick={() => setForm({ ...form!, handle: slug(form!.title, '_') })} />
            </div>
            <FieldError msg={errors.handle} />
          </div>
          <div className="space-y-3">
            <p className="text-sm font-medium text-zinc-800 dark:text-white">Roles</p>
            {roles.length === 0 && <Text>No roles yet.</Text>}
            {form && roles.map((r) => (
              <Checkbox key={r.handle} label={r.title} checked={form.roleHandles.includes(r.handle)}
                onChange={(v) => setForm({ ...form, roleHandles: v ? [...form.roleHandles, r.handle] : form.roleHandles.filter((h) => h !== r.handle) })} />
            ))}
            <FieldError msg={errors.roleHandles} />
          </div>
          <ModalActions><Button variant="filled" onClick={() => setForm(null)}>Cancel</Button><Button variant="primary" type="submit">Save</Button></ModalActions>
        </form>
      </Modal>

      <Modal open={del !== null} onClose={() => setDel(null)} className="min-w-[24rem]">
        <div className="space-y-4">
          <Heading className="text-lg">Delete this group?</Heading>
          <Text>This cannot be undone.</Text>
          <ModalActions><Button variant="filled" onClick={() => setDel(null)}>Cancel</Button><Button variant="danger" onClick={remove}>Delete</Button></ModalActions>
        </div>
      </Modal>

      <Toasts toasts={toasts} />
    </section>
  );
}
