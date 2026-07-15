<script>
  import { onMount } from 'svelte'
  import { authMe, authLogout, getStorage, getHealth, getCollections } from './lib/api.js'
  import { bytes } from './lib/fmt.js'
  import Login from './lib/Login.svelte'
  import Storage from './lib/Storage.svelte'
  import Collections from './lib/Collections.svelte'
  import Translation from './lib/Translation.svelte'
  import Import from './lib/Import.svelte'
  import Users from './lib/Users.svelte'

  let tab = $state('storage')
  let health = $state({ shim: '…', engine: '…' })
  let used = $state(0)
  let colCount = $state(0)

  let me = $state(null) // {setupNeeded, user}
  let loading = $state(true)

  const TABS = [
    { id: 'storage', label: 'Almacenamiento' },
    { id: 'collections', label: 'Colecciones' },
    { id: 'translation', label: 'Traducción' },
    { id: 'import', label: 'Importar' },
    { id: 'users', label: 'Usuarios' },
  ]

  async function loadKpis() {
    health = await getHealth()
    try { used = (await getStorage()).usedBytes || 0 } catch { used = 0 }
    try { colCount = (await getCollections()).length } catch { colCount = 0 }
  }

  async function refreshAuth() {
    me = await authMe()
    if (me.user?.isAdmin) loadKpis()
  }

  async function logout() {
    await authLogout()
    await refreshAuth()
  }

  onMount(async () => {
    await refreshAuth()
    loading = false
  })

  const isAdmin = $derived(me?.user?.isAdmin)
</script>

<div class="win">
  <div class="tbar">
    <b>Nimos Library</b><span class="sep">·</span>panel de control
    {#if isAdmin}<span class="sep">·</span><span style="color:var(--ink-dim)">{me.user.username}</span>{/if}
    <div class="dots"><i class="y"></i><i class="g"></i><i class="r"></i></div>
  </div>

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
      <header class="head">
        <div class="head-ic">
          <svg class="ic" viewBox="0 0 24 24" style="width:22px;height:22px">
            <path d="M4 5h6a2 2 0 012 2v12a3 3 0 00-3-3H4z" /><path d="M20 5h-6a2 2 0 00-2 2v12a3 3 0 013-3h5z" />
          </svg>
        </div>
        <div class="head-tx">
          <b>Panel de Control</b>
          <div class="head-sub">
            <span class="head-dot" class:off={health.shim !== 'up'}></span>
            Core {health.shim} · motor {health.engine}
          </div>
        </div>
        <span style="flex:1"></span>
        <button class="btn" onclick={logout}>Cerrar sesión</button>
      </header>

      <div class="kpis">
        <div class="kpi"><div class="n">{colCount}</div><div class="l">Colecciones</div></div>
        <div class="kpi"><div class="n">{bytes(used)}</div><div class="l">En uso en el pool</div></div>
        <div class="kpi"><div class="n">{health.engine === 'up' ? 'OK' : '—'}</div><div class="l">Motor ZIM</div></div>
      </div>

      <div class="tabs">
        {#each TABS as t (t.id)}
          <button class="tab" class:on={tab === t.id} onclick={() => (tab = t.id)}>{t.label}</button>
        {/each}
      </div>

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
