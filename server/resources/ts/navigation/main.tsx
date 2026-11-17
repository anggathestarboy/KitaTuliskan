// <navigation-manager data-cp="/admin-panel"> dan <navigation-editor data-cp=".." data-handle="sidebar-admin">:
// custom element supaya React ikut siklus Turbo (mount saat masuk DOM, unmount saat body diganti).
import { createRoot, type Root } from 'react-dom/client';
import { Editor } from './Editor';
import { Manager } from './Manager';

const island = (tag: string, render: (el: HTMLElement) => React.ReactNode) =>
  customElements.get(tag) || customElements.define(tag, class extends HTMLElement {
    private root?: Root;
    connectedCallback() { this.root = createRoot(this); this.root.render(render(this)); }
    disconnectedCallback() { const r = this.root; this.root = undefined; queueMicrotask(() => r?.unmount()); }
  });

island('navigation-manager', (el) => <Manager cp={el.dataset.cp ?? ''} />);
island('navigation-editor', (el) => <Editor cp={el.dataset.cp ?? ''} handle={el.dataset.handle ?? ''} />);
