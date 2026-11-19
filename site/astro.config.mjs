import { defineConfig } from 'astro/config';
import node from '@astrojs/node';
import { loadEnv } from 'vite';
import templateScope from './plugins/template-scope.mjs';
import cmsTypes from './plugins/cms-types.mjs';
// SSR: halaman dirender per request dari data live Go, jadi entry baru langsung bisa dikunjungi tanpa restart/build ulang.
export default defineConfig({ output: 'server', adapter: node({ mode: 'standalone' }), vite: { plugins: [templateScope(), cmsTypes(loadEnv('', process.cwd(), '').CMS_URL)] } });
