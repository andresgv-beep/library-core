<script>
  import { onMount } from 'svelte'
  import { authMe, authLogout, authLogoutAll, authRefresh, getHealth, getServiceStatus, restartLibraryServer } from './lib/api.js'
  import Login from './lib/Login.svelte'
  import Storage from './lib/Storage.svelte'
  import Collections from './lib/Collections.svelte'
  import Translation from './lib/Translation.svelte'
  import Import from './lib/Import.svelte'
  import Users from './lib/Users.svelte'
  import Maps from './lib/Maps.svelte'

  let tab = $state('storage')
  let health = $state({ shim: '…', engine: '…' })
  let me = $state(null) // {setupNeeded, user}
  let loading = $state(true)
  let service = $state({ supervised: false })
  let restarting = $state(false)
  let desktop = $state(false)

  const TABS = [
    { id: 'storage', label: 'Almacenamiento', icon: 'M4 6c0-1.7 3.6-3 8-3s8 1.3 8 3-3.6 3-8 3-8-1.3-8-3M4 6v12c0 1.7 3.6 3 8 3s8-1.3 8-3V6M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3' },
    { id: 'collections', label: 'Colecciones', icon: 'M4 5h5v14H4zM10 5h4v14h-4zM16 6l4 13' },
    { id: 'translation', label: 'Traducción', icon: 'M4 5h8M8 3v2M6 5c0 4-1.5 7-4 9M5 9c1.5 3 4 4 6 4M13 20l4-9 4 9M14.5 17h5' },
    { id: 'maps', label: 'Mapas', icon: 'M4 6l5-2 6 2 5-2v14l-5 2-6-2-5 2zM9 4v14M15 6v14' },
    { id: 'import', label: 'Importar', icon: 'M12 4v11M8 11l4 4 4-4M5 20h14' },
    { id: 'users', label: 'Usuarios', icon: 'M9 11a3.5 3.5 0 100-7 3.5 3.5 0 000 7zM3 20c0-3.3 2.7-5 6-5s6 1.7 6 5M17 8l2 2 3-3' },
  ]

  async function loadHealth() {
    health = await getHealth()
    if (health.shim === 'up') service = await getServiceStatus()
  }

  async function refreshAuth() {
    me = await authMe()
    if (me.user) await authRefresh()
    if (me.user?.isAdmin) loadHealth()
  }
  async function logout() { await authLogout(); await refreshAuth() }
  async function logoutAll() { await authLogoutAll(); await refreshAuth() }

  async function restartServer() {
    if (restarting || !confirm('¿Reiniciar Library Server ahora? Las conexiones se recuperarán automáticamente.')) return
    restarting = true
    try {
      await restartLibraryServer()
      health = { shim: 'restarting', engine: 'restarting' }
      await new Promise((resolve) => setTimeout(resolve, 1200))
      for (let attempt = 0; attempt < 60; attempt++) {
        const next = await getHealth()
        if (next.shim === 'up') { location.reload(); return }
        await new Promise((resolve) => setTimeout(resolve, 1000))
      }
    } catch (e) {}
    restarting = false
  }

  onMount(() => {
    desktop = typeof window.runtime?.WindowMinimise === 'function'
    refreshAuth().finally(() => (loading = false))
    const timer = setInterval(() => { if (me?.user?.isAdmin && !restarting) loadHealth() }, 5000)
    return () => clearInterval(timer)
  })

  const isAdmin = $derived(me?.user?.isAdmin)

  function minimiseWindow() { window.runtime?.WindowMinimise?.() }
  function toggleMaximiseWindow() { window.runtime?.WindowToggleMaximise?.() }
  function closeWindow() { window.runtime?.Quit?.() }
</script>

