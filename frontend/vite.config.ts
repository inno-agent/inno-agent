import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import svgr from '@svgr/rollup'

export default defineConfig({
  plugins: [
    tanstackRouter({ routesDirectory: './src/routes' }),
    svgr(),
    react(),
  ],
})