// <media-library data-cp="/admin-panel">: custom element supaya React ikut siklus Turbo (mount saat masuk DOM, unmount saat body diganti).
import { createRoot, type Root } from 'react-dom/client';
import { App } from './App';

class MediaLibrary extends HTMLElement {
  private root?: Root;
  connectedCallback() {
    this.root = createRoot(this);
    this.root.render(<App cp={this.dataset.cp ?? ''} />);
  }
  disconnectedCallback() {
    const r = this.root;
    this.root = undefined;
    queueMicrotask(() => r?.unmount()); // unmount tidak boleh dipanggil sinkron saat React sedang render
  }
}
if (!customElements.get('media-library')) customElements.define('media-library', MediaLibrary);
