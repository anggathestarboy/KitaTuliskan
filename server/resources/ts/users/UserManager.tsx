// Padanan UserManager + user-manager.blade.php.
import { useEffect, useState } from 'react';
import { Badge, Cell, Column, ContextMenu, Menu, MenuItem, Table, useToasts, type MenuAction } from '../shared/flux';
import { Button, Heading, Input, Modal, ModalActions, Text, Toasts } from '../shared/ui';
import { call, visit } from '../navigation/api';
import { dash, icons, ListHeader, TableCard } from './parts';

interface User { id: string; name: string; email: string; banned: boolean; roles: string[]; groups: string[] }
interface Data { users: User[]; me: string; canCreate: boolean; canEdit: boolean; canReset: boolean; canBan: boolean; canDelete: boolean }

const count = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;
const box = 'rounded border-zinc-300 dark:border-zinc-600';
const SENT = 'We have emailed your password reset link.';

export function UserManager({ cp }: { cp: string }) {
  const api = `${cp}/users/api`;
  const { toasts, toast } = useToasts();
  const [d, setD] = useState<Data | null>(null);
  const [search, setSearch] = useState('');
  const [selected, setSelected] = useState<string[]>([]);
  const [del, setDel] = useState<string | null>(null);
  const [bulkDel, setBulkDel] = useState(false);
  const [menu, setMenu] = useState<{ id: string; x: number; y: number } | null>(null);

  const load = () => call<Data>(`${api}/list`).then(setD).catch((e) => toast(e.message, true));
  useEffect(() => { load(); }, []);
  if (!d) return null;

  // Pencarian = where name/email like %q%; mengubah pencarian mengosongkan centang.
  const q = search.toLowerCase();
  const users = d.users.filter((u) => !q || u.name.toLowerCase().includes(q) || u.email.toLowerCase().includes(q));
  const allChecked = selected.length > 0 && selected.length === users.length;
  const find = (id: string) => d.users.find((u) => u.id === id)!;

  // Menjalankan aksi lalu memuat ulang; error server (mis. "can't delete your own account") tampil sebagai toast merah.
  const run = async (path: string, body: unknown, ok: string) => {
    try { await call(`${api}/${path}`, body); toast(ok); } catch (e) { toast(e instanceof Error ? e.message : String(e), true); }
    load();
  };
  const bulk = async (action: 'delete' | 'ban' | 'unban') => {
    const n = selected.filter((id) => id !== d.me).length;
    const done = { delete: 'deleted', ban: 'banned', unban: 'unbanned' }[action];
    try { await call(`${api}/bulk`, { action, ids: selected }); toast(`${count(n, 'user', 'users')} ${done}.`); } catch (e) { toast(e instanceof Error ? e.message : String(e), true); }
    setSelected([]);
    load();
  };
  const remove = () => { const id = del!; setDel(null); setSelected((s) => s.filter((x) => x !== id)); run('delete', { id }, 'User deleted.'); };

  const actions = (u: User): MenuAction[] => {
    const self = u.id === d.me;
    return [
      d.canEdit && { label: 'Edit', run: () => visit(`${cp}/users/${u.id}`) },
      d.canReset && { label: 'Send password reset', run: () => run('reset', { id: u.id }, SENT) },
      !self && d.canBan && { label: u.banned ? 'Unban' : 'Ban', run: () => run('ban', { id: u.id, ban: !u.banned }, u.banned ? 'User unbanned.' : 'User banned.') },
      !self && d.canDelete && { label: 'Delete', danger: true, run: () => setDel(u.id) },
    ].filter(Boolean) as MenuAction[];
  };

  return (
    <section className="w-full" onClick={() => setMenu(null)}>
      <ListHeader icon={icons.users} title="Users" action={d.canCreate ? { label: 'Add user', href: `${cp}/users/create` } : undefined} />

      <div className="mt-3 mb-7 flex items-center justify-between gap-4">
        <div className="max-w-sm flex-1">
          <Input small icon="magnifying-glass" placeholder="Search by name or email…" value={search} onChange={(e) => { setSearch(e.target.value); setSelected([]); }} />
        </div>
        {selected.length > 0 && (
          <div className="flex items-end gap-2">
            <Text>{selected.length} selected</Text>
            {d.canBan && <><Button onClick={() => bulk('unban')}>Unban</Button><Button onClick={() => bulk('ban')}>Ban</Button></>}
            {d.canDelete && <Button variant="danger" onClick={() => setBulkDel(true)}>Delete</Button>}
          </div>
        )}
      </div>

      <TableCard>
        <Table>
          <thead className="bg-zinc-100 dark:bg-zinc-900">
            <tr>
              <Column className="w-10"><input type="checkbox" className={`${box} w-max`} checked={allChecked} onChange={() => setSelected(allChecked ? [] : users.map((u) => u.id))} /></Column>
              <Column className="w-[15%]">Name</Column><Column>Email</Column><Column>Status</Column><Column>Roles</Column><Column>Groups</Column><Column className="w-32" />
            </tr>
          </thead>
          <tbody>
            {users.length === 0 && <tr><Cell colSpan={7}>{search !== '' ? 'No users match your search.' : 'No users yet.'}</Cell></tr>}
            {users.map((u) => {
              const self = u.id === d.me;
              const acts = actions(u);
              return (
                <tr key={u.id} onContextMenu={(e) => { if (!acts.length) return; e.preventDefault(); e.stopPropagation(); setMenu({ id: u.id, x: e.clientX - 20, y: e.clientY + 10 }); }}>
                  <Cell>
                    {self
                      ? <input type="checkbox" disabled title="You can't select your own account." className={box} />
                      : <input type="checkbox" className={box} checked={selected.includes(u.id)} onChange={(e) => setSelected(e.target.checked ? [...selected, u.id] : selected.filter((x) => x !== u.id))} />}
                  </Cell>
                  <Cell className="w-[15%]"><a className="hover:text-orange-700" href={`${cp}/users/${u.id}`}>{u.name}</a></Cell>
                  <Cell>{u.email}</Cell>
                  <Cell>{u.banned ? <Badge color="red">Banned</Badge> : <Badge color="green">Active</Badge>}</Cell>
                  <Cell><div className="flex flex-wrap gap-1">{u.roles.length ? u.roles.map((r, i) => <Badge key={i}>{r}</Badge>) : dash}</div></Cell>
                  <Cell><div className="flex flex-wrap gap-1">{u.groups.length ? u.groups.map((g, i) => <Badge key={i}>{g}</Badge>) : dash}</div></Cell>
                  <Cell>
                    <div className="flex w-full justify-end">
                      {acts.length > 0 && (
                        <Menu trigger={<Button variant="ghost" icon="ellipsis-vertical" aria-label="Actions" />}>
                          {d.canEdit && <MenuItem icon="pencil" onClick={() => visit(`${cp}/users/${u.id}`)}>Edit</MenuItem>}
                          {d.canReset && <MenuItem icon="key" onClick={() => run('reset', { id: u.id }, SENT)}>Send password reset</MenuItem>}
                          {!self && d.canBan && (u.banned
                            ? <MenuItem icon="arrow-path" onClick={() => run('ban', { id: u.id, ban: false }, 'User unbanned.')}>Unban</MenuItem>
                            : <MenuItem icon="hand-raised" onClick={() => run('ban', { id: u.id, ban: true }, 'User banned.')}>Ban</MenuItem>)}
                          {!self && d.canDelete && <MenuItem icon="trash" danger onClick={() => setDel(u.id)}>Delete</MenuItem>}
                        </Menu>
                      )}
                    </div>
                  </Cell>
                </tr>
              );
            })}
          </tbody>
        </Table>
      </TableCard>

      {menu && <ContextMenu x={menu.x} y={menu.y} actions={actions(find(menu.id))} close={() => setMenu(null)} />}

      <Modal open={del !== null} onClose={() => setDel(null)} className="min-w-[24rem]">
        <div className="space-y-4">
          <Heading className="text-lg">Delete this user?</Heading>
          <Text>This cannot be undone.</Text>
          <ModalActions><Button variant="filled" onClick={() => setDel(null)}>Cancel</Button><Button variant="danger" onClick={remove}>Delete</Button></ModalActions>
        </div>
      </Modal>

      <Modal open={bulkDel} onClose={() => setBulkDel(false)} className="min-w-[24rem]">
        <div className="space-y-4">
          <Heading className="text-lg">Delete {selected.length} user(s)?</Heading>
          <Text>This cannot be undone. Your own account, if selected, is skipped.</Text>
          <ModalActions><Button variant="filled" onClick={() => setBulkDel(false)}>Cancel</Button><Button variant="danger" onClick={() => { setBulkDel(false); bulk('delete'); }}>Delete</Button></ModalActions>
        </div>
      </Modal>

      <Toasts toasts={toasts} />
    </section>
  );
}
