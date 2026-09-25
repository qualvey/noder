import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

// 构建产物输出到后端 static/（后端挂载 /static 与 "/" 兜底）
export default defineConfig({
  plugins: [
    vue(),
    tailwindcss(),
  ],
  "resolve": {
    "alias": {
      '@': fileURLToPath(new URL('./src', import.meta.url)),

    }
  },
  // 相对路径 base：兼容根路径与子路径反代部署
  base: './',
  build: {
    outDir: '../static',
    emptyOutDir: true,
  },
  server: {
    port: 5273,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
      },
      '/sub': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
      },
      '/node/': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
      },
      '/dl': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
      },
    },
  },
})
