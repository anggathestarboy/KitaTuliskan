import { defineConfig } from 'astro/config';
import node from '@astrojs/node';
// SSR: halaman dirender per request dari data live Go, jadi entry baru langsung bisa dikunjungi tanpa restart/build ulang.
export default defineConfig({ output: 'server', adapter: node({ mode: 'standalone' }) });
