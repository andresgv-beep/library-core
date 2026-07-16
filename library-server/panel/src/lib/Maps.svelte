<script>
  import { onMount } from 'svelte'
  import { getMaps, downloadMap, cancelMapDownload, activateMap, deleteMap, installMapGeocoder, indexMapStreets, cancelMapStreetIndex } from './api.js'
  import { bytes } from './fmt.js'

  let data = $state({ catalog: [], installed: [], active: null, job: null, available: false, geocoder: { installed: false, job: null } })
  let category = $state('Europa')
  let detail = $state(13)
  let busy = $state(false)
  let error = $state('')

  const categories = $derived([...new Set((data.catalog || []).map((r) => r.category))])
  const regions = $derived((data.catalog || []).filter((r) => r.category === category))
  const downloading = $derived(data.job && ['starting', 'downloading'].includes(data.job.status))
  const geocoding = $derived(data.geocoder?.job && ['downloading', 'indexing'].includes(data.geocoder.job.status))
  const streetIndexing = $derived(data.streetJob?.status === 'indexing')

  async function load() {
    try { data = await getMaps(); error = '' } catch (e) { error = e.message }
  }
  onMount(() => {
    load()
    const timer = setInterval(load, 1200)
    return () => clearInterval(timer)
  })

  async function start(region) {
    if (busy || downloading) return
    const warning = detail === 15 ? 'El nivel detallado puede ocupar varios GB según la región.' : 'El tamaño depende de la región seleccionada.'
    if (!confirm(`Descargar ${region.name} hasta zoom ${detail}?\n\n${warning}`)) return
    busy = true
    try { await downloadMap(region.id, detail); await load() } catch (e) { error = e.message } finally { busy = false }
  }
  async function activate(file) {
    busy = true
    try { await activateMap(file); await load() } catch (e) { error = e.message } finally { busy = false }
  }
  async function remove(file) {
    if (!confirm(`Eliminar ${file} del servidor?`)) return
    busy = true
    try { await deleteMap(file); await load() } catch (e) { error = e.message } finally { busy = false }
  }
  async function installGeocoder() {
    busy = true
    try { await installMapGeocoder(); await load() } catch (e) { error = e.message } finally { busy = false }
  }
  async function indexStreets(map) {
    if (!confirm(`Indexar las calles de ${map.name}?\n\nSe procesa el mapa localmente. Puede tardar varios minutos, pero el servidor seguirá funcionando.`)) return
    busy = true
    try { await indexMapStreets(map.file); await load() } catch (e) { error = e.message } finally { busy = false }
  }
</script>

<div class="toolbar">
  <span class="cnt">Mapas offline · <b>{data.installed?.length || 0}</b> instalados</span>
  <span class="grow"></span>
  <button class="btn" onclick={load}>↻ Actualizar</button>
</div>

