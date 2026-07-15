<script>
  // Medios locales del pool: lista todo el contenido publicado y permite
  // ELIMINAR un item no deseado (ficha + fichero(s) + portada + pistas). Admin.
  import { onMount } from 'svelte'
  import { getMedia, deleteMedia } from './api.js'

  let items = $state([])
  let loading = $state(true)
  let busy = $state({}) // id -> true mientras borra
  let filter = $state('')

  async function load() {
    loading = true
    items = await getMedia()
    loading = false
  }
  onMount(load)

  async function del(it) {
    if (!confirm(`¿Eliminar "${it.title}" de la biblioteca?\nSe borra del pool y no se puede deshacer.`)) return
    busy = { ...busy, [it.id]: true }
    try {
      await deleteMedia(it.id)
      items = items.filter((x) => x.id !== it.id)
    } catch (e) {
      alert('No se pudo eliminar.')
    }
    busy = { ...busy, [it.id]: false }
  }

  const KIND = { video: 'Vídeo', audio: 'Audio', gallery: 'Imagen', pdf: 'Texto', reader: 'Texto' }
  const kindLabel = (t) => KIND[t] || 'Documento'

  const shown = $derived(
    items.filter((it) => {
      const q = filter.trim().toLowerCase()
      if (!q) return true
      return (it.title + ' ' + (it.collection || '') + ' ' + (it.author || '')).toLowerCase().includes(q)
    })
  )
</script>

<div class="toolbar">
  <span class="cnt"><b>{items.length}</b> contenidos en el pool</span>
  <span class="grow"></span>
  <input class="search-in" placeholder="Filtrar…" bind:value={filter} />
  <button class="btn" onclick={load}>↻ Actualizar</button>
</div>

{#if loading}
  <div class="empty">Cargando…</div>
{:else if shown.length}
  {#each shown as it (it.id)}
    <div class="row" style="grid-template-columns:1fr auto">
      <div style="min-width:0">
        <div class="cname">{it.title}</div>
        <div class="cpath">
          <span class="badge b-mute">{kindLabel(it.template)}</span>
          {it.collection}{#if it.author} · {it.author}{/if}
        </div>
      </div>
      <button class="btn btn-danger" title="Eliminar del pool" onclick={() => del(it)} disabled={busy[it.id]}>
        {busy[it.id] ? '…' : '🗑 Eliminar'}
      </button>
    </div>
  {/each}
{:else if items.length}
  <div class="empty">Ningún contenido coincide con el filtro.</div>
{:else}
  <div class="empty"><div class="big">Sin medios locales</div>Añade contenido propio al pool y aparecerá aquí para gestionarlo.</div>
{/if}

<style>
  .search-in { height: 34px; padding: 0 12px; border-radius: 8px; border: 1px solid var(--border); background: var(--card); color: var(--ink); font: inherit; outline: none; min-width: 160px; }
  .search-in:focus { border-color: var(--accent, #3fb950); }
  .btn-danger:hover:not(:disabled) { border-color: var(--crit, #da6b74); color: var(--crit, #da6b74); }
</style>
