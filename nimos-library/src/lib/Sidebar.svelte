<script>
  import Icon from './Icon.svelte';
  import Logo from './Logo.svelte';
  import ZimIcon from './ZimIcon.svelte';
  import BrandIcon from './BrandIcon.svelte';
  import { fmtSize } from './libraryApi.js';
  import { siteShown, toggleSite } from './sites.svelte.js';
  import { t } from './i18n.svelte.js';
  import { profile, profileInitials, profileGradient } from './profile.svelte.js';

  let { libraries = [], activeLib = null, activeView = 'home', user = null, onOpenLibrary, onOpenHome, onOpenView, onAccount } = $props();

  // key = valor de tab.view; 'home' abre el inicio (buscador + guardados).
  const NAV = [
    { labelKey: 'menu.home', icon: 'home', key: 'home' },
    { labelKey: 'menu.maps', icon: 'map', key: 'maps' },
    { labelKey: 'menu.favorites', icon: 'star', key: 'favorites' },
    { labelKey: 'menu.recent', icon: 'clock', key: 'recent' },
    { labelKey: 'menu.history', icon: 'history', key: 'history' },
    { labelKey: 'menu.notes', icon: 'note', key: 'notes' },
    { labelKey: 'menu.tags', icon: 'tag', key: 'tags' },
  ];

</script>

