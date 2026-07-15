<script>
  import { auth, login, logout, changePassword } from './auth.svelte.js';

  let { onClose, onChanged, reason = '' } = $props();

  let username = $state('');
  let password = $state('');
  let err = $state('');
  let busy = $state(false);

  // Cambio de contraseña propia (solo con sesión iniciada).
  let showChange = $state(false);
  let curPw = $state('');
  let newPw = $state('');
  let newPw2 = $state('');
  let cpErr = $state('');
  let cpOk = $state(false);

  // Política (espejo del servidor): 10 caracteres + un especial. Solo feedback.
  const pwProblem = (pw) => {
    if ((pw || '').length < 10) return 'Mínimo 10 caracteres';
    if (!/[^\p{L}\p{N}]/u.test(pw)) return 'Debe incluir un carácter especial';
    return '';
  };

  async function submitChange(e) {
    e?.preventDefault();
    cpErr = ''; cpOk = false;
    const prob = pwProblem(newPw);
    if (prob) { cpErr = prob; return; }
    if (newPw !== newPw2) { cpErr = 'Las contraseñas nuevas no coinciden'; return; }
    busy = true;
    try {
      await changePassword(curPw, newPw);
      cpOk = true;
      curPw = ''; newPw = ''; newPw2 = '';
    } catch (e2) {
      cpErr = e2.message || 'no se pudo cambiar la contraseña';
    } finally {
      busy = false;
    }
  }

  async function submit(e) {
    e?.preventDefault();
    err = ''; busy = true;
    try {
      await login(username.trim(), password);
      onChanged?.();
      onClose?.();
    } catch (e2) {
      err = e2.message || 'no se pudo entrar';
    } finally {
      busy = false;
    }
  }

  async function doLogout() {
    busy = true;
    await logout();
    onChanged?.();
    onClose?.();
  }

  const initials = (n) => (n || '?').slice(0, 2).toUpperCase();
</script>

<div class="ov" onclick={onClose} role="presentation">
  <div class="card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
    {#if auth.user}
      <div class="who">
        <span class="av">{initials(auth.user.username)}</span>
        <div class="wt">
          <b>{auth.user.username}</b>
          <small>{auth.user.isAdmin ? 'Administrador · ve todo' : `${auth.user.age} años`}</small>
        </div>
      </div>
      <p class="hint">Sesión iniciada. Ves las colecciones según tu nivel y edad.</p>

      {#if showChange}
        {#if cpOk}
          <div class="note ok">Contraseña actualizada correctamente.</div>
        {/if}
        <form onsubmit={submitChange}>
          <label>Contraseña actual<input type="password" bind:value={curPw} autocomplete="current-password" required /></label>
          <label>Nueva contraseña<input type="password" bind:value={newPw} autocomplete="new-password" required /></label>
          <label>Repetir nueva<input type="password" bind:value={newPw2} autocomplete="new-password" required /></label>
          <div class="pwhint">Mínimo 10 caracteres e incluir un carácter especial (por ejemplo !@#$%).</div>
          {#if cpErr}<div class="err">{cpErr}</div>{/if}
          <div class="actions">
            <button type="button" class="btn" onclick={() => { showChange = false; cpErr = ''; cpOk = false; }}>Volver</button>
            <button type="submit" class="btn primary" disabled={busy}>{busy ? '…' : 'Guardar'}</button>
          </div>
        </form>
      {:else}
        <button class="btn" onclick={() => { showChange = true; cpErr = ''; cpOk = false; }}>Cambiar contraseña</button>
        <button class="btn primary" onclick={doLogout} disabled={busy}>{busy ? '…' : 'Cerrar sesión'}</button>
      {/if}
    {:else}
      <h3>Iniciar sesión</h3>
      {#if reason === 'download'}
        <div class="note">Para descargar este contenido necesitas una cuenta. Inicia sesión y vuelve a pulsar descargar.</div>
      {/if}
      <p class="hint">Inicia sesión para ver colecciones restringidas por sesión o edad.</p>
      <form onsubmit={submit}>
        <label>Usuario<input bind:value={username} autocomplete="username" required /></label>
        <label>Contraseña<input type="password" bind:value={password} autocomplete="current-password" required /></label>
        {#if err}<div class="err">{err}</div>{/if}
        {#if auth.setupNeeded}
          <div class="note">Aún no hay cuentas. Crea el administrador desde el <b>Panel de Control</b>.</div>
        {/if}
        <div class="actions">
          <button type="button" class="btn" onclick={onClose}>Cancelar</button>
          <button type="submit" class="btn primary" disabled={busy}>{busy ? '…' : 'Entrar'}</button>
        </div>
      </form>
    {/if}
  </div>
</div>

<style>
  .ov { position: fixed; inset: 0; background: rgba(0, 0, 0, .5); display: grid; place-items: center; z-index: 60; }
  .card { width: 100%; max-width: 340px; background: var(--panel); border: 1px solid var(--border); border-radius: 14px; padding: 22px; display: flex; flex-direction: column; gap: 12px; box-shadow: 0 20px 50px rgba(0, 0, 0, .4); }
  h3 { font-size: 16px; color: var(--ink); }
  .hint { font-size: 12.5px; color: var(--muted); line-height: 1.5; }
  form { display: flex; flex-direction: column; gap: 11px; }
  label { display: flex; flex-direction: column; gap: 5px; font-size: 11px; letter-spacing: .05em; text-transform: uppercase; color: var(--faint); }
  input { background: var(--panel-2, var(--raise)); border: 1px solid var(--border); border-radius: 8px; padding: 9px 11px; font-size: 14px; color: var(--ink); text-transform: none; letter-spacing: normal; }
  input:focus { outline: none; border-color: var(--accent-2); }
  .err { font-size: 12.5px; color: #f87171; }
  .note { font-size: 12px; color: var(--ink-dim); background: var(--raise); border: 1px solid var(--border); border-radius: 8px; padding: 8px 10px; }
  .note.ok { color: var(--ink); border-color: color-mix(in srgb, var(--accent) 40%, transparent); background: color-mix(in srgb, var(--accent) 12%, transparent); }
  .pwhint { font-size: 11px; color: var(--faint); line-height: 1.4; }
  .actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 2px; }
  .btn { padding: 9px 15px; border-radius: 9px; border: 1px solid var(--border); color: var(--ink-dim); font-size: 13.5px; }
  .btn:hover { background: var(--raise); color: var(--ink); }
  .btn.primary { background: color-mix(in srgb, var(--accent) 20%, transparent); border-color: color-mix(in srgb, var(--accent) 40%, transparent); color: var(--ink); width: 100%; justify-content: center; }
  .who { display: flex; align-items: center; gap: 12px; }
  .av { width: 42px; height: 42px; border-radius: 50%; display: grid; place-items: center; background: color-mix(in srgb, var(--accent) 30%, transparent); color: var(--ink); font-weight: 700; font-size: 15px; }
  .wt { display: flex; flex-direction: column; }
  .wt b { font-size: 15px; color: var(--ink); }
  .wt small { font-size: 12px; color: var(--muted); }
</style>
