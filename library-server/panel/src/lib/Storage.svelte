<script>
  import { onMount } from 'svelte'
  import { getStorage } from './api.js'
  import { bytes, num, SECTION_META } from './fmt.js'

  let data = $state(null)
  let error = $state('')
  let loading = $state(true)

  async function load() {
    loading = true; error = ''
    try {
      data = await getStorage()
    } catch (e) {
      error = e.message || 'no se pudo leer el pool'
    } finally {
      loading = false
    }
  }
  onMount(load)

  const used = $derived(data?.usedBytes || 0)
  // el ancho de barra es relativo a la sección más grande, para que se lea bien
  const maxSection = $derived(Math.max(1, ...(data?.sections || []).map((s) => s.bytes || 0)))
</script>

<div class="toolbar">
  <span class="cnt">Pool de almacenamiento · <b>{bytes(used)}</b> en uso</span>
  <span class="grow"></span>
  <button class="btn" onclick={load} disabled={loading}>↻ Actualizar</button>
</div>

{#if data}
  <div class="setcard">
    <h4>Raíz del pool</h4>
    <div class="setrow">
      <code>{data.root || '— sin POOL_ROOT (rutas legacy) —'}</code>
      <span class="badge {data.provider === 'nimos' ? 'b-signal' : 'b-info'}">
        {data.provider || 'host'}
      </span>
    </div>
    <div class="setrow" style="color:var(--ink-faint);font-size:11.5px;padding-top:0">
      {#if data.root}
        La bifurcación por sistema vive aquí: NimOS concede esta raíz como volumen; en SO normal la apunta el admin.
      {:else}
        En dev no hay POOL_ROOT; cada ruta usa su default. Define POOL_ROOT para unificarlas (POOL-CONTRACT.md §5).
      {/if}
    </div>
  </div>

  <div class="label">Contenido del pool</div>
  {#each data.sections as s (s.key)}
    {@const meta = SECTION_META[s.key] || { label: s.key, glyph: '·', color: 'var(--ink-mute)' }}
    <div class="row" style="grid-template-columns:40px 1fr 150px">
      <div class="cic" style="background:color-mix(in srgb, {meta.color} 15%, transparent);color:{meta.color}">{meta.glyph}</div>
      <div style="min-width:0">
        <div class="cname">
          {meta.label}
          <span class="badge b-mute">{s.engine}</span>
          {#if !s.exists}<span class="badge b-warn">no encontrado</span>{/if}
        </div>
        <div class="cpath">{s.path || '— ubicación no declarada —'}</div>
        {#if s.exists && s.bytes > 0}
          <div class="bar" style="max-width:280px"><i style="width:{Math.max(3, (s.bytes / maxSection) * 100)}%"></i></div>
        {/if}
      </div>
      <div class="cmeta">
        {bytes(s.bytes)}<br>
        <span style="color:var(--ink-faint)">{num(s.items)} {s.key === 'zim' ? 'ficheros' : 'items'}</span>
      </div>
    </div>
  {/each}
{:else if loading}
  <div class="empty">Leyendo el pool…</div>
{:else if error}
  <div class="empty"><div class="big">No se pudo leer el pool</div>{error}</div>
{/if}
