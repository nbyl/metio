import { defineConfig } from 'vite'
import { transformWithOxc } from 'vite:oxc'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  ...defineConfig().plugins,
  plugins: [
    ...defineConfig().plugins,
    react(),
    tailwindcss(),
  ],
  test: {
    transform: {
      // Use the default SWC transformer instead of oxc for test files
      // or configure oxc appropriately
    },
  },
}
