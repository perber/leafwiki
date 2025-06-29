import react from '@vitejs/plugin-react'
import path from 'path'
import { defineConfig } from 'vite'

const isDev = process.env.NODE_ENV !== 'production'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    emptyOutDir: false, // optional: Verzeichnis vor dem Build leeren
    outDir: '../../dist', // optional: Ausgabeordner für den Build
    assetsDir: 'static', // <--- hier änderst du das Zielverzeichnis
    sourcemap: isDev, // optional: sourcemaps nur im Dev-Modus
    minify: !isDev, // optional: Minifizierung nur im Prod-Modus
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
