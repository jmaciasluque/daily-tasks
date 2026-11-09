import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

const configDir = fileURLToPath(new URL('.', import.meta.url));
const appVersion = readFileSync(resolve(configDir, '../VERSION'), 'utf8').trim();

// Two builds from one codebase:
//   npm run build          → CLI-served UI (talks to `daily-tasks web` on localhost)
//   npm run build:hosted   → hosted UI served by the sync server at /app/
const hosted = process.env.VITE_HOSTED === '1';

export default defineConfig({
  base: hosted ? '/app/' : '/',
  define: {
    __APP_VERSION__: JSON.stringify(appVersion),
  },
  resolve: {
    alias: {
      // Platform-neutral sync/history/Connect code shared with the mobile app.
      '@mobile': resolve(configDir, '../mobile/src'),
    },
    // Shared files live under mobile/; make their package imports resolve to
    // this project's node_modules.
    dedupe: ['@bufbuild/protobuf', '@connectrpc/connect', '@connectrpc/connect-web', 'base-64'],
  },
  build: {
    outDir: hosted ? 'dist-hosted' : 'dist',
    emptyOutDir: true,
  },
  plugins: [react()],
  server: {
    proxy: hosted
      ? {
          '/auth': { target: 'http://127.0.0.1:8080', changeOrigin: true },
          '/dailytasks.v1.': { target: 'http://127.0.0.1:8080', changeOrigin: true },
        }
      : {
          '/api': { target: 'http://127.0.0.1:8421', changeOrigin: true },
        },
  },
  test: {
    environment: 'jsdom',
  },
});
