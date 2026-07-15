<script>
  import Icon from './Icon.svelte';
  import TranslateMenu from './TranslateMenu.svelte';
  import { t } from './i18n.svelte.js';
  import { formatLibraryAddress } from './libraryAddress.js';
  import { profile, profileInitials, profileGradient } from './profile.svelte.js';

  let { active = null, sidebarOpen = true, indexOpen = false, user = null, starred = false, noted = false, tagged = false,
        onToggleSidebar, onToggleIndex, onBack, onForward, onReload, onHome, onToggleFav, onOpenNote, onOpenTags,
        onAccount, onNavigateAddress } = $props();

  let canBack = $derived(!!(active && active.back && active.back.length));
  let canForward = $derived(!!(active && active.fwd && active.fwd.length));
  let address = $derived(formatLibraryAddress(active));
  let addressValue = $state('library://home');
  let editingAddress = $state(false);
  $effect(() => { if (!editingAddress) addressValue = address; });
  function submitAddress(e) {
    e?.preventDefault();
    editingAddress = false;
    onNavigateAddress?.(addressValue);
  }
  // Carga en curso (búsqueda global de la pestaña activa) → barra en la pill.
  let loading = $derived(!!(active && active.search && active.search.loading));
</script>

<div class="nav">
  <button class="navbtn" class:on={!sidebarOpen} title={sidebarOpen ? t('nav.hideLibrary') : t('nav.showLibrary')} onclick={() => onToggleSidebar?.()}><Icon name="panel" /></button>
  <button class="navbtn" title={t('nav.back')} disabled={!canBack} onclick={() => onBack?.()}><Icon name="back" /></button>
  <button class="navbtn" title={t('nav.forward')} disabled={!canForward} onclick={() => onForward?.()}><Icon name="forward" /></button>
  <button class="navbtn" title={t('nav.reload')} onclick={() => onReload?.()}><Icon name="reload" /></button>
  <button class="navbtn" title={t('nav.home')} onclick={() => onHome?.()}><Icon name="home" /></button>

  <form class="address" onsubmit={submitAddress}>
    <Icon name="lock" size={15} />
    <input class="url" aria-label="Dirección de Library" bind:value={addressValue}
      onfocus={(e) => { editingAddress = true; e.currentTarget.select(); }}
      onblur={() => { editingAddress = false; addressValue = address; }} />
    {#if active && (active.kind === 'article' || active.kind === 'item')}
      <button class="mini" class:tagged title={tagged ? t('nav.editTags') : t('nav.tag')} onclick={() => onOpenTags?.()}><Icon name="tag" size={15} /></button>
      <button class="mini" class:noted title={noted ? t('nav.editNote') : t('nav.addNote')} onclick={() => onOpenNote?.()}><Icon name="note" size={15} /></button>
      <button class="mini" class:starred title={starred ? t('nav.removeFav') : t('nav.saveFav')} onclick={() => onToggleFav?.()}><Icon name="star" size={15} /></button>
    {/if}
    <TranslateMenu />
    {#if loading}<div class="loadbar" aria-hidden="true"></div>{/if}
  </form>

  <button class="userbtn" title={t('side.account')} onclick={() => onAccount?.()}>
    <span class="uav" style:background={profileGradient(profile.color)}>{user ? user.username.slice(0, 2).toUpperCase() : profileInitials(profile.name)}</span>
  </button>

  <button class="idxbtn" class:on={indexOpen} title={t('nav.indexTitle')} onclick={() => onToggleIndex?.()}>
    <Icon name="list" size={16} /> <span>{t('nav.index')}</span>
  </button>
</div>

<style>
  .nav{display:flex;align-items:center;gap:6px;padding:0 16px;background:var(--panel-2);border-bottom:1px solid var(--border);height:100%}
  .navbtn{width:32px;height:32px;border-radius:8px;display:grid;place-items:center;color:var(--ink-dim);transition:background .12s;flex:none}
  .navbtn:hover{background:var(--panel)}
  .navbtn.on{background:color-mix(in srgb,var(--accent) 16%,transparent);color:var(--accent-2)}
  .navbtn:disabled{color:var(--faint);cursor:default}
  .navbtn:disabled:hover{background:none}
  .address{flex:1;display:flex;align-items:center;gap:10px;height:36px;margin:0 6px;padding:0 8px 0 12px;background:var(--panel);border:1px solid var(--border);border-radius:10px;color:var(--ink-dim);font-size:13.5px;min-width:0;position:relative;overflow:hidden}
  .loadbar{position:absolute;left:0;right:0;bottom:0;height:2px;background:linear-gradient(90deg,transparent,var(--accent),var(--accent-2),transparent);background-size:45% 100%;background-repeat:no-repeat;animation:navload 1.05s ease-in-out infinite}
  @keyframes navload{0%{background-position:-45% 0}100%{background-position:145% 0}}
  @media (prefers-reduced-motion:reduce){.loadbar{animation:none;background:linear-gradient(90deg,transparent,color-mix(in srgb,var(--accent) 60%,transparent),transparent)}}
  .address :global(.ic){color:var(--muted)}
  .url{flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;background:none;border:0;outline:0;color:var(--ink-dim);font:inherit}
  .url:focus{color:var(--ink)}
  .mini{width:26px;height:26px;border-radius:7px;display:grid;place-items:center;color:var(--muted);flex:none}
  .mini.starred{color:var(--accent-2)}
  .mini.starred :global(.ic){fill:var(--accent-2);stroke:var(--accent-2)}
  .mini.noted{color:var(--accent-2)}
  .mini.noted :global(.ic){fill:color-mix(in srgb,var(--accent-2) 22%,transparent)}
  .mini.tagged{color:var(--accent-2)}
  .mini.tagged :global(.ic){fill:color-mix(in srgb,var(--accent-2) 22%,transparent)}
  .mini:hover{background:var(--raise);color:var(--ink)}
  .userbtn{width:36px;height:36px;border-radius:10px;display:grid;place-items:center;flex:none;transition:background .12s}
  .userbtn:hover{background:var(--panel)}
  .uav{width:28px;height:28px;border-radius:50%;display:grid;place-items:center;color:#fff;font-size:11px;font-weight:650;border:1px solid rgba(255,255,255,.14)}
  .idxbtn{display:flex;align-items:center;gap:8px;height:36px;padding:0 14px;background:var(--panel);border:1px solid var(--border);border-radius:10px;color:var(--ink-dim);font-size:13.5px;flex:none;transition:background .12s,color .12s,border-color .12s}
  .idxbtn:hover{background:var(--raise);color:var(--ink)}
  .idxbtn :global(.ic){color:var(--muted)}
  .idxbtn.on{background:color-mix(in srgb,var(--accent) 18%,transparent);color:var(--accent-2);border-color:color-mix(in srgb,var(--accent) 40%,transparent)}
  .idxbtn.on :global(.ic){color:var(--accent-2)}
</style>
