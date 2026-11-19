// <users-manager>, <users-form>, <roles-manager>, <roles-form>, <user-groups-manager>: custom element supaya React ikut siklus Turbo
// (mount saat masuk DOM, unmount saat body diganti), sama seperti globals/main.tsx.
import { createRoot, type Root } from 'react-dom/client';
import { GroupManager } from './GroupManager';
import { RoleForm } from './RoleForm';
import { RoleManager } from './RoleManager';
import { UserForm } from './UserForm';
import { UserManager } from './UserManager';

const island = (tag: string, render: (el: HTMLElement) => React.ReactNode) =>
  customElements.get(tag) || customElements.define(tag, class extends HTMLElement {
    private root?: Root;
    connectedCallback() { this.root = createRoot(this); this.root.render(render(this)); }
    disconnectedCallback() { const r = this.root; this.root = undefined; queueMicrotask(() => r?.unmount()); }
  });

const cp = (el: HTMLElement) => el.dataset.cp ?? '';
island('users-manager', (el) => <UserManager cp={cp(el)} />);
island('users-form', (el) => <UserForm cp={cp(el)} id={el.dataset.id ?? ''} />);
island('roles-manager', (el) => <RoleManager cp={cp(el)} />);
island('roles-form', (el) => <RoleForm cp={cp(el)} handle={el.dataset.handle ?? ''} />);
island('user-groups-manager', (el) => <GroupManager cp={cp(el)} />);
