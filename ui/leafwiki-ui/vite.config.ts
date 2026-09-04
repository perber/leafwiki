import { execSync } from 'child_process'
import react from '@vitejs/plugin-react'
import path from 'path'
import { defineConfig, type ConfigEnv } from 'vite'

// Mirrors scripts/resolve-version.sh's own env-var-override branch before
// falling back to invoking the script. This isn't just an optimization: the
// Docker frontend-build stage only COPYs ./ui/leafwiki-ui/, so the repo-root
// scripts/ directory (and even bash, on node:*-alpine) generally isn't
// present there. Every real build always sets APP_VERSION via --build-arg,
// so checking it first keeps that path working without depending on the
// script being reachable at all; the script invocation remains as the local
// `npm run build`/`npm run dev` fallback (git describe), where the full repo
// and a real shell are available.
export function resolveAppVersion(): string {
  const envVersion = process.env.APP_VERSION?.trim()
  if (envVersion) {
    return envVersion
  }

  try {
    return execSync('./scripts/resolve-version.sh', {
      cwd: path.resolve(__dirname, '../..'),
      encoding: 'utf-8',
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim()
  } catch {
    return 'v0.1.0'
  }
}

function manualChunks(id: string): string | undefined {
  const normalizedId = id.replaceAll('\\', '/')

  if (
    normalizedId.includes('/node_modules/@codemirror/') ||
    normalizedId.includes('/node_modules/@lezer/') ||
    normalizedId.includes('/node_modules/@fsegurai/')
  ) {
    return 'codemirror'
  }

  return undefined
}

// https://vite.dev/config/
export default defineConfig(({ command }: ConfigEnv) => ({
  // Relative base so asset paths in the built HTML are ./static/... instead of /static/...
  // Go rewrites these references to absolute paths at serve time so they resolve correctly
  // when index.html is served for deep SPA routes. Lazy chunks resolve via import.meta.url
  // and therefore work correctly under any sub-path without further server-side patching.
  base: command === 'build' ? './' : '/',
  define: {
    __APP_VERSION__: JSON.stringify(resolveAppVersion()),
  },
  plugins: [react()],
  server: {
    proxy: {
      '/assets': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false,
      },
      '/branding': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false,
      },
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false,
      },
    },
  },
  optimizeDeps: {
    include: ['mermaid', 'dagre-d3-es'],
  },
  build: {
    assetsDir: 'static', // <--- here you change the target directory
    rollupOptions: {
      output: {
        chunkFileNames: 'static/[name]-[hash].js',
        entryFileNames: 'static/[name]-[hash].js',
        assetFileNames: 'static/[name].[hash][extname]',
        manualChunks,
      },
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
}))
