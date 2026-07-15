<script>
  import { onMount } from 'svelte'
  import { listUsers, createUser, deleteUser, resetPassword } from './api.js'

  let { me } = $props()

  // Política de contraseñas (espejo de validatePassword en el servidor): 10
  // caracteres + al menos uno no alfanumérico. El servidor es la autoridad; esto
  // solo da feedback inmediato para no ir y volver al backend por un error obvio.
  const pwProblem = (pw) => {
    if ((pw || '').length < 10) return 'Mínimo 10 caracteres'
    if (!/[^\p{L}\p{N}]/u.test(pw)) return 'Debe incluir un carácter especial'
    return ''
  }
  // Generador de temporal que cumple la política (para el reset por olvido).
  const genTemp = () => {
    const set = 'abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789'
    const specials = '!@#$%&*?'
    let out = ''
    const rnd = (n) => Math.floor(Math.random() * n)
    for (let i = 0; i < 11; i++) out += set[rnd(set.length)]
    // Insertar un especial en posición aleatoria (garantiza la regla).
    const pos = rnd(out.length + 1)
    return out.slice(0, pos) + specials[rnd(specials.length)] + out.slice(pos)
  }

  // Estado del reset por fila.
  let resetId = $state(null)   // id del usuario cuyo form de reset está abierto
  let rpw = $state('')
  let rbusy = $state(false)
  let rflash = $state('')
  let rdone = $state('')       // muestra la temporal recién puesta (para copiarla)

  function openReset(u) {
    resetId = u.id; rpw = ''; rflash = ''; rdone = ''
  }
  function fillTemp() {
    rpw = genTemp(); rflash = ''
  }
  async function doReset(u) {
    rflash = ''
    const prob = pwProblem(rpw)
    if (prob) { rflash = prob; return }
    rbusy = true
    try {
      const r = await resetPassword(u.id, rpw)
      if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || 'no se pudo')
      rdone = rpw           // deja la clave a la vista para copiarla y pasársela al usuario
      rpw = ''
    } catch (e) { rflash = e.message } finally { rbusy = false }
  }

  let users = $state([])
  let loading = $state(true)
  let err = $state('')
  let flash = $state('')

  // formulario de alta
  let show = $state(false)
  let nu = $state('')
  let np = $state('')
  let nc = $state('')
  let na = $state(18)
  let nadmin = $state(false)
  let creating = $state(false)

  async function load() {
    loading = true; err = ''
    try { users = await listUsers() } catch (e) { err = e.message } finally { loading = false }
  }
  onMount(load)

  async function add() {
    flash = ''
    const prob = pwProblem(np)
    if (prob) { flash = prob; return }
    if (np !== nc) { flash = 'Las contraseñas no coinciden'; return }
    creating = true
    try {
      const r = await createUser({ username: nu.trim(), password: np, age: nadmin ? 0 : Number(na), isAdmin: nadmin })
      if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || 'error')
      nu = ''; np = ''; nc = ''; na = 18; nadmin = false; show = false
      await load()
    } catch (e) { flash = e.message } finally { creating = false }
  }

  async function del(u) {
    if (!confirm(`Borrar la cuenta de "${u.username}"?`)) return
    const r = await deleteUser(u.id)
    if (!r.ok) { flash = (await r.json().catch(() => ({}))).error || 'no se pudo'; return }
    await load()
  }

  const initials = (n) => (n || '?').slice(0, 2).toUpperCase()
</script>

