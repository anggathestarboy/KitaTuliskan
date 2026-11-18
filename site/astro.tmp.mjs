import { defineConfig } from 'astro/config';

import templateScope from './plugins/template-scope.mjs';
// SSR: halaman dirender per request dari data live Go, jadi entry baru langsung bisa dikunjungi tanpa restart/build ulang.
export default defineConfig({ output: 'server', vite: { plugins: [templateScope()] } });
