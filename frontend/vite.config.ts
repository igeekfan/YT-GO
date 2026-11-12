import {defineConfig} from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
  server: {
    // Only enable API proxy when VITE_API_TARGET is set (web mode dev).
    // In `wails dev` mode, Wails handles the Go backend directly — no proxy needed.
    proxy: process.env.VITE_API_TARGET ? {
      '/api': {
        target: process.env.VITE_API_TARGET,
        changeOrigin: true,
      },
    } : undefined,
  },
})
