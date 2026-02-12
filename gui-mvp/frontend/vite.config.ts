import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Vite config: simple React + TS SPA.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist', // backend expects ../frontend/dist
  },
})
