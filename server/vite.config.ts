import { defineConfig } from 'vite';
import tailwindcss from '@tailwindcss/vite';

// Dev: Go merender <script src="http://localhost:5173/...">; Prod: Go membaca static/build/.vite/manifest.json.
export default defineConfig({
  plugins: [tailwindcss()],
  esbuild: { jsx: 'automatic' }, // React tanpa plugin: tidak perlu preamble HMR di template Go (dev = full reload)
  base: '/static/build/',
  build: { outDir: 'static/build', manifest: true, emptyOutDir: true, rollupOptions: { input: ['resources/css/app.css', 'resources/ts/app.ts', 'resources/ts/media/main.tsx', 'resources/ts/navigation/main.tsx'] } },
  server: { port: 5173, strictPort: true, origin: 'http://localhost:5173', cors: true },
});
