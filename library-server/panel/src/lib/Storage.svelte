<script>
  import { onMount } from 'svelte'
  import { getStorage, setStorageRoot } from './api.js'
  import { bytes, num, SECTION_META } from './fmt.js'

  let data = $state(null)
  let error = $state('')
  let loading = $state(true)
  let editingRoot = $state(false)
  let draftRoot = $state('')
  let savingRoot = $state(false)
  let rootError = $state('')

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

  function startRootEdit() {
    draftRoot = data?.root || ((data?.volumes || [])[0]?.path || '') + 'Nimos Library'
    rootError = ''
    editingRoot = true
  }

  function chooseVolume(volume) {
    const separator = volume.includes('\\') ? '\\' : '/'
    draftRoot = volume.replace(/[\\/]+$/, '') + separator + 'Nimos Library'
  }

  async function saveRoot() {
    const next = draftRoot.trim()
    if (!next || savingRoot) return
    if (data.usedBytes > 0 && next.toLowerCase() !== (data.root || '').toLowerCase() &&
        !confirm('La ubicación cambiará para el contenido nuevo. Los archivos que ya existen no se moverán automáticamente. ¿Continuar?')) return
    savingRoot = true; rootError = ''
    try {
      await setStorageRoot(next)
      editingRoot = false
      await new Promise((resolve) => setTimeout(resolve, 1200))
      for (let attempt = 0; attempt < 60; attempt++) {
        try {
          const fresh = await getStorage()
          if (fresh.root) { location.reload(); return }
        } catch (e) {}
        await new Promise((resolve) => setTimeout(resolve, 1000))
      }
      rootError = 'El servidor tarda más de lo esperado en reiniciarse.'
    } catch (e) {
      rootError = e.message || 'No se pudo cambiar la ubicación.'
    } finally {
      savingRoot = false
    }
  }

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
    <div class="root-title">
      <h4>Carpeta de la biblioteca</h4>
      {#if data.configurable && !editingRoot}
        <button class="btn root-change" onclick={startRootEdit}>Cambiar ubicación</button>
      {/if}
    </div>
    <div class="setrow">
      <code>{data.root || '— sin POOL_ROOT (rutas legacy) —'}</code>
      <span class="badge {data.provider === 'nimos' ? 'b-signal' : 'b-info'}">
        {data.provider || 'host'}
      </span>
    </div>
    <div class="setrow" style="color:var(--ink-faint);font-size:11.5px;padding-top:0">
      {#if data.configurable}
        Aquí se guardan ZIM, medios, mapas y modelos. Las cuentas y la configuración interna permanecen protegidas en el sistema.
      {:else}
        Esta ubicación está administrada por la configuración externa del servidor.
      {/if}
    </div>

    {#if editingRoot}
      <div class="root-editor">
        <div class="root-label">Disco del servidor</div>
        <div class="volume-list">
          {#each data.volumes || [] as volume (volume.path)}
            <button class="chip" onclick={() => chooseVolume(volume.path)}>{volume.path}</button>
          {/each}
        </div>
        <label>
          Carpeta absoluta
          <input bind:value={draftRoot} placeholder="D:\Nimos Library" disabled={savingRoot} />
        </label>
        <div class="root-note">Si no existe, Library Server la creará y comprobará que puede escribir en ella.</div>
        {#if rootError}<div class="root-error">{rootError}</div>{/if}
        <div class="root-actions">
          <button class="btn" onclick={() => (editingRoot = false)} disabled={savingRoot}>Cancelar</button>
          <button class="btn btn-primary" onclick={saveRoot} disabled={savingRoot || !draftRoot.trim()}>{savingRoot ? 'Aplicando…' : 'Crear y usar'}</button>
        </div>
      </div>
    {:else if rootError}
      <div class="root-error">{rootError}</div>
    {/if}
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

<style>
  .root-title { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
  .root-title h4 { margin-bottom: 10px; }
  .root-change { padding: 5px 10px; font-size: 11.5px; margin-bottom: 8px; }
  .root-editor { margin-top: 12px; padding-top: 13px; border-top: 1px solid var(--line); }
  .root-label, .root-editor label { display: flex; flex-direction: column; gap: 6px; color: var(--ink-mute); font-size: 11.5px; }
  .volume-list { display: flex; flex-wrap: wrap; gap: 7px; margin: 7px 0 12px; }
  .root-editor input { width: 100%; padding: 10px 11px; border: 1px solid var(--line-bright); border-radius: 7px; background: var(--window-bg); color: var(--ink); font-family: var(--font-mono); font-size: 12.5px; }
  .root-editor input:focus { border-color: var(--signal-border); }
  .root-note { margin-top: 7px; color: var(--ink-faint); font-size: 11.5px; }
  .root-error { margin-top: 10px; padding: 8px 10px; border: 1px solid var(--crit-border); border-radius: 7px; background: var(--crit-dim); color: var(--crit); font-size: 12px; }
  .root-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 13px; }
</style>
