<script>
  import { onMount } from 'svelte'
  import { catalogCategories, catalogEntries, catalogDownload } from './api.js'
  import { bytes, num } from './fmt.js'

  let cats = $state([])
  let selCat = $state('wikipedia')
  let selLang = $state('')
  let q = $state('')
  let entries = $state([])
  let loading = $state(false)
  let busy = $state({})
  let flash = $state('')

  const LANGS = [
    { c: '', n: 'Todos' }, { c: 'spa', n: 'Español' }, { c: 'eng', n: 'Inglés' },
    { c: 'cat', n: 'Catalán' }, { c: 'fra', n: 'Francés' },
  ]

  async function loadEntries() {
    loading = true; flash = ''
    try {
      entries = await catalogEntries({ category: selCat, lang: selLang, q })
    } catch (e) {
      flash = 'Catálogo no accesible: ' + (e.message || '')
      entries = []
    } finally {
      loading = false
    }
  }

  onMount(async () => {
    try { cats = await catalogCategories() } catch {}
    await loadEntries()
  })

  function pickCat(c) { selCat = c; loadEntries() }
  function pickLang(l) { selLang = l; loadEntries() }

  async function download(e) {
    busy = { ...busy, [e.filename]: true }; flash = `Encolando ${e.filename}…`
    try {
      const r = await catalogDownload(e.zimUrl, e.filename)
      if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || 'error')
      flash = `En cola: ${e.title} · se registra solo al terminar (ver Cola)`
    } catch (err) {
      flash = `No se pudo encolar: ${err.message}`
    } finally {
      busy = { ...busy, [e.filename]: false }
    }
  }
</script>

<div class="ccnote">
  Catálogo público de Kiwix. Descarga un <code>.zim</code> al pool; al terminar
  <b>se registra y empieza a servirse solo</b> (lo ves en Colecciones).
</div>

<!-- Categorías -->
<div class="chips">
  {#each cats as c (c)}
    <button class="chip" class:on={selCat === c} onclick={() => pickCat(c)}>{c}</button>
  {/each}
</div>

<!-- Idioma + búsqueda -->
<div class="searchbar">
  <div class="search">
    <input placeholder="Filtrar por nombre…" bind:value={q} onkeydown={(e) => e.key === 'Enter' && loadEntries()} />
  </div>
  <button class="go" onclick={loadEntries} disabled={loading}>{loading ? '…' : 'Buscar'}</button>
</div>
<div class="chips">
  {#each LANGS as l (l.c)}
    <button class="chip" class:on={selLang === l.c} onclick={() => pickLang(l.c)}>{l.n}</button>
  {/each}
  {#if flash}<span style="margin-left:auto;font-size:12px;color:var(--ink-mute);align-self:center">{flash}</span>{/if}
</div>

{#if loading}
  <div class="empty">Consultando catálogo de Kiwix…</div>
{:else if entries.length}
  <div class="label">{entries.length} colecciones en «{selCat}»</div>
  {#each entries as e (e.filename)}
    {@const flav = (e.filename.replace(/\.zim$/i, '').match(/_(maxi|mini|nopic|nodet|novid)(?:_|$)/i) || [])[1]}
    <div class="row" style="grid-template-columns:1fr auto">
      <div style="min-width:0">
        <div class="cname">
          {e.title}
          {#if e.language}<span class="badge b-info">{e.language}</span>{/if}
          {#if flav}<span class="badge b-mute">{flav}</span>{/if}
          {#if e.inPool}<span class="badge b-signal">en el pool</span>{/if}
        </div>
        <div class="cpath">{e.filename}{#if e.articles} · {num(e.articles)} art{/if}</div>
      </div>
      {#if e.inPool}
        <button class="btn" disabled>✓ Descargado</button>
      {:else}
        <button class="btn btn-primary" disabled={busy[e.filename]} onclick={() => download(e)}>
          {busy[e.filename] ? 'Encolando…' : `Descargar ${e.bytes ? bytes(e.bytes) : ''}`}
        </button>
      {/if}
    </div>
  {/each}
{:else}
  <div class="empty">Sin resultados en esta categoría/idioma.</div>
{/if}
