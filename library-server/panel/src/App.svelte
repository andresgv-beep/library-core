<script>
  import { onMount } from 'svelte'
  import { authMe, authLogout, getHealth } from './lib/api.js'
  import Login from './lib/Login.svelte'
  import Storage from './lib/Storage.svelte'
  import Collections from './lib/Collections.svelte'
  import Translation from './lib/Translation.svelte'
  import Import from './lib/Import.svelte'
  import Users from './lib/Users.svelte'

  let tab = $state('storage')
  let health = $state({ shim: '…', engine: '…' })
  let me = $state(null) // {setupNeeded, user}
  let loading = $state(true)

  const TABS = [
    { id: 'storage', label: 'Almacenamiento', icon: 'M4 6c0-1.7 3.6-3 8-3s8 1.3 8 3-3.6 3-8 3-8-1.3-8-3M4 6v12c0 1.7 3.6 3 8 3s8-1.3 8-3V6M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3' },
    { id: 'collections', label: 'Colecciones', icon: 'M4 5h5v14H4zM10 5h4v14h-4zM16 6l4 13' },
    { id: 'translation', label: 'Traducción', icon: 'M4 5h8M8 3v2M6 5c0 4-1.5 7-4 9M5 9c1.5 3 4 4 6 4M13 20l4-9 4 9M14.5 17h5' },
    { id: 'import', label: 'Importar', icon: 'M12 4v11M8 11l4 4 4-4M5 20h14' },
    { id: 'users', label: 'Usuarios', icon: 'M9 11a3.5 3.5 0 100-7 3.5 3.5 0 000 7zM3 20c0-3.3 2.7-5 6-5s6 1.7 6 5M17 8l2 2 3-3' },
  ]

  async function loadHealth() { health = await getHealth() }

  async function refreshAuth() {
    me = await authMe()
    if (me.user?.isAdmin) loadHealth()
  }
  async function logout() { await authLogout(); await refreshAuth() }

  onMount(async () => { await refreshAuth(); loading = false })

  const isAdmin = $derived(me?.user?.isAdmin)
</script>

<div class="win">
  <div class="tbar">
    <span class="tic"><svg viewBox="0 0 24 24" style="width:15px;height:15px"><path d="M4 5h6a2 2 0 012 2v12a3 3 0 00-3-3H4z" /><path d="M20 5h-6a2 2 0 00-2 2v12a3 3 0 013-3h5z" /></svg></span>
    <b>Nimos Library</b><span class="sep">·</span>panel de control
    {#if isAdmin}<span class="sep">·</span><span style="color:var(--ink-dim)">{me.user.username}</span>{/if}
    {#if isAdmin}
      <span class="thealth"><span class="head-dot" class:off={health.shim !== 'up'}></span>Core {health.shim} · motor {health.engine}</span>
    {/if}
    <div class="dots"><i class="y"></i><i class="g"></i><i class="r"></i></div>
  </div>

  {#if isAdmin}
    <nav class="topnav">
      {#each TABS as t (t.id)}
        <button class="tab" class:on={tab === t.id} onclick={() => (tab = t.id)}>
          <svg class="ic" viewBox="0 0 24 24"><path d={t.icon} /></svg>{t.label}
        </button>
      {/each}
      <span class="grow"></span>
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

  .topnav { flex: none; display: flex; align-items: center; gap: 3px; padding: 8px 14px; background: #131316; border-bottom: 1px solid var(--line); }
  .topnav .grow { flex: 1; }
  .topnav .tab { display: inline-flex; align-items: center; gap: 8px; padding: 8px 14px; border-radius: 8px; font-size: 12.5px; color: var(--ink-mute); letter-spacing: .01em; }
  .topnav .tab:hover { color: var(--ink); background: var(--canvas); }
  .topnav .tab.on { background: var(--signal-dim); color: var(--signal); border: 1px solid var(--signal-border); }
  .topnav .tab .ic { width: 16px; height: 16px; stroke: currentColor; stroke-width: 1.7; fill: none; stroke-linecap: round; stroke-linejoin: round; }
  .topnav .logout { padding: 6px 12px; font-size: 12px; }

  .content { padding-top: 18px; }
</style>
