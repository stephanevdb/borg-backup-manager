import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:5001',
      '/auth': 'http://localhost:5001',
      '/health': 'http://localhost:5001',
      '/metrics': 'http://localhost:5001',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
