import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// Vite builds the SPA straight into the Go embed directory so `go build`
// picks it up via //go:embed.
export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: '../internal/ui/dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/ping': 'http://localhost:8080',
      '/ws': { target: 'ws://localhost:8080', ws: true },
    },
  },
})
