import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// 开发时代理到 Go admin (默认 8080)
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/jobinfo': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/jobgroup': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/joblog': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/jobcode': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/index': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/auth': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/user': { target: 'http://127.0.0.1:8080', changeOrigin: true },
    },
  },
})
