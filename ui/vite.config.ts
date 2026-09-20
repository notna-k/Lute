import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 3000,
    // In the compose stack nginx serves the SPA and proxies /api to core, so
    // the app calls the API same-origin. Mirroring that here means `npm run
    // dev` talks to the dev stack with no VITE_API_URL and no CORS entry.
    proxy: {
      '/api': {
        target: process.env.DEV_API_TARGET ?? 'http://localhost:8080',
        changeOrigin: true,
        // The build log stream is a WebSocket on the same prefix.
        ws: true,
      },
    },
  },
  envDir: './',
});
