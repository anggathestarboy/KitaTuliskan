// <globals-manager data-cp="/admin-panel"> dan <globals-editor data-cp=".." data-handle="..">: custom element supaya React
// ikut siklus Turbo (mount saat masuk DOM, unmount saat body diganti), sama seperti navigation/main.tsx.
import { createRoot, type Root } from 'react-dom/client';
import { Content } from '../navigation/Content';
import { Manager } from './Manager';

const island = (tag: string, render: (el: HTMLElement) => React.ReactNode) =>
  customElements.get(tag) || customElements.define(tag, class extends HTMLElement {
    private root?: Root;
    connectedCallback() { this.root = createRoot(this); this.root.render(render(this)); }
    disconnectedCallback() { const r = this.root; this.root = undefined; queueMicrotask(() => r?.unmount()); }
  });

island('globals-manager', (el) => <Manager cp={el.dataset.cp ?? ''} />);
island('globals-editor', (el) => <Content cp={el.dataset.cp ?? ''} handle={el.dataset.handle ?? ''} scope="globals" noun="Global set" />);
