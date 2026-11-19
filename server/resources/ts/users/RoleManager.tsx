// Padanan RoleManager + role-manager.blade.php.
import { useEffect, useState } from 'react';
import { Badge, Cell, Column, ContextMenu, Menu, MenuItem, Table, useToasts, type MenuAction } from '../shared/flux';
import { Button, Heading, Modal, ModalActions, Text, Toasts } from '../shared/ui';
import { call, visit } from '../navigation/api';
import { icons, ListHeader, TableCard } from './parts';

interface Role { id: number; handle: string; title: string; super: boolean; permissions: number; users: number; groups: number }

export function RoleManager({ cp }: { cp: string }) {
  const api = `${cp}/roles/api`;
  const { toasts, toast } = useToasts();
  const [roles, setRoles] = useState<Role[] | null>(null);
  const [del, setDel] = useState<number | null>(null);
  const [menu, setMenu] = useState<{ id: number; x: number; y: number } | null>(null);

  const load = () => call<{ roles: Role[] }>(`${api}/list`).then((d) => setRoles(d.roles)).catch((e) => toast(e.message, true));
  useEffect(() => { load(); }, []);

  const remove = async () => {
    const id = del!; setDel(null);
    try { await call(`${api}/delete`, { id }); toast('Role deleted.'); } catch (e) { toast(e instanceof Error ? e.message : String(e), true); }
    load();
  };
  const edit = (r: Role) => visit(`${cp}/roles/${encodeURIComponent(r.handle)}`);
  const actions = (r: Role): MenuAction[] => [{ label: 'Edit', run: () => edit(r) }, { label: 'Delete', danger: true, run: () => setDel(r.id) }];

  if (!roles) return null;
  return (
    <section className="w-full space-y-6" onClick={() => setMenu(null)}>
      <ListHeader icon={icons.roles} title="Roles" action={{ label: 'Add role', href: `${cp}/roles/create` }} />

      <TableCard>
        <Table>
          <thead className="bg-zinc-100 dark:bg-zinc-900">
            <tr><Column>Title</Column><Column>Handle</Column><Column>Permissions</Column><Column>Users</Column><Column>Groups</Column><Column className="w-32" /></tr>
          </thead>
          <tbody>
            {roles.length === 0 && <tr><Cell colSpan={6}>No roles yet.</Cell></tr>}
            {roles.map((r) => (
              <tr key={r.id} onContextMenu={(e) => { e.preventDefault(); e.stopPropagation(); setMenu({ id: r.id, x: e.clientX - 20, y: e.clientY + 10 }); }}>
                <Cell><a className="hover:text-orange-700" href={`${cp}/roles/${encodeURIComponent(r.handle)}`}>{r.title}</a></Cell>
                <Cell>{r.handle}</Cell>
                <Cell>{r.super ? <Badge color="red">Super Admin</Badge> : r.permissions}</Cell>
                <Cell>{r.users}</Cell>
                <Cell>{r.groups}</Cell>
                <Cell>
                  <div className="flex justify-end gap-2">
                    <Menu trigger={<Button variant="ghost" icon="ellipsis-vertical" aria-label="Actions" />}>
                      <MenuItem icon="pencil" onClick={() => edit(r)}>Edit</MenuItem>
                      <MenuItem icon="trash" danger onClick={() => setDel(r.id)}>Delete</MenuItem>
                    </Menu>
                  </div>
                </Cell>
              </tr>
            ))}
          </tbody>
        </Table>
      </TableCard>

      {menu && <ContextMenu x={menu.x} y={menu.y} actions={actions(roles.find((x) => x.id === menu.id)!)} close={() => setMenu(null)} />}

      <Modal open={del !== null} onClose={() => setDel(null)} className="min-w-[24rem]">
        <div className="space-y-4">
          <Heading className="text-lg">Delete this role?</Heading>
          <Text>This cannot be undone.</Text>
          <ModalActions><Button variant="filled" onClick={() => setDel(null)}>Cancel</Button><Button variant="danger" onClick={remove}>Delete</Button></ModalActions>
        </div>
      </Modal>

      <Toasts toasts={toasts} />
    </section>
  );
}
