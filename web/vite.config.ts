import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// The production build is written into the Go module (api/webdist), which
// embeds it into the server binary and the Android library. During
// development, `npm run dev` proxies /api to a locally running Go server
// (`go run ./cmd/career107`).
export default defineConfig({
  plugins: [react()],
  base: './',
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: { '/api': 'http://127.0.0.1:8080' },
  },
  build: {
    target: 'es2020',
    outDir: '../api/webdist',
    emptyOutDir: true,
    sourcemap: false,
  },
});
