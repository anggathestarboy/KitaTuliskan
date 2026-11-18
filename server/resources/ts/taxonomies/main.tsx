// <taxonomies-manager>, <taxonomies-terms data-handle> dan <taxonomies-term data-handle data-id data-site>: custom element
// supaya React ikut siklus Turbo (mount saat masuk DOM, unmount saat body diganti), sama seperti globals/main.tsx.
import { createRoot, type Root } from 'react-dom/client';
import { Content } from '../navigation/Content';
import { Manager } from './Manager';
import { Terms } from './Terms';

const island = (tag: string, render: (el: HTMLElement) => React.ReactNode) =>
  customElements.get(tag) || customElements.define(tag, class extends HTMLElement {
    private root?: Root;
    connectedCallback() { this.root = createRoot(this); this.root.render(render(this)); }
    disconnectedCallback() { const r = this.root; this.root = undefined; queueMicrotask(() => r?.unmount()); }
  });

island('taxonomies-manager', (el) => <Manager cp={el.dataset.cp ?? ''} />);
island('taxonomies-terms', (el) => <Terms cp={el.dataset.cp ?? ''} handle={el.dataset.handle ?? ''} />);
island('taxonomies-term', (el) => <Content cp={el.dataset.cp ?? ''} handle={el.dataset.handle ?? ''} scope="taxonomies" noun="Term" term={{ id: Number(el.dataset.id) || 0, site: el.dataset.site ?? '' }} />);
