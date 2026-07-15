import { readdirSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// pdf.js necesita ficheros auxiliares en runtime que NO entran por import: los
// decodificadores wasm (JPEG2000/JBIG2 de escaneos antiguos, ICC), los
// CMaps de codificación de texto y las fuentes estándar no embebidas. Sin ellos las
// páginas escaneadas salen EN BLANCO. Se copian de pdfjs-dist a la build bajo
// /pdfjs/ (PdfReader les pasa esa ruta a getDocument) y en dev se sirven directo
// de node_modules.
const PDFJS_DIRS = ['wasm', 'cmaps', 'standard_fonts']
function pdfjsAssets() {
  const root = fileURLToPath(new URL('./node_modules/pdfjs-dist/', import.meta.url))
  return {
    name: 'pdfjs-assets',
    generateBundle() {
      for (const dir of PDFJS_DIRS) {
        for (const f of readdirSync(root + dir)) {
          if (f.startsWith('LICENSE')) continue
          this.emitFile({ type: 'asset', fileName: `pdfjs/${dir}/${f}`, source: readFileSync(`${root}${dir}/${f}`) })
        }
      }
    },
    configureServer(server) {
      server.middlewares.use('/pdfjs', (req, res, next) => {
        const rel = decodeURIComponent((req.url || '').split('?')[0]).replace(/^\/+/, '')
        if (!PDFJS_DIRS.some((d) => rel.startsWith(d + '/')) || rel.includes('..')) return next()
        try { res.end(readFileSync(root + rel)) } catch { next() }
      })
    },
  }
}

// Nimos Library es un cliente independiente de Library Server.
// - dev (npm run dev): sirve en :5173 y puede usar el proxy local al Core (:8090).
// - build (npm run build): produce el cliente en dist/.
export default defineConfig({
  plugins: [svelte(), pdfjsAssets()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:8090',
      '/content': 'http://127.0.0.1:8090',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
