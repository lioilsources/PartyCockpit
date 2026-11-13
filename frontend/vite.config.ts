import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

const backend = process.env.PC_BACKEND ?? 'http://localhost:8098';

export default defineConfig({
  plugins: [svelte()],
  server: {
    host: true,
    proxy: {
      '/api': backend,
      '/files': backend,
      '/webtoon': backend,
      '/stylemaps': backend,
      '/tributes': backend,
      '/glyphs': backend,
      '/ws': { target: backend.replace(/^http/, 'ws'), ws: true },
    },
  },
  build: { outDir: 'dist', emptyOutDir: true, chunkSizeWarningLimit: 1200 },
});