<div class="win">
  <div class="tbar" class:desktop>
    <span class="tic"><svg viewBox="0 0 24 24" style="width:15px;height:15px"><path d="M4 5h6a2 2 0 012 2v12a3 3 0 00-3-3H4z" /><path d="M20 5h-6a2 2 0 00-2 2v12a3 3 0 013-3h5z" /></svg></span>
    <b>Nimos Library</b><span class="sep">·</span>panel de control
    {#if isAdmin}<span class="sep">·</span><span style="color:var(--ink-dim)">{me.user.username}</span>{/if}
    {#if isAdmin}
      <span class="thealth"><span class="head-dot" class:off={health.shim !== 'up'}></span>Core {health.shim} · motor {health.engine}</span>
    {/if}
    {#if desktop}
      <div class="window-controls">
        <button class="window-button" title="Minimizar" aria-label="Minimizar" onclick={minimiseWindow}>
          <svg viewBox="0 0 10 10" aria-hidden="true"><line x1="1.5" y1="5" x2="8.5" y2="5" /></svg>
        </button>
        <button class="window-button" title="Maximizar" aria-label="Maximizar" onclick={toggleMaximiseWindow}>
          <svg viewBox="0 0 10 10" aria-hidden="true"><rect x="1.5" y="1.5" width="7" height="7" rx="1" /></svg>
        </button>
        <button class="window-button close" title="Cerrar" aria-label="Cerrar" onclick={closeWindow}>
          <svg viewBox="0 0 10 10" aria-hidden="true"><line x1="1.5" y1="1.5" x2="8.5" y2="8.5" /><line x1="8.5" y1="1.5" x2="1.5" y2="8.5" /></svg>
        </button>
      </div>
    {/if}
  </div>

  {#if isAdmin}
    <nav class="topnav">
      {#each TABS as t (t.id)}
        <button class="tab" class:on={tab === t.id} onclick={() => (tab = t.id)}>
          <svg class="ic" viewBox="0 0 24 24"><path d={t.icon} /></svg>{t.label}
        </button>
      {/each}
      <span class="grow"></span>
      {#if service.supervised}
        <button class="btn restart" disabled={restarting} onclick={restartServer}>{restarting ? 'Reiniciando…' : 'Reiniciar servidor'}</button>
      {/if}
      <button class="btn logout" onclick={logoutAll}>Cerrar todas</button>
      <button class="btn logout" onclick={logout}>Cerrar sesión</button>
    </nav>
  {/if}

  <div class="content scroll">
    {#if loading}
      <div class="empty" style="flex:1;display:grid;place-items:center">Cargando…</div>
    {:else if me.setupNeeded}
      <Login setupNeeded onDone={refreshAuth} />
    {:else if !me.user}
      <Login onDone={refreshAuth} />
    {:else if !me.user.isAdmin}
      <div class="empty" style="flex:1;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:12px">
        <div class="big">Panel solo para administradores</div>
        <div>Tu cuenta (<b>{me.user.username}</b>) no tiene permisos de administración.</div>
        <button class="btn" onclick={logout}>Cerrar sesión</button>
      </div>
    {:else}
      <div class="section scroll">
        {#if tab === 'storage'}
          <Storage />
        {:else if tab === 'collections'}
          <Collections />
        {:else if tab === 'translation'}
          <Translation />
        {:else if tab === 'maps'}
          <Maps />
        {:else if tab === 'import'}
          <Import />
        {:else if tab === 'users'}
          <Users me={me.user} />
        {/if}
      </div>
    {/if}
  </div>
</div>

<style>
  .tbar .tic { display: grid; place-items: center; color: var(--signal); margin-right: 9px; }
  .tbar .tic svg { stroke: currentColor; stroke-width: 1.6; fill: none; stroke-linecap: round; stroke-linejoin: round; }
  .thealth { display: inline-flex; align-items: center; gap: 6px; margin-left: 18px; font-size: 11.5px; color: var(--ink-mute); }
  .tbar.desktop { --wails-draggable: drag; }
  .tbar.desktop :is(button, .window-controls) { --wails-draggable: no-drag; }
  .window-controls { display: flex; align-items: center; gap: 2px; margin-left: auto; }
  .window-button { width: 34px; height: 30px; border-radius: 8px; display: grid; place-items: center; color: var(--ink-mute); transition: background .12s, color .12s; }
  .window-button svg { width: 10px; height: 10px; stroke: currentColor; stroke-width: 1.3; fill: none; stroke-linecap: round; }
  .window-button rect { stroke-width: 1.1; }
  .window-button:hover { background: var(--canvas-soft); color: var(--ink); }
  .window-button.close:hover { background: #e5484d; color: #fff; }

  .topnav { flex: none; display: flex; align-items: center; gap: 3px; padding: 8px 14px; background: #131316; border-bottom: 1px solid var(--line); }
  .topnav .grow { flex: 1; }
  .topnav .tab { display: inline-flex; align-items: center; gap: 8px; padding: 8px 14px; border-radius: 8px; font-size: 12.5px; color: var(--ink-mute); letter-spacing: .01em; }
  .topnav .tab:hover { color: var(--ink); background: var(--canvas); }
  .topnav .tab.on { background: var(--signal-dim); color: var(--signal); border: 1px solid var(--signal-border); }
  .topnav .tab .ic { width: 16px; height: 16px; stroke: currentColor; stroke-width: 1.7; fill: none; stroke-linecap: round; stroke-linejoin: round; }
  .topnav .logout { padding: 6px 12px; font-size: 12px; }
  .topnav .restart { padding: 6px 12px; font-size: 12px; color: var(--warn); border-color: var(--warn-border); }

  .content { padding-top: 18px; }
</style>
