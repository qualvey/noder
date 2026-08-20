import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
// 构建产物输出到后端 static/（后端挂载 /static 与 "/" 兜底）
export default defineConfig({
  plugins: [
    vue()
  ],
  // 相对路径 base：兼容根路径与子路径反代部署
  base: './',
  build: {
    outDir: '../static',
    emptyOutDir: true,
  },
  server: {
    port: 5273,
    proxy: {
      '/api': 'http://127.0.0.1:8000',
      '/sub': 'http://127.0.0.1:8000',
      '/node': 'http://127.0.0.1:8000',
      '/dl': 'http://127.0.0.1:8000',
    },
  },
})
