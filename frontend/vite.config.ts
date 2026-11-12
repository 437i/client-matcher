import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    // Dev-сервер отдаёт приложение с того же origin, что и API:
    // браузер обращается к /api/v1/... относительно текущего хоста,
    // а Vite проксирует запрос на локальный backend.
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    server: {
      deps: {
        // openapi-fetch обращается к глобальному fetch, поэтому библиотека
        // должна выполняться в том же контексте, где тест подменяет fetch.
        inline: ['openapi-fetch'],
      },
    },
  },
})
