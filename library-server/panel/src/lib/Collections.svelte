<script>
  import { onMount } from 'svelte'
  import { getAdminZim, getCollections, registerZim, unregisterZim, setZimInteractive, getAccessMap, setAccess } from './api.js'
  import { bytes, num } from './fmt.js'
  import Downloaded from './Downloaded.svelte'

  let sub = $state('kiwix')    // 'kiwix' (ZIM) | 'media' (medios locales del pool)
  let zim = $state(null)       // /api/admin/zim
  let rich = $state({})        // providerItemId → {id, itemCount, description}
  let accessMap = $state({})   // collectionId → {access, minAge}
  let error = $state('')
  let loading = $state(true)
  let busy = $state({})
  let flash = $state('')

  const ACCESS = [
    { k: 'open', label: 'Abierto', cls: 'b-signal' },
    { k: 'login', label: 'Sesión', cls: 'b-info' },
    { k: 'blocked', label: 'Bloqueado', cls: 'b-mute' },
  ]
  const accLabel = (a) => (ACCESS.find((x) => x.k === a) || ACCESS[2]).label
  const accCls = (a) => (ACCESS.find((x) => x.k === a) || ACCESS[2]).cls

  async function load() {
    loading = true; error = ''
    try {
      zim = await getAdminZim()
    } catch (e) {
      error = e.message; loading = false; return
    }
    try {
      const cols = await getCollections()
      const map = {}
      for (const c of cols) {
        const pid = c.source?.providerItemId
        if (pid) map[pid] = { id: c.id, itemCount: c.itemCount, description: c.description }
      }
      rich = map
    } catch { rich = {} }
    try { accessMap = await getAccessMap() } catch { accessMap = {} }
    loading = false
  }
  onMount(load)

  const richOf = (file) => rich[file.replace(/\.zim$/i, '')] || {}
  const cfgOf = (cid) => (cid && accessMap[cid]) || { access: 'blocked', minAge: 0, allowDownload: false }

  async function saveAccess(cid, access, minAge, allowDownload) {
    minAge = Math.max(0, Math.min(18, Number(minAge) || 0)) // siempre entero limpio
    accessMap = { ...accessMap, [cid]: { access, minAge, allowDownload } } // optimista
    const r = await setAccess(cid, access, minAge, allowDownload)
    if (!r.ok) flash = 'No se pudo guardar el acceso'
    else if (flash === 'No se pudo guardar el acceso') flash = ''
  }
  const changeAccess = (cid, a) => saveAccess(cid, a, cfgOf(cid).minAge, cfgOf(cid).allowDownload)
  const changeAge = (cid, age) => saveAccess(cid, cfgOf(cid).access, Math.max(0, Math.min(18, Number(age) || 0)), cfgOf(cid).allowDownload)
  const toggleDownload = (cid) => saveAccess(cid, cfgOf(cid).access, cfgOf(cid).minAge, !cfgOf(cid).allowDownload)

  async function doRegister(file) {
    busy = { ...busy, [file]: true }; flash = ''
    try {
      const r = await registerZim(file)
      if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || 'error')
      flash = `Añadida: ${file}`; await load()
    } catch (e) { flash = `No se pudo añadir: ${e.message}` } finally { busy = { ...busy, [file]: false } }
  }
  async function doUnregister(z) {
    if (!confirm(`Quitar "${z.title || z.file}" de la biblioteca?\n\nEl fichero .zim se conserva en el pool.`)) return
    busy = { ...busy, [z.id]: true }; flash = ''
    try {
      const r = await unregisterZim(z.id)
      if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || 'error')
      flash = `Quitada: ${z.title || z.file}`; await load()
    } catch (e) { flash = `No se pudo quitar: ${e.message}` } finally { busy = { ...busy, [z.id]: false } }
  }

  async function toggleInteractive(z) {
    const enabled = !z.interactive
    let acknowledge = false
    if (enabled && !z.official) {
      acknowledge = confirm(
        `Desbloquear contenido interactivo en "${z.title || z.file}"?\n\n` +
        `Este ZIM no procede del catálogo oficial verificado, o el archivo ha cambiado. ` +
        `Sus scripts se ejecutarán dentro de Noumon. Úsalo solo si confías en su origen y bajo tu responsabilidad.`
      )
      if (!acknowledge) return
    }
    busy = { ...busy, [`interactive:${z.id}`]: true }; flash = ''
    try {
      const r = await setZimInteractive(z.id, enabled, acknowledge)
      if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || 'error')
      flash = enabled ? `Contenido interactivo habilitado: ${z.title || z.file}` : `Contenido interactivo bloqueado: ${z.title || z.file}`
      await load()
    } catch (e) {
      flash = `No se pudo cambiar el contenido interactivo: ${e.message}`
    } finally {
      busy = { ...busy, [`interactive:${z.id}`]: false }
    }
  }

  const registered = $derived(zim?.registered || [])
  const unregistered = $derived(zim?.unregistered || [])
  const canManage = $derived(zim?.canManage ?? false)