<div class="toolbar">
  <span class="cnt"><b>{users.length}</b> usuarios</span>
  <span class="grow"></span>
  {#if flash}<span style="font-size:12px;color:var(--crit)">{flash}</span>{/if}
  <button class="btn btn-primary" onclick={() => (show = !show)}>{show ? 'Cerrar' : '＋ Añadir usuario'}</button>
</div>

{#if show}
  <div class="setcard">
    <h4>Nueva cuenta</h4>
    <div style="display:grid;grid-template-columns:1fr 1fr;gap:10px;margin-bottom:10px">
      <input class="uinput" placeholder="Usuario" bind:value={nu} />
      {#if nadmin}
        <div style="align-self:center;font-size:11.5px;color:var(--ink-faint)">Admin: ve todo (sin edad)</div>
      {:else}
        <input class="uinput" type="number" min="0" max="120" placeholder="Edad" bind:value={na} />
      {/if}
      <input class="uinput" type="password" placeholder="Contraseña" bind:value={np} />
      <input class="uinput" type="password" placeholder="Repetir contraseña" bind:value={nc} />
    </div>
    <div class="pwhint" style="margin-bottom:10px">Mínimo 10 caracteres e incluir al menos un carácter especial (por ejemplo !@#$%).</div>
    <div class="setrow" style="padding-top:0">
      <label style="display:flex;align-items:center;gap:8px;font-size:12.5px;color:var(--ink-dim);cursor:pointer">
        <input type="checkbox" bind:checked={nadmin} /> Administrador (ve y gestiona todo)
      </label>
      <button class="btn btn-primary" disabled={creating || !nu || !np || !nc} onclick={add}>{creating ? '…' : 'Crear'}</button>
    </div>
  </div>
{/if}

{#if loading}
  <div class="empty">Leyendo usuarios…</div>
{:else if err}
  <div class="empty"><div class="big">No se pudieron leer los usuarios</div>{err}</div>
{:else}
  {#each users as u (u.id)}
    <div class="row" style="grid-template-columns:40px 1fr auto">
      <div class="cic" style="background:{u.isAdmin ? 'var(--signal)' : 'var(--info-dim)'};color:{u.isAdmin ? '#0a0a0c' : 'var(--info)'}">{initials(u.username)}</div>
      <div style="min-width:0">
        <div class="cname">
          {u.username}
          {#if u.isAdmin}<span class="badge b-signal">admin</span>{:else}<span class="badge b-mute">{u.age} años</span>{/if}
          {#if me && u.id === me.id}<span class="badge b-info">tú</span>{/if}
        </div>
        <div class="cpath">{u.isAdmin ? 've todo · administra' : `ve ≤ ${u.age}+ según cada colección`}</div>
      </div>
      <div style="display:flex;gap:6px">
        <button class="btn" onclick={() => (resetId === u.id ? (resetId = null) : openReset(u))}>
          {resetId === u.id ? 'Cerrar' : 'Restablecer'}
        </button>
        {#if !me || u.id !== me.id}
          <button class="btn" onclick={() => del(u)}>Borrar</button>
        {/if}
      </div>
    </div>

    {#if resetId === u.id}
      <div class="setcard" style="margin:-4px 0 8px">
        {#if rdone}
          <h4>Contraseña temporal establecida</h4>
          <p class="tmphint">Pásasela a <b>{u.username}</b>. Podrá cambiarla desde su cuenta en el lector.</p>
          <div class="tmpbox">
            <code>{rdone}</code>
            <button class="btn" onclick={() => navigator.clipboard?.writeText(rdone)}>Copiar</button>
          </div>
          <div class="setrow" style="padding-top:8px">
            <span class="grow"></span>
            <button class="btn btn-primary" onclick={() => { resetId = null; rdone = '' }}>Hecho</button>
          </div>
        {:else}
          <h4>Restablecer contraseña de {u.username}</h4>
          <p class="tmphint">Pon una temporal (o genera una) y pásasela al usuario. Deberá cambiarla después desde su cuenta.</p>
          <div style="display:flex;gap:8px;align-items:center;margin-bottom:8px">
            <input class="uinput" style="flex:1" type="text" placeholder="Nueva contraseña temporal" bind:value={rpw} />
            <button class="btn" onclick={fillTemp}>Generar</button>
          </div>
          <div class="pwhint">Mínimo 10 caracteres e incluir un carácter especial.</div>
          <div class="setrow" style="padding-top:8px">
            {#if rflash}<span style="font-size:12px;color:var(--crit)">{rflash}</span>{/if}
            <span class="grow"></span>
            <button class="btn btn-primary" disabled={rbusy || !rpw} onclick={() => doReset(u)}>{rbusy ? '…' : 'Restablecer'}</button>
          </div>
        {/if}
      </div>
    {/if}
  {/each}
  <div class="empty" style="padding:16px 24px;font-size:11.5px">
    El primer usuario es admin (estilo Immich). La <b>edad</b> decide qué colecciones con restricción puede ver cada cuenta.
  </div>
{/if}

<style>
  .uinput { background: var(--window-bg); border: 1px solid var(--line-bright); border-radius: 7px; padding: 8px 10px; font-size: 13px; color: var(--ink); }
  .uinput:focus { border-color: var(--signal-border); }
  .pwhint { font-size: 11px; color: var(--ink-faint); line-height: 1.4; }
  .tmphint { font-size: 12px; color: var(--ink-dim); line-height: 1.5; margin-bottom: 8px; }
  .tmpbox { display: flex; align-items: center; gap: 8px; }
  .tmpbox code { flex: 1; background: var(--window-bg); border: 1px solid var(--line-bright); border-radius: 7px; padding: 8px 10px; font-size: 13px; color: var(--signal); letter-spacing: .02em; user-select: all; }
</style>
