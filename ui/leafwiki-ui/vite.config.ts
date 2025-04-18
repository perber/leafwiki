import react from '@vitejs/plugin-react'
import path from 'path'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/assets': 'http://localhost:8080', // dein Go-Backend
    },
  },
  build: {
    assetsDir: 'static', // <--- hier änderst du das Zielverzeichnis
    rollupOptions: {
      output: {
        assetFileNames: 'static/[name].[hash][extname]', // optional: für konsistente Benennung
      },
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
})
