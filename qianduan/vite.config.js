import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    host: true,
    proxy: {
      // 队友通过手机热点访问 http://<本机IP>:5173 时，/api 与 /uploads 请求
      // 统一由 vite 代理到本机后端，前端不再需要写死后端地址
      // 可用环境变量 API_PROXY 覆盖目标（如本地评测测试后端）
      '/api': { target: process.env.API_PROXY || 'http://localhost:8080', changeOrigin: true },
      '/uploads': { target: process.env.API_PROXY || 'http://localhost:8080', changeOrigin: true },
    },
  },
})
