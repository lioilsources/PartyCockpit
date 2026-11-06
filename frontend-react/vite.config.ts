import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

// The React Cockpit (D21) shares transport and types with the Svelte app:
// `@pc/lib/*` is frontend/src/lib. Everything React lives in this folder.
const shared = fileURLToPath(new URL('../frontend/src', import.meta.url));
const backend = process.env.PC_BACKEND ?? 'http://localhost:8098';

export default defineConfig({
  base: '/react/',
  plugins: [react()],
  resolve: { alias: { '@pc': shared } },
  server: {
    port: 5174,
    host: true,
    fs: { allow: ['.', shared] },
    proxy: {
      '/api': backend,
      '/files': backend,
      '/webtoon': backend,
      '/stylemaps': backend,
      '/glyphs': backend,
      '/ws': { target: backend.replace(/^http/, 'ws'), ws: true },
    },
  },
  build: { outDir: 'dist', emptyOutDir: true },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: { modules: { classNameStrategy: 'non-scoped' } },
  },
});
