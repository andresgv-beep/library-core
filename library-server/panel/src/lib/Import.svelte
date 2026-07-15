<script>
  import { onMount, onDestroy } from 'svelte'
  import { listDownloads, pauseDownload, resumeDownload, cancelDownload, clearDownloads } from './api.js'
  import { bytes } from './fmt.js'
  import KiwixCatalog from './KiwixCatalog.svelte'

  let sub = $state('kiwix')

  // ── Cola ──
  let jobs = $state([])
  let poll
  const ACTIVE = new Set(['queued', 'downloading', 'paused'])

  async function refreshQueue() {
    jobs = await listDownloads()
  }
  async function clearQueue() {
    try { await clearDownloads() } catch (e) {}
    refreshQueue()
  }
  function jobName(j) {
    if (j.dest_path) return j.dest_path.split(/[\\/]/).pop()
    return j.owner_id || j.url
  }
  const queueCount = $derived(jobs.filter((j) => ACTIVE.has(j.status)).length)

  onMount(() => {
    refreshQueue()
    poll = setInterval(refreshQueue, 2000)
  })
  onDestroy(() => clearInterval(poll))
</script>

<div class="stabs">
  <button class="stab" class:on={sub === 'kiwix'} onclick={() => (sub = 'kiwix')}>Kiwix</button>
  <button class="stab" class:on={sub === 'cola'} onclick={() => (sub = 'cola')}>
    Cola {#if queueCount}<span class="qn">{queueCount}</span>{/if}
  </button>
</div>

{#if sub === 'kiwix'}
  <KiwixCatalog />
{:else}
  <div class="toolbar">
    <span class="cnt"><b>{jobs.length}</b> descargas · {queueCount} activas</span>
    <span class="grow"></span>
    <button class="btn" onclick={refreshQueue}>↻ Actualizar</button>
    <button class="btn" title="Limpiar historial (quita las descargas terminadas; no borra ficheros)" aria-label="Limpiar historial"
      onclick={clearQueue} disabled={!jobs.some((j) => j.status === 'done' || j.status === 'error' || j.status === 'cancelled')}>
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6M10 11v5M14 11v5"/></svg>
    </button>
  </div>
  {#if jobs.length}
    {#each jobs as j (j.id)}
      {@const pct = j.total_bytes > 0 ? Math.round((j.written_bytes / j.total_bytes) * 100) : 0}
      <div class="row" style="grid-template-columns:1fr 90px auto">
        <div style="min-width:0">
          <div class="cname">{jobName(j)}</div>
          <div class="cpath">
            {j.owner_kind} · {bytes(j.written_bytes)}{#if j.total_bytes > 0} / {bytes(j.total_bytes)}{/if}
            {#if j.error_msg} · <span style="color:var(--crit)">{j.error_msg}</span>{/if}
          </div>
          {#if ACTIVE.has(j.status) && j.total_bytes > 0}
            <div class="bar" style="max-width:320px"><i style="width:{pct}%"></i></div>
          {/if}
        </div>
        <div class="cmeta">
          <span class="badge {j.status === 'done' ? 'b-signal' : j.status === 'error' ? 'b-warn' : j.status === 'downloading' ? 'b-info' : 'b-mute'}">{j.status}</span>
          {#if j.total_bytes > 0}<br><span style="color:var(--ink-faint)">{pct}%</span>{/if}
        </div>
        <div style="display:flex;gap:6px;justify-self:end">
          {#if j.status === 'downloading' || j.status === 'queued'}
            <button class="btn" title="Pausar" onclick={() => pauseDownload(j.id).then(refreshQueue)}>⏸</button>
          {:else if j.status === 'paused' || j.status === 'error'}
            <button class="btn" title="Reanudar" onclick={() => resumeDownload(j.id).then(refreshQueue)}>▶</button>
          {/if}
          {#if j.status !== 'done' && j.status !== 'cancelled'}
            <button class="btn" title="Cancelar" onclick={() => cancelDownload(j.id).then(refreshQueue)}>✕</button>
          {/if}
        </div>
      </div>
    {/each}
  {:else}
    <div class="empty"><div class="big">Cola vacía</div>Descarga un ZIM desde el catálogo Kiwix y aparecerá aquí con su progreso.</div>
  {/if}
{/if}
