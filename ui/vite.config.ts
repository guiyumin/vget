import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/health': 'http://localhost:8080',
      '/download': 'http://localhost:8080',
      '/status': 'http://localhost:8080',
      '/jobs': 'http://localhost:8080',
    },
  },
})