<aside class="side">
  <div class="head"><Logo size={22} />{t('side.library')}</div>

  <nav class="navlist">
    {#each NAV as n}
      <button class="navlink" class:active={activeView === n.key}
        onclick={() => { if (n.key === 'home') onOpenHome?.(); else onOpenView?.(n.key); }}>
        <Icon name={n.icon} /> {t(n.labelKey)}
      </button>
    {/each}
  </nav>

  <div class="sec-label">{t('side.collections')} <button class="add" title={t('side.addCollection')}><Icon name="plus" size={15} /></button></div>

  <div class="libs scroll thin">
    <div class="lib" class:active={activeView === 'cabinet'}>
      <button class="libopen" onclick={() => onOpenView?.('cabinet')} title={t('menu.cabinet')}>
        <BrandIcon kind="cabinet" size={26} radius={7} />
        <span class="meta">
          <span class="nm">{t('menu.cabinet')}</span>
          <span class="sub">{t('cabinet.sidebarSub')}</span>
        </span>
      </button>
      <button class="star" class:on={siteShown('cabinet')} title={siteShown('cabinet') ? t('side.unpinSite') : t('side.pinSite')} onclick={() => toggleSite('cabinet')}><Icon name="star" size={15} /></button>
    </div>
    <div class="lib" class:active={activeView === 'moments'}>
      <button class="libopen" onclick={() => onOpenView?.('moments')} title={t('menu.moments')}>
        <BrandIcon kind="moments" size={26} radius={7} />
        <span class="meta">
          <span class="nm">{t('menu.moments')}</span>
          <span class="sub">{t('moments.sidebarSub')}</span>
        </span>
      </button>
      <button class="star" class:on={siteShown('moments')} title={siteShown('moments') ? t('side.unpinSite') : t('side.pinSite')} onclick={() => toggleSite('moments')}><Icon name="star" size={15} /></button>
    </div>
    {#each libraries as lib}
      <div class="lib" class:active={activeLib === lib.id}>
        <button class="libopen" onclick={() => onOpenLibrary?.(lib)} title={lib.name}>
          <ZimIcon icon={lib.icon} name={lib.name} size={26} radius={7} />
          <span class="meta">
            <span class="nm">{lib.name}</span>
            <span class="sub">{lib.date}{lib.size ? ' · ' + fmtSize(lib.size) : (lib.articles ? ' · ' + lib.articles + ' art.' : '')}</span>
          </span>
        </button>
        <button class="star" class:on={siteShown(lib.id)} title={siteShown(lib.id) ? t('side.unpinSite') : t('side.pinSite')} onclick={() => toggleSite(lib.id)}><Icon name="star" size={15} /></button>
      </div>
    {/each}
    {#if libraries.length === 0}
      <div class="empty">{t('side.noCollections')}</div>
    {/if}
  </div>

  <div class="foot">
    <button class="footbtn" class:active={activeView === 'settings'} title={t('side.settings')} onclick={() => onOpenView?.('settings')}><Icon name="settings" size={17} /> {t('side.settings')}</button>
    <button class="user" title={t('side.account')} onclick={() => onAccount?.()}>
      <span class="avatar" style:background={profileGradient(profile.color)}>{user ? user.username.slice(0, 2).toUpperCase() : profileInitials(profile.name)}</span>
      <span class="uinfo">
        <b>{user ? user.username : 'Invitado'}</b>
        <small>{user ? (user.isAdmin ? 'Administrador' : user.age + ' años') : 'Iniciar sesión'}</small>
      </span>
      <Icon name="chevron" size={15} />
    </button>
  </div>
</aside>

<style>
  .side{background:var(--panel);border-right:1px solid var(--border);display:flex;flex-direction:column;min-height:0;overflow:hidden;height:100%}
  .head{display:flex;align-items:center;gap:10px;padding:16px 18px 12px;font-weight:640;font-size:15px}
  .navlist{display:flex;flex-direction:column;gap:1px;padding:0 10px}
  .navlink{display:flex;align-items:center;gap:11px;padding:8px 10px;border-radius:9px;color:var(--ink-dim);font-size:14px;text-align:left;transition:background .12s,color .12s}
  .navlink :global(.ic){color:var(--muted)}
  .navlink:hover{background:var(--raise);color:var(--ink)}
  .navlink.active{background:color-mix(in srgb,var(--accent) 16%,transparent);color:var(--ink)}
  .navlink.active :global(.ic){color:var(--accent-2)}
  .sec-label{padding:16px 20px 8px;font-size:11px;font-weight:650;letter-spacing:.7px;color:var(--faint);text-transform:uppercase;display:flex;align-items:center;justify-content:space-between}
  .add{width:20px;height:20px;border-radius:5px;display:grid;place-items:center;color:var(--muted)}
  .add:hover{background:var(--raise);color:var(--ink)}
  .libs{flex:1;overflow-y:auto;padding:0 10px;display:flex;flex-direction:column;gap:1px}
  .lib{display:flex;align-items:center;border-radius:9px;transition:background .12s;width:100%}
  .lib:hover{background:var(--raise)}
  .lib.active{background:color-mix(in srgb,var(--accent) 13%,transparent)}
  .libopen{display:flex;align-items:center;gap:11px;flex:1;min-width:0;text-align:left;padding:7px 4px 7px 10px}
  .meta{flex:1;min-width:0;display:flex;flex-direction:column}
  .nm{font-size:13.5px;color:var(--ink);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;text-align:left}
  .sub{font-size:11.5px;color:var(--muted);font-variant-numeric:tabular-nums;text-align:left}
  .star{width:30px;height:30px;margin-right:5px;border-radius:8px;display:grid;place-items:center;flex:none;color:var(--faint);transition:background .12s,color .12s}
  .star:hover{background:var(--border);color:var(--ink)}
  .star.on{color:var(--accent-2)}
  .star.on :global(.ic){fill:var(--accent-2);stroke:var(--accent-2)}
  .empty{padding:14px;color:var(--faint);font-size:13px}
  .foot{padding:8px 10px 10px;border-top:1px solid var(--border);display:flex;flex-direction:column;gap:2px}
  .footbtn{display:flex;align-items:center;gap:11px;padding:9px 10px;border-radius:9px;color:var(--ink-dim);font-size:14px;text-align:left;transition:background .12s,color .12s}
  .footbtn :global(.ic){color:var(--muted)}
  .footbtn:hover{background:var(--raise);color:var(--ink)}
  .footbtn.active{background:color-mix(in srgb,var(--accent) 16%,transparent);color:var(--ink)}
  .footbtn.active :global(.ic){color:var(--accent-2)}
  .user{display:flex;align-items:center;gap:10px;padding:8px 10px;border-radius:9px;transition:background .12s;width:100%}
  .user:hover{background:var(--raise)}
  .avatar{width:30px;height:30px;border-radius:50%;flex:none;display:grid;place-items:center;color:#fff;font-size:13px;font-weight:700;border:1px solid rgba(255,255,255,.12)}
  .uinfo{flex:1;min-width:0;display:flex;flex-direction:column;text-align:left}
  .uinfo b{font-size:13.5px;color:var(--ink);font-weight:560;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .uinfo small{font-size:11.5px;color:var(--muted)}
  .user :global(.ic){color:var(--faint)}
</style>
