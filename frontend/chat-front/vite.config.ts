/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import svgr from 'vite-plugin-svgr'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  resolve: {
    alias: {
      '@libs': fileURLToPath(new URL('./libs', import.meta.url)),
      '@shared': fileURLToPath(new URL('./libs/shared', import.meta.url)),
      '@images': fileURLToPath(new URL('./libs/shared/images', import.meta.url)),
    },
  },
  plugins: [
    tanstackRouter({
      routesDirectory: './projects/app/routes',
      generatedRouteTree: './projects/app/routeTree.gen.ts',
    }),
    svgr(),
    tailwindcss(),
    react(),
  ],
  test: {
    environment: 'jsdom',
    setupFiles: ['./projects/app/test/setup.ts'],
  },
})
