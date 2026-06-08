import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const apiProxyTarget = process.env.VITE_API_PROXY_TARGET || 'http://localhost:9080'
const devServerHost = process.env.VITE_DEV_SERVER_HOST || '0.0.0.0'
const backendProxy = {
  target: apiProxyTarget,
  changeOrigin: true,
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    host: devServerHost,
    proxy: {
      '/api': backendProxy,
      '/uploads': backendProxy,
    },
  },
})