{#if !data.available}
  <div class="ccnote">El extractor PMTiles no está instalado junto al servidor. Reinstala el paquete todo-en-uno.</div>
{/if}

{#if streetIndexing}
  <div class="mapjob">
    <div><b>Indexando calles de {data.streetJob.name}</b><small>{data.streetJob.tiles || 0} / {data.streetJob.totalTiles || '…'} teselas · {data.streetJob.streets || 0} calles · z{data.streetJob.zoom || '…'}</small></div>
    <span class="pspin"></span>
    <button class="btn" onclick={cancelMapStreetIndex}>Cancelar</button>
  </div>
{:else if data.streetJob?.status === 'error'}
  <div class="root-error">{data.streetJob.error || 'No se pudieron indexar las calles.'}</div>
{/if}

{#if downloading}
  <div class="mapjob">
    <div><b>Descargando {data.job.name}</b><small>{bytes(data.job.bytes)} escritos · zoom {data.job.maxZoom}</small></div>
    <span class="pspin"></span>
    <button class="btn" onclick={cancelMapDownload}>Cancelar</button>
  </div>
{:else if data.job?.status === 'error'}
  <div class="root-error">{data.job.error || 'No se pudo descargar el mapa.'}</div>
{/if}

<div class="geocoder" class:ready={data.geocoder?.installed}>
  <div class="cic">⌕</div>
  <div>
    <b>Buscador de lugares offline</b>
    {#if data.geocoder?.installed}
      <small>Activo · ciudades y localidades de todo el mundo · {bytes(data.geocoder.bytes)}</small>
    {:else if geocoding}
      <small>{data.geocoder.job.status === 'indexing' ? 'Creando índice…' : `Descargando… ${bytes(data.geocoder.job.bytes)}`}</small>
    {:else if data.geocoder?.job?.status === 'error'}
      <small class="geoerr">{data.geocoder.job.error}</small>
    {:else}
      <small>Necesario para buscar Barcelona, Madrid, Tokio y otras localidades.</small>
    {/if}
  </div>
  {#if !data.geocoder?.installed && !geocoding}<button class="btn" onclick={installGeocoder} disabled={busy}>Instalar buscador</button>{/if}
  {#if geocoding}<span class="pspin"></span>{/if}
</div>

<div class="label">Mapas instalados</div>
{#if data.installed?.length}
  {#each data.installed as map (map.file)}
    <div class="row installed">
      <div class="cic">◈</div>
      <div><div class="cname">{map.name} {#if data.active?.file === map.file}<span class="badge b-signal">activo</span>{/if}</div><div class="cpath">{map.file} · zoom {map.maxZoom} · {bytes(map.bytes)}</div></div>
      <div class="actions">
        {#if map.streetIndexed}
          <span class="badge b-signal">calles indexadas · {bytes(map.streetBytes)}</span>
        {:else}
          <button class="btn" onclick={() => indexStreets(map)} disabled={busy || streetIndexing}>Indexar calles</button>
        {/if}
        {#if data.active?.file !== map.file}<button class="btn" onclick={() => activate(map.file)} disabled={busy}>Activar</button>{/if}
        <button class="btn danger" onclick={() => remove(map.file)} disabled={busy}>Eliminar</button>
      </div>
    </div>
  {/each}
{:else}
  <div class="empty compact">Aún no hay mapas. Elige una zona del catálogo.</div>
{/if}

<div class="label catalog-label">Catálogo mundial</div>
<div class="chips">
  {#each categories as cat}<button class="chip" class:on={category === cat} onclick={() => (category = cat)}>{cat}</button>{/each}
</div>
<div class="detail">
  <span>Nivel de detalle</span>
  <button class="chip" class:on={detail === 10} onclick={() => (detail = 10)}>Básico · z10</button>
  <button class="chip" class:on={detail === 13} onclick={() => (detail = 13)}>Normal · z13</button>
  <button class="chip" class:on={detail === 15} onclick={() => (detail = 15)}>Detallado · z15</button>
</div>
<div class="region-grid">
  {#each regions as region (region.id)}
    <button class="region" onclick={() => start(region)} disabled={!data.available || busy || downloading}>
      <span class="rglyph">◈</span><span><b>{region.name}</b><small>Descargar recorte offline</small></span><span class="arrow">↓</span>
    </button>
  {/each}
</div>
{#if error}<div class="root-error">{error}</div>{/if}

<style>
  .mapjob { display:flex;align-items:center;gap:14px;margin-bottom:16px;padding:13px 15px;border:1px solid var(--info-border);border-radius:9px;background:var(--info-dim) }
  .mapjob div{flex:1}.mapjob b{display:block;font-size:13px}.mapjob small{display:block;margin-top:3px;color:var(--ink-mute)}
  .pspin{width:16px;height:16px;border:2px solid var(--line-bright);border-top-color:var(--info);border-radius:50%;animation:spin .8s linear infinite}@keyframes spin{to{transform:rotate(360deg)}}
  .installed{grid-template-columns:40px 1fr auto}.installed .cic{color:var(--signal);background:var(--signal-dim)}.actions{display:flex;gap:7px}.danger{color:var(--crit);border-color:var(--crit-border)}
  .compact{padding:22px}.catalog-label{margin-top:20px}.detail{display:flex;align-items:center;gap:7px;flex-wrap:wrap;margin-bottom:13px;color:var(--ink-faint);font-size:11.5px}.detail span{margin-right:4px}
  .geocoder{display:grid;grid-template-columns:40px 1fr auto;align-items:center;gap:10px;margin-bottom:16px;padding:12px 14px;border:1px solid var(--line);border-radius:9px;background:var(--canvas)}.geocoder.ready{border-color:var(--signal-border);background:var(--signal-soft)}.geocoder .cic{color:var(--signal);background:var(--signal-dim)}.geocoder b,.geocoder small{display:block}.geocoder b{font-size:13px}.geocoder small{margin-top:3px;color:var(--ink-mute);font-size:11px}.geocoder .geoerr{color:var(--crit)}
  .region-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px}.region{display:flex;align-items:center;gap:11px;padding:12px 13px;text-align:left;border:1px solid var(--line);border-radius:9px;background:var(--canvas)}.region:hover:not(:disabled){border-color:var(--signal-border);background:var(--signal-soft)}.region:disabled{opacity:.45;cursor:not-allowed}.region span:nth-child(2){flex:1}.region b{display:block;font-size:13px}.region small{display:block;margin-top:2px;color:var(--ink-faint);font-size:11px}.rglyph{color:var(--signal)}.arrow{color:var(--ink-faint);font-size:17px}
  .root-error{margin-top:12px;padding:9px 11px;border:1px solid var(--crit-border);border-radius:7px;background:var(--crit-dim);color:var(--crit);font-size:12px}
  @media(max-width:720px){.region-grid{grid-template-columns:1fr}}
</style>
