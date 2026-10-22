import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  // The shared/ folder (API client + types) lives outside web/.
  server: { fs: { allow: ['..'] } },
});
