import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const apiProxyTarget = process.env.VITE_API_PROXY_TARGET || 'http://localhost:9080'
const devServerHost = process.env.VITE_DEV_SERVER_HOST || '0.0.0.0'
const backendProxy = {
  target: apiProxyTarget,
  changeOrigin: true,
}

function appManualChunks(id: string) {
  if (!id.includes('/node_modules/')) {
    return undefined
  }
  if (id.includes('/@uiw/react-md-editor/')) {
    return 'markdown-editor-vendor'
  }
  if (id.includes('/@uiw/react-markdown-preview/')) {
    return 'markdown-preview-vendor'
  }
  if (id.includes('/katex/')) {
    return 'katex-vendor'
  }
  if (
    id.includes('/react-markdown/') ||
    id.includes('/remark-') ||
    id.includes('/rehype-') ||
    id.includes('/unified/') ||
    id.includes('/micromark') ||
    id.includes('/mdast-util') ||
    id.includes('/hast-util') ||
    id.includes('/unist-util') ||
    id.includes('/vfile') ||
    id.includes('/property-information/') ||
    id.includes('/space-separated-tokens/') ||
    id.includes('/comma-separated-tokens/')
  ) {
    return 'markdown-pipeline-vendor'
  }
  if (id.includes('/prismjs/') || id.includes('/refractor/')) {
    return 'markdown-highlight-vendor'
  }
  if (id.includes('/@rc-component/picker/')) {
    return 'antd-picker-vendor'
  }
  if (id.includes('/@ant-design/icons')) {
    return 'antd-icons-vendor'
  }
  if (id.includes('/@rc-component/') || id.includes('/rc-')) {
    return 'antd-rc-vendor'
  }
  if (id.includes('/dayjs/')) {
    return 'dayjs-vendor'
  }
  if (id.includes('/antd/es/date-picker/') || id.includes('/antd/es/time-picker/') || id.includes('/antd/es/calendar/')) {
    return 'antd-picker-components-vendor'
  }
  if (id.includes('/antd/es/button/')) {
    return 'antd-button-vendor'
  }
  if (id.includes('/antd/es/alert/')) {
    return 'antd-alert-vendor'
  }
  if (id.includes('/antd/es/config-provider/') || id.includes('/antd/es/theme/')) {
    return 'antd-config-vendor'
  }
  if (id.includes('/antd/es/_util/') || id.includes('/antd/es/style/') || id.includes('/antd/es/locale/')) {
    return 'antd-shared-vendor'
  }
  if (id.includes('/antd/')) {
    return 'antd-misc-vendor'
  }
  if (id.includes('/react/') || id.includes('/react-dom/') || id.includes('/react-router-dom/')) {
    return 'react-vendor'
  }
  return undefined
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    rollupOptions: {
      output: {
        manualChunks: appManualChunks,
        codeSplitting: true,
      },
    },
  },
  server: {
    host: devServerHost,
    proxy: {
      '/api': backendProxy,
      '/uploads': backendProxy,
    },
  },
})
