<script>
  import Icon from './Icon.svelte';
  import Logo from './Logo.svelte';
  import { t } from './i18n.svelte.js';
  import { profile, profileInitials, profileGradient } from './profile.svelte.js';
  import { shell, win } from './shell.svelte.js';
  let { tabs = [], activeId = null, onActivate, onClose, onNew } = $props();
</script>

<header class="top" class:desktop={shell.desktop}>
  <div class="brand" title="Noumon" aria-label="Noumon">
    <Logo size={28} />
  </div>

  <div class="tabs">
    {#each tabs as tab (tab.id)}
      <div class="tabwrap" class:active={tab.id === activeId}>
        <button class="tab" onclick={() => onActivate?.(tab.id)}>
          <span class="ti"><Icon name={tab.kind === 'home' ? 'home' : (tab.kind === 'view' ? 'list' : 'book')} size={14} /></span>
          <span class="lbl">{tab.titleKey ? t(tab.titleKey) : tab.title}</span>
        </button>
        <button class="x" title={t('common.close')} onclick={() => onClose?.(tab.id)}><Icon name="close" size={13} /></button>
      </div>
    {/each}
    <button class="newtab" title={t('tabs.new')} onclick={() => onNew?.()}><Icon name="plus" size={16} /></button>
  </div>

  <div class="actions">
    {#if shell.desktop}
      <div class="wc">
        <button class="wcbtn" title={t('win.minimise')} aria-label={t('win.minimise')} onclick={() => win.minimise()}>
          <svg viewBox="0 0 10 10" width="10" height="10" aria-hidden="true"><line x1="1.5" y1="5" x2="8.5" y2="5" /></svg>
        </button>
        <button class="wcbtn" title={t('win.maximise')} aria-label={t('win.maximise')} onclick={() => win.toggleMaximise()}>
          <svg viewBox="0 0 10 10" width="10" height="10" aria-hidden="true"><rect x="1.5" y="1.5" width="7" height="7" rx="1" /></svg>
        </button>
        <button class="wcbtn close" title={t('common.close')} aria-label={t('common.close')} onclick={() => win.close()}>
          <svg viewBox="0 0 10 10" width="10" height="10" aria-hidden="true"><line x1="1.5" y1="1.5" x2="8.5" y2="8.5" /><line x1="8.5" y1="1.5" x2="1.5" y2="8.5" /></svg>
        </button>
      </div>
    {:else}
      <span class="avatar" title={profile.name || t('side.user')} style:background={profileGradient(profile.color)}>{profileInitials(profile.name)}</span>
    {/if}
  </div>
</header>

<style>
  .top{display:flex;align-items:center;gap:6px;background:var(--panel-2);border-bottom:1px solid var(--border-soft);padding:0 12px 0 16px;height:100%}
  .brand{display:grid;place-items:center;width:32px;padding-right:6px;flex:none}
  .tabs{display:flex;align-items:center;gap:4px;flex:1;min-width:0;height:100%;padding-top:0;overflow:hidden}
  .tabwrap{display:flex;align-items:center;max-width:210px;height:34px;border-radius:9px;color:var(--muted);font-size:13px;white-space:nowrap;border:1px solid transparent;transition:background .12s,color .12s,border-color .12s;overflow:hidden}
  .tabwrap:hover{background:var(--panel);color:var(--ink-dim)}
  .tab{display:flex;align-items:center;gap:9px;min-width:0;height:100%;padding:0 6px 0 11px;flex:1;text-align:left}
  .tab .ti{display:grid;place-items:center;color:var(--faint)}
  .tabwrap.active .ti{color:var(--accent-2)}
  .tab .lbl{overflow:hidden;text-overflow:ellipsis}
  .x{width:22px;height:100%;display:grid;place-items:center;color:var(--faint);opacity:0;transition:opacity .12s,background .12s;flex:none}
  .tabwrap:hover .x,.tabwrap.active .x{opacity:1}
  .x:hover{background:var(--raise);color:var(--ink)}
  .tabwrap.active{background:var(--panel);color:var(--ink);border-color:var(--border-soft)}
  .newtab{width:30px;height:30px;border-radius:8px;display:grid;place-items:center;color:var(--muted);flex:none}
  .newtab:hover{background:var(--panel);color:var(--ink)}
  .actions{display:flex;align-items:center;gap:2px;flex:none;padding-left:8px}
  .avatar{width:28px;height:28px;border-radius:50%;flex:none;display:grid;place-items:center;color:#fff;font-size:11px;font-weight:650;border:1px solid rgba(255,255,255,.12)}

  /* Shell nativo: toda la barra arrastra la ventana; lo interactivo, no. */
  .top.desktop{--wails-draggable:drag}
  .top.desktop :is(button,input,.tabwrap,.wc){--wails-draggable:no-drag}

  /* Controles de ventana (min/max/cerrar) — solo en desktop. */
  .wc{display:flex;align-items:center;gap:2px;margin-left:4px}
  .wcbtn{width:34px;height:30px;border-radius:8px;display:grid;place-items:center;color:var(--muted);transition:background .12s,color .12s;flex:none}
  .wcbtn svg{stroke:currentColor;stroke-width:1.3;fill:none;stroke-linecap:round}
  .wcbtn rect{fill:none;stroke:currentColor;stroke-width:1.1}
  .wcbtn:hover{background:var(--raise);color:var(--ink)}
  .wcbtn.close:hover{background:#e5484d;color:#fff}
</style>
