<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import { t, i18n, setLocale, LANGS } from './i18n.svelte.js';
  import { theme, setTheme, THEMES } from './theme.svelte.js';
  import { profile, PROFILE_COLORS, setProfileName, setProfileColor, profileInitials, profileGradient } from './profile.svelte.js';
  import { getServerBase, setServerBase, serverFetch, isGateway, isShell, getGatewayTarget, setGatewayTarget } from './connection.js';

  let serverAddress = $state(getServerBase());
  let serverStatus = $state('');
  let checkingServer = $state(false);

  onMount(async () => {
    if (!isGateway()) return;
    try { serverAddress = await getGatewayTarget(); } catch (e) {}
  });

  async function saveServer() {
    checkingServer = true;
    serverStatus = 'Comprobando Library Server…';
    try {
      if (isGateway()) {
        await setGatewayTarget(serverAddress);
        location.reload();
        return;
      }
      setServerBase(serverAddress);
      const response = await serverFetch('/api/health');
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      serverStatus = 'Conexión correcta. Recargando…';
      setTimeout(() => location.reload(), 350);
    } catch (e) {
      serverStatus = 'No se pudo conectar con ese Library Server.';
      checkingServer = false;
    }
  }
</script>

<div class="view scroll">
  <header class="vhead">
    <span class="vtile"><Icon name="settings" size={20} /></span>
    <div class="vtitle">
      <h1>{t('settings.title')}</h1>
      <span class="vsub">{t('settings.subtitle')}</span>
    </div>
  </header>

  <div class="body">
    <section class="card">
      <div class="srow">
        <div class="sinfo">
          <b>Library Server</b>
          <small>Dirección del servidor que posee y sirve las colecciones, ZIM, vídeos y archivos.</small>
        </div>
      </div>
      {#if isShell() && !isGateway()}
        <div class="local-server"><span class="status-dot"></span><span>Servidor local integrado</span></div>
      {:else}
        <label class="field">
          <span>Dirección del servidor</span>
          <input bind:value={serverAddress} placeholder="http://192.168.1.50:8090"
            onkeydown={(e) => e.key === 'Enter' && saveServer()} />
        </label>
        <div class="server-actions">
          <button class="save" disabled={checkingServer} onclick={saveServer}>Guardar y conectar</button>
          {#if serverStatus}<small class="server-status">{serverStatus}</small>{/if}
        </div>
      {/if}
    </section>

    <section class="card">
      <div class="srow">
        <div class="sinfo">
          <b>{t('settings.profile')}</b>
          <small>{t('settings.profileDesc')}</small>
        </div>
        <span class="preview" style:background={profileGradient(profile.color)}>{profileInitials(profile.name)}</span>
      </div>
      <label class="field">
        <span>{t('settings.profileName')}</span>
        <input value={profile.name} maxlength="40" placeholder={t('settings.profileNamePlaceholder')}
          oninput={(e) => setProfileName(e.currentTarget.value)} />
      </label>
      <div class="swatches" aria-label={t('settings.profileColor')}>
        {#each PROFILE_COLORS as c}
          <button class="swatch" class:on={profile.color === c.id} title={t('settings.profileColor')}
            style:background={`linear-gradient(140deg, ${c.a}, ${c.b})`}
            onclick={() => setProfileColor(c.id)}>
            {#if profile.color === c.id}<Icon name="check" size={15} />{/if}
          </button>
        {/each}
      </div>
    </section>

    <section class="card">
      <div class="srow">
        <div class="sinfo">
          <b>{t('settings.theme')}</b>
          <small>{t('settings.themeDesc')}</small>
        </div>
      </div>
      <div class="langs">
        {#each THEMES as th}
          <button class="lang" class:on={theme.choice === th.code} onclick={() => setTheme(th.code)}>
            <span class="ticon"><Icon name={th.icon} size={17} /></span>
            <span class="lname">{t(th.labelKey)}</span>
            {#if theme.choice === th.code}<Icon name="check" size={16} />{/if}
          </button>
        {/each}
      </div>
    </section>

    <section class="card">
      <div class="srow">
        <div class="sinfo">
          <b>{t('settings.language')}</b>
          <small>{t('settings.languageDesc')}</small>
        </div>
      </div>
      <div class="langs">
        {#each LANGS as l}
          <button class="lang" class:on={i18n.locale === l.code} onclick={() => setLocale(l.code)}>
            <span class="flag">{l.flag}</span>
            <span class="lname">{l.label}</span>
            {#if i18n.locale === l.code}<Icon name="check" size={16} />{/if}
          </button>
        {/each}
      </div>
    </section>

    <section class="card soon">
      <div class="srow">
        <div class="sinfo">
          <b>{t('settings.more')}</b>
          <small>{t('settings.moreDesc')}</small>
        </div>
        <span class="pill">{t('settings.soon')}</span>
      </div>
    </section>
  </div>
</div>

<style>
  .view{background:var(--ground);min-height:100%;overflow-y:auto}
  .vhead{max-width:760px;margin:0 auto;padding:34px 40px 18px;display:flex;align-items:center;gap:14px}
  .vtile{width:44px;height:44px;border-radius:12px;flex:none;display:grid;place-items:center;background:color-mix(in srgb,var(--accent) 16%,transparent);color:var(--accent-2)}
  .vtitle{display:flex;flex-direction:column;gap:2px}
  .vtitle h1{font-size:24px;font-weight:680;letter-spacing:-.4px}
  .vsub{font-size:13px;color:var(--muted)}

  .body{max-width:760px;margin:0 auto;padding:6px 40px 90px;display:flex;flex-direction:column;gap:14px}
  .card{background:var(--card);border:1px solid var(--border);border-radius:14px;padding:18px 18px}
  .srow{display:flex;align-items:flex-start;justify-content:space-between;gap:16px}
  .sinfo{display:flex;flex-direction:column;gap:4px;min-width:0}
  .sinfo b{font-size:15px;color:var(--ink);font-weight:600}
  .sinfo small{font-size:12.5px;color:var(--muted);line-height:1.5}

  .preview{width:44px;height:44px;border-radius:50%;flex:none;display:grid;place-items:center;color:#fff;font-size:14px;font-weight:750;border:1px solid rgba(255,255,255,.12)}
  .field{margin-top:16px;display:flex;flex-direction:column;gap:7px}
  .field span{font-size:12px;font-weight:650;color:var(--faint);text-transform:uppercase;letter-spacing:.5px}
  .field input{height:42px;border-radius:10px;border:1px solid var(--border);background:var(--ground);color:var(--ink);padding:0 13px;font-size:14px;outline:none;transition:border-color .12s,background .12s}
  .field input:focus{border-color:color-mix(in srgb,var(--accent) 55%,var(--border));background:var(--panel-2)}
  .field input::placeholder{color:var(--muted)}
  .server-actions{display:flex;align-items:center;gap:12px;margin-top:12px;flex-wrap:wrap}
  .save{height:38px;padding:0 15px;border-radius:9px;border:1px solid color-mix(in srgb,var(--accent) 50%,var(--border));background:color-mix(in srgb,var(--accent) 15%,transparent);color:var(--ink);font-size:13px;font-weight:620}
  .save:hover:not(:disabled){background:color-mix(in srgb,var(--accent) 24%,transparent)}
  .save:disabled{opacity:.55;cursor:wait}
  .server-status{color:var(--muted);font-size:12.5px}
  .local-server{display:flex;align-items:center;gap:9px;margin-top:16px;padding:12px 14px;border-radius:10px;background:var(--ground);border:1px solid var(--border);color:var(--ink-dim);font-size:13px}
  .status-dot{width:8px;height:8px;border-radius:50%;background:#43b581;box-shadow:0 0 0 3px color-mix(in srgb,#43b581 18%,transparent)}
  .swatches{display:flex;gap:8px;margin-top:12px}
  .swatch{width:30px;height:30px;border-radius:50%;display:grid;place-items:center;color:#fff;border:2px solid transparent;box-shadow:inset 0 0 0 1px rgba(255,255,255,.16);transition:transform .12s,border-color .12s}
  .swatch:hover{transform:translateY(-1px)}
  .swatch.on{border-color:var(--ink)}

  .langs{display:grid;grid-template-columns:repeat(auto-fill,minmax(180px,1fr));gap:8px;margin-top:16px}
  .lang{display:flex;align-items:center;gap:11px;padding:12px 14px;border-radius:11px;border:1px solid var(--border);background:var(--ground);color:var(--ink-dim);text-align:left;transition:border-color .12s,background .12s,color .12s}
  .lang:hover{background:var(--raise);color:var(--ink)}
  .lang.on{border-color:color-mix(in srgb,var(--accent) 55%,var(--border));background:color-mix(in srgb,var(--accent) 12%,transparent);color:var(--ink)}
  .lang .flag{font-size:20px;line-height:1}
  .lang .ticon{display:grid;place-items:center;color:var(--muted)}
  .lang.on .ticon :global(.ic){color:var(--accent-2)}
  .lang .lname{flex:1;font-size:14px;font-weight:520}
  .lang.on :global(.ic){color:var(--accent-2)}

  .soon{opacity:.8}
  .pill{flex:none;font-size:11px;font-weight:600;letter-spacing:.4px;text-transform:uppercase;color:var(--accent-2);background:color-mix(in srgb,var(--accent) 16%,transparent);border-radius:20px;padding:5px 11px;white-space:nowrap}
</style>