</script>

<div class="stabs">
  <button class="stab" class:on={sub === 'kiwix'} onclick={() => (sub = 'kiwix')}>Kiwix / ZIM</button>
  <button class="stab" class:on={sub === 'media'} onclick={() => (sub = 'media')}>Medios locales</button>
</div>

{#if sub === 'media'}
  <Downloaded />
{:else}
<div class="toolbar">
  <span class="cnt"><b>{registered.length}</b> registradas · <b>{unregistered.length}</b> sin registrar</span>
  <span class="grow"></span>
  {#if flash}<span style="font-size:12px;color:var(--ink-mute)">{flash}</span>{/if}
  <button class="btn" onclick={load} disabled={loading}>↻ Actualizar</button>
</div>

{#if error}
  <div class="empty"><div class="big">No se pudo leer la biblioteca</div>{error}</div>
{:else if loading && !zim}
  <div class="empty">Leyendo biblioteca…</div>
{:else}
  {#if unregistered.length}
    <div class="label">En el pool, sin registrar</div>
    {#each unregistered as u (u.file)}
      <div class="row" style="grid-template-columns:40px 1fr auto">
        <div class="cic" style="background:var(--warn-dim);color:var(--warn)">＋</div>
        <div style="min-width:0">
          <div class="cname">{u.file}</div>
          <div class="cpath">{bytes(u.bytes)} · en el pool, no servido</div>
        </div>
        <button class="btn btn-primary" disabled={!canManage || busy[u.file]} onclick={() => doRegister(u.file)}>
          {busy[u.file] ? '…' : 'Añadir'}
        </button>
      </div>
    {/each}
  {/if}

  <div class="label" style="margin-top:{unregistered.length ? '16px' : '0'}">Registradas · nivel de acceso por colección</div>
  {#if registered.length}
    {#each registered as z (z.id)}
      {@const info = richOf(z.file)}
      {@const cid = info.id}
      {@const cfg = cfgOf(cid)}
      <div class="row" style="grid-template-columns:40px 1fr auto;align-items:start">
        <div class="cic" style="background:var(--info-dim);color:var(--info);margin-top:2px">{(z.title || z.file).charAt(0).toUpperCase()}</div>
        <div style="min-width:0">
          <div class="cname">
            {z.title || z.file}
            {#if z.language}<span class="badge b-info">{z.language}</span>{/if}
            {#if cid}<span class="badge {accCls(cfg.access)}">{accLabel(cfg.access)}{#if cfg.minAge > 0} · {cfg.minAge}+{/if}</span>{/if}
            {#if !z.present}<span class="badge b-warn">fichero ausente</span>{/if}
            {#if z.official}<span class="badge b-signal">origen oficial</span>{/if}
            {#if z.trustStale}<span class="badge b-warn">archivo cambiado</span>{/if}
          </div>
          <div class="cpath">{z.file}{#if info.itemCount} · {num(info.itemCount)} items{/if}</div>
          {#if cid}
            <div class="acc-strip">
              {#each ACCESS as a (a.k)}
                <button class="chip" class:on={cfg.access === a.k} onclick={() => changeAccess(cid, a.k)}>{a.label}</button>
              {/each}
              {#if cfg.access !== 'blocked'}
                <span class="acc-age">edad mín:
                  <input type="number" min="0" max="18" value={cfg.minAge} onchange={(e) => changeAge(cid, e.target.value)} />
                  <small>{cfg.minAge > 0 ? `solo ${cfg.minAge}+ (exige cuenta)` : 'sin límite'}</small>
                </span>
                <label class="acc-dl" title="Si está activo, cualquiera que pueda ver puede además descargar sin cuenta. Si no, para descargar hay que iniciar sesión.">
                  <input type="checkbox" checked={cfg.allowDownload} onchange={() => toggleDownload(cid)} />
                  <span>descarga anónima</span>
                </label>
              {/if}
            </div>
          {/if}
          <div class="interactive-strip">
            <span class="interactive-state" class:on={z.interactive}>
              {z.interactive ? 'Contenido interactivo permitido' : 'Contenido interactivo bloqueado'}
            </span>
            <button class="chip" class:on={z.interactive} disabled={!canManage || !z.present || busy[`interactive:${z.id}`]} onclick={() => toggleInteractive(z)}>
              {busy[`interactive:${z.id}`] ? '…' : z.interactive ? 'Bloquear scripts' : 'Desbloquear'}
            </button>
            {#if !z.interactive && !z.official}
              <small>Los ZIM añadidos manualmente no ejecutan scripts hasta que los autorices.</small>
            {/if}
          </div>
        </div>
        <button class="btn" disabled={!canManage || busy[z.id]} onclick={() => doUnregister(z)} style="margin-top:2px">
          {busy[z.id] ? '…' : 'Quitar'}
        </button>
      </div>
    {/each}
  {:else}
    <div class="empty"><div class="big">Ninguna colección registrada</div>Añade los .zim del pool con "Añadir".</div>
  {/if}

  <div class="empty" style="padding:14px 24px;font-size:11.5px">
    Lo nuevo entra <b>Bloqueado</b> por defecto. <b>Abierto</b> = todos · <b>Sesión</b> = con cuenta · <b>Bloqueado</b> = solo admin.
    Una edad mínima exige cuenta (sin sesión no se puede comprobar la edad). <b>Descarga anónima</b>: si está activa, quien pueda ver también puede bajar el fichero sin registrarse; si no, para descargar hay que iniciar sesión (ver sigue siendo público).
  </div>
{/if}
{/if}

<style>
  .acc-strip { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; margin-top: 8px; }
  .acc-strip .chip { padding: 3px 10px; font-size: 11.5px; }
  .acc-age { display: inline-flex; align-items: center; gap: 6px; font-size: 11.5px; color: var(--ink-faint); margin-left: 4px; }
  .acc-age input { width: 52px; background: var(--window-bg); border: 1px solid var(--line-bright); border-radius: 6px; padding: 4px 7px; font-size: 12px; color: var(--ink); }
  .acc-age small { color: var(--ink-faint); }
  .acc-dl { display: inline-flex; align-items: center; gap: 5px; font-size: 11.5px; color: var(--ink-faint); margin-left: 8px; cursor: pointer; }
  .acc-dl input { cursor: pointer; }
  .interactive-strip { display:flex; align-items:center; gap:8px; flex-wrap:wrap; margin-top:8px; }
  .interactive-state { font-size:11.5px; color:var(--ink-faint); }
  .interactive-state.on { color:var(--signal); }
  .interactive-strip small { color:var(--ink-faint); font-size:11px; }
</style>
