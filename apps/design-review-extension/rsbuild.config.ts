import { defineConfig } from '@rsbuild/core'
import { pluginReact } from '@rsbuild/plugin-react'

export default defineConfig({
  plugins: [pluginReact()],
  source: { entry: { popup: './src/popup.tsx' } },
  html: { template: './src/popup.html', title: 'PM 设计走查采集器' },
  output: { distPath: { root: 'dist' }, assetPrefix: './' },
  performance: { chunkSplit: { strategy: 'all-in-one' } },
})
