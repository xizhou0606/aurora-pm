import { defineConfig } from 'tsup'

export default defineConfig({
  entry: ['src/index.ts'],
  format: ['esm'],
  platform: 'node',
  target: 'node24',
  bundle: true,
  noExternal: ['zod'],
  clean: true,
  banner: { js: '#!/usr/bin/env node' },
  external: ['mobilewright', '@mobilewright/protocol', 'mobilecli'],
})
