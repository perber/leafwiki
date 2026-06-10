import fs from 'fs'
import { pipeline } from 'stream/promises'
import { createGzip } from 'zlib'
import { execSync } from 'child_process'
import react from '@vitejs/plugin-react'
import path from 'path'
import { defineConfig, type Plugin } from 'vite'

const packageJson = JSON.parse(
  fs.readFileSync(new URL('./package.json', import.meta.url), 'utf-8'),
) as { version: string }

function resolveAppVersion(): string {
  const envVersion = process.env.APP_VERSION?.trim()
  if (envVersion) {
    return envVersion
  }

  try {
    return execSync('git describe --tags --abbrev=0', {
      cwd: __dirname,
      encoding: 'utf-8',
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim()
  } catch {
    return packageJson.version
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

function gzipStaticPlugin(): Plugin {
  return {
    name: 'gzip-static',
    apply: 'build',
    async closeBundle() {
      const staticDir = path.join(__dirname, 'dist', 'static')
      if (!fs.existsSync(staticDir)) return

      const files = fs.readdirSync(staticDir).filter((f) => /\.(js|css)$/.test(f))

      await Promise.all(
        files.map(async (file) => {
          const src = path.join(staticDir, file)
          if (fs.statSync(src).size < 1024) return
          await pipeline(
            fs.createReadStream(src),
            createGzip({ level: 9 }),
            fs.createWriteStream(src + '.gz'),
          )
        }),
      )
    },
  }
}

// https://vite.dev/config/
export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(resolveAppVersion()),
  },
  plugins: [react(), gzipStaticPlugin()],
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
})
