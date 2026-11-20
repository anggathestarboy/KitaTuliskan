// <collections-manager>, <collections-entries data-handle> dan <collections-entry data-handle data-id data-site>: custom element
// supaya React ikut siklus Turbo (mount saat masuk DOM, unmount saat body diganti), sama seperti taxonomies/main.tsx.
import { createRoot, type Root } from 'react-dom/client';
import { Entries } from './Entries';
import { EntryForm } from './EntryForm';
import { Manager } from './Manager';

const island = (tag: string, render: (el: HTMLElement) => React.ReactNode) =>
  customElements.get(tag) || customElements.define(tag, class extends HTMLElement {
    private root?: Root;
    connectedCallback() { this.root = createRoot(this); this.root.render(render(this)); }
    disconnectedCallback() { const r = this.root; this.root = undefined; queueMicrotask(() => r?.unmount()); }
  });

island('collections-manager', (el) => <Manager cp={el.dataset.cp ?? ''} />);
island('collections-entries', (el) => <Entries cp={el.dataset.cp ?? ''} handle={el.dataset.handle ?? ''} />);
island('collections-entry', (el) => <EntryForm cp={el.dataset.cp ?? ''} handle={el.dataset.handle ?? ''} id={el.dataset.id ?? ''} site={el.dataset.site ?? ''} />);
