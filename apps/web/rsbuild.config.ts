import { defineConfig, loadEnv } from '@rsbuild/core'
import { pluginReact } from '@rsbuild/plugin-react'
import { pluginTailwindcss } from '@rsbuild/plugin-tailwindcss'
import { readFileSync } from 'node:fs'
import { fileURLToPath, URL } from 'node:url'

const packageVersion = JSON.parse(
  readFileSync(fileURLToPath(new URL('./package.json', import.meta.url)), 'utf8'),
).version as string

export default defineConfig(({ envMode }) => {
  loadEnv({ mode: envMode })

  const proxyTarget = (process.env.VITE_DEV_PROXY_TARGET || 'http://127.0.0.1:8080').replace(
    /\/+$/,
    '',
  )
  const hostAuthProxyTarget = (
    process.env.VITE_DEV_HOST_AUTH_PROXY_TARGET || proxyTarget
  ).replace(/\/+$/, '')
  const compilerEnabled = process.env.REACT_COMPILER !== 'off'
  return {
    plugins: [
      pluginReact({ reactCompiler: compilerEnabled }),
      pluginTailwindcss(),
    ],
  // Rsbuild 2.2 官方 API（原 performance.chunkSplit 已废弃，迁移配方见
  // rsbuild.rs/guide/upgrade/v1-to-v2）：preset 'default' 即原
  // split-by-experience；cacheGroups 等价原 forceSplitting。
  splitChunks: {
    preset: 'default',
    cacheGroups: {
      'pm-copy': {
        test: /[\\/]src[\\/]shared[\\/]copy[\\/]/,
        name: 'pm-copy',
        chunks: 'all',
        priority: 0,
        enforce: true,
      },
      'pm-icons': {
        test: /[\\/]src[\\/]shared[\\/]ui[\\/]icons[\\/]/,
        name: 'pm-icons',
        chunks: 'all',
        priority: 0,
        enforce: true,
      },
      'chart-renderer': {
        test: /node_modules[\\/]zrender[\\/]/,
        name: 'chart-renderer',
        chunks: 'all',
        priority: 0,
        enforce: true,
      },
      'landing-icons': {
        test: /node_modules[\\/]@phosphor-icons[\\/]/,
        name: 'landing-icons',
        chunks: 'all',
        priority: 0,
        enforce: true,
      },
      'landing-motion': {
        test: /node_modules[\\/](?:gsap|@gsap|lenis)[\\/]/,
        name: 'landing-motion',
        chunks: 'all',
        priority: 0,
        enforce: true,
      },
      'lucide-react': {
        test: /node_modules[\\/]lucide-react[\\/]/,
        name: 'lucide-react',
        chunks: 'all',
        priority: 0,
        enforce: true,
      },
      'base-ui': {
        test: /node_modules[\\/]@base-ui[\\/]react[\\/]/,
        name: 'base-ui',
        chunks: 'all',
        priority: 0,
        enforce: true,
      },
      'tiptap-core': {
        test: /node_modules[\\/]@tiptap[\\/](?:core|react)[\\/]/,
        name: 'tiptap-core',
        chunks: 'all',
        priority: 0,
        enforce: true,
      },
      'tiptap-extensions': {
        test: /node_modules[\\/]@tiptap[\\/](?:starter-kit|extension-|extensions)/,
        name: 'tiptap-extensions',
        chunks: 'all',
        priority: 0,
        enforce: true,
      },
      'prosemirror-model-state': {
        test: /node_modules[\\/]prosemirror-(?:model|state|transform)[\\/]/,
        name: 'prosemirror-model-state',
        chunks: 'all',
        priority: 0,
        enforce: true,
      },
      'prosemirror-view': {
        test: /node_modules[\\/]prosemirror-view[\\/]/,
        name: 'prosemirror-view',
        chunks: 'all',
        priority: 0,
        enforce: true,
      },
      'prosemirror-plugins': {
        test: /node_modules[\\/]prosemirror-(?:commands|history|inputrules|keymap|schema-list|dropcursor|gapcursor)[\\/]/,
        name: 'prosemirror-plugins',
        chunks: 'all',
        priority: 0,
        enforce: true,
      },
    },
  },
    source: {
      entry: {
        index: './src/main.tsx',
      },
      define: {
        'import.meta.env.BASE_URL': JSON.stringify('/'),
        'import.meta.env.VITE_PM_API_BASE_URL': JSON.stringify(
          process.env.VITE_PM_API_BASE_URL || '',
        ),
        'import.meta.env.VITE_PM_HOST_API_BASE_URL': JSON.stringify(
          process.env.VITE_PM_HOST_API_BASE_URL || '',
        ),
        'import.meta.env.VITE_PM_PRODUCT_NAME': JSON.stringify(
          process.env.VITE_PM_PRODUCT_NAME || '',
        ),
        'import.meta.env.VITE_PM_APP_VERSION': JSON.stringify(
          process.env.VITE_PM_APP_VERSION || packageVersion,
        ),
        'import.meta.env.VITE_PM_VERSION_SUMMARY': JSON.stringify(
          process.env.VITE_PM_VERSION_SUMMARY || '[]',
        ),
        'import.meta.env.VITE_PM_RELEASE_DATE': JSON.stringify(
          process.env.VITE_PM_RELEASE_DATE || '',
        ),
      },
    },
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),

      },
    },
    dev: {
      // Persistent editor tabs must not depend on a dev-server lazy-compilation
      // subscription surviving an API/frontend restart. Keep runtime code-splitting
      // but compile its chunks up front so old tabs cannot suspend indefinitely.
      lazyCompilation: false,
    },
    html: {
      template: './index.html',
    },
    output: {
      legalComments: 'none',
      assetPrefix: '/',
      minify: {
        jsOptions: {
          minimizerOptions: {
            compress: {
              passes: 2,
            },
          },
        },
      },
      distPath: {
        root: 'dist',
      },
    },
    performance: {
      // 官方 removeConsole（保留 error/warn 供线上诊断）：生产移除 log/info/debug
      removeConsole: { exclude: ['error', 'warn'] },
    },
    server: {
      host: '127.0.0.1',
      port: 5188,
      strictPort: true,
      cors: true,
      publicDir: {
        name: 'public',
        copyOnBuild: true,
      },
      proxy: {
        '/api/v1/core/workspaces': {
          target: hostAuthProxyTarget,
          changeOrigin: true,
          on: {
            proxyReq(proxyRequest) {
              proxyRequest.removeHeader('origin')
            },
          },
        },
        '/api/v1/core/auth/login': {
          target: hostAuthProxyTarget,
          changeOrigin: true,
          on: {
            proxyReq(proxyRequest) {
              proxyRequest.removeHeader('origin')
            },
          },
        },
        '/api/v1/core/auth/logout': {
          target: hostAuthProxyTarget,
          changeOrigin: true,
          on: {
            proxyReq(proxyRequest) {
              proxyRequest.removeHeader('origin')
            },
          },
        },
        '/api/v1/core/auth/me': {
          target: hostAuthProxyTarget,
          changeOrigin: true,
          on: {
            proxyReq(proxyRequest) {
              proxyRequest.removeHeader('origin')
            },
          },
        },
        // Public release announcements are served by pm-api at its root
        // (/release-notes); the browser calls /api/release-notes so the same
        // origin can also keep serving the /release-notes page route.
        '/api/release-notes': {
          target: proxyTarget,
          changeOrigin: true,
          pathRewrite: { '^/api/release-notes': '/release-notes' },
        },
        '/api': {
          target: proxyTarget,
          changeOrigin: true,
          ws: true,
        },
        '/health': {
          target: proxyTarget,
          changeOrigin: true,
        },
        '/.well-known': {
          target: proxyTarget,
          changeOrigin: true,
        },
      },
    },
  }
})
