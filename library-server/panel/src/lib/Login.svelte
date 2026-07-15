<script>
  import { authRegister, authLogin } from './api.js'

  let { setupNeeded = false, onDone } = $props()

  let username = $state('')
  let password = $state('')
  let confirm = $state('')
  let err = $state('')
  let busy = $state(false)

  async function submit(e) {
    e?.preventDefault()
    err = ''
    if (setupNeeded && password !== confirm) {
      err = 'Las contraseñas no coinciden'
      return
    }
    busy = true
    try {
      const r = setupNeeded
        ? await authRegister(username.trim(), password, 0) // el admin no tiene edad
        : await authLogin(username.trim(), password)
      if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || 'error')
      onDone?.()
    } catch (e2) {
      err = e2.message || 'no se pudo'
    } finally {
      busy = false
    }
  }
</script>

<div class="login-wrap">
  <form class="login-card" onsubmit={submit}>
    <div class="login-ic">
      <svg class="ic" viewBox="0 0 24 24" style="width:24px;height:24px">
        <path d="M4 5h6a2 2 0 012 2v12a3 3 0 00-3-3H4z" /><path d="M20 5h-6a2 2 0 00-2 2v12a3 3 0 013-3h5z" />
      </svg>
    </div>
    <h2>{setupNeeded ? 'Crear administrador' : 'Panel de Control'}</h2>
    <p class="login-sub">
      {setupNeeded
        ? 'Primer uso: crea la cuenta de administrador. Será la dueña del sistema.'
        : 'Inicia sesión para administrar.'}
    </p>

    <label>Usuario<input bind:value={username} autocomplete="username" required /></label>
    <label>Contraseña<input type="password" bind:value={password} autocomplete={setupNeeded ? 'new-password' : 'current-password'} required /></label>
    {#if setupNeeded}
      <label>Repetir contraseña<input type="password" bind:value={confirm} autocomplete="new-password" required /></label>
    {/if}

    {#if err}<div class="login-err">{err}</div>{/if}
    <button class="btn btn-primary" type="submit" disabled={busy} style="justify-content:center;height:38px">
      {busy ? '…' : setupNeeded ? 'Crear y entrar' : 'Entrar'}
    </button>
  </form>
</div>

<style>
  .login-wrap { flex: 1; display: flex; align-items: center; justify-content: center; padding: 24px; }
  .login-card { width: 100%; max-width: 340px; display: flex; flex-direction: column; gap: 12px; background: var(--canvas); border: 1px solid var(--line); border-radius: 12px; padding: 26px 24px; }
  .login-ic { width: 46px; height: 46px; border-radius: 11px; background: var(--window-bg); border: 1px solid var(--signal-border); color: var(--signal); display: grid; place-items: center; }
  .login-card h2 { font-size: 17px; color: var(--ink); }
  .login-sub { font-size: 12px; color: var(--ink-mute); line-height: 1.5; margin-bottom: 4px; }
  .login-card label { display: flex; flex-direction: column; gap: 5px; font-size: 11px; letter-spacing: .06em; text-transform: uppercase; color: var(--ink-faint); }
  .login-card input { background: var(--window-bg); border: 1px solid var(--line-bright); border-radius: 7px; padding: 9px 11px; font-size: 13px; color: var(--ink); text-transform: none; letter-spacing: normal; }
  .login-card input:focus { border-color: var(--signal-border); }
  .login-err { font-size: 12px; color: var(--crit); background: var(--crit-dim); border: 1px solid var(--crit-border); border-radius: 7px; padding: 8px 10px; }
</style>
