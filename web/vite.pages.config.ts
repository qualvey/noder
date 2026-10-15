import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

// 独立的 Cloudflare Pages 构建入口，不影响 Go 服务使用的 static/ 构建。
export default defineConfig({
  root: fileURLToPath(new URL('./tools', import.meta.url)),
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  base: './',
  publicDir: fileURLToPath(new URL('./pages-public', import.meta.url)),
  build: {
    outDir: fileURLToPath(new URL('./pages-dist', import.meta.url)),
    emptyOutDir: true,
  },
})
