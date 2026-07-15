<script>
  import Icon from './Icon.svelte';
  import ZimIcon from './ZimIcon.svelte';
  import BrandIcon from './BrandIcon.svelte';
  import { sourceOfItemId } from './brand.js';
  import { t } from './i18n.svelte.js';

  let { favorites = [], libraries = [], onOpenFav, onToggleHome, onRemoveFav, compact = false } = $props();

  let open = $state(false);
  const bookName = (id) => libraries.find((l) => l.id === id)?.name || id;
  const iconOf = (id) => libraries.find((l) => l.id === id)?.icon;
</script>

<div class="favwrap" class:compact onfocusout={(e) => { if (!e.currentTarget.contains(e.relatedTarget)) open = false; }}>
  <button class="favbtn" class:on={open} class:compact onclick={() => (open = !open)} title={t('fav.title')}>
    <Icon name="star" size={16} />
    <span>{t('fav.title')}</span>
    {#if favorites.length}<span class="badge">{favorites.length}</span>{/if}
  </button>

  {#if open}
    <div class="favdrop scroll thin">
      {#if favorites.length}
        {#each favorites as f}
          {@const src = sourceOfItemId(f.itemId)}
          <div class="favrow">
            <button class="favgo" onclick={() => { onOpenFav?.(f); open = false; }} title={f.title}>
              {#if src}<BrandIcon kind={src} size={28} radius={7} />{:else}<ZimIcon icon={iconOf(f.lib)} name={f.book || bookName(f.lib)} size={28} radius={7} />{/if}
              <span class="fmeta"><b>{f.title}</b><small>{f.book || bookName(f.lib)}</small></span>
            </button>
            <button class="pin" class:on={f.onHome} title={f.onHome ? t('fav.removeHome') : t('fav.pinHome')} onclick={() => onToggleHome?.(f)}><Icon name="home" size={14} /></button>
            <button class="rm" title={t('nav.removeFav')} onclick={() => onRemoveFav?.(f)}><Icon name="close" size={14} /></button>
          </div>
        {/each}
      {:else}
        <div class="favempty">{t('fav.empty1')}<br />{t('fav.empty2')}</div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .favwrap{position:relative;flex:none}
  .favbtn{display:flex;align-items:center;gap:8px;height:36px;padding:0 14px;background:var(--panel);border:1px solid var(--border);border-radius:10px;color:var(--ink-dim);font-size:13.5px;transition:background .12s,color .12s}
  .favbtn.compact{height:28px;padding:0 10px;font-size:12.5px;background:transparent}
  .favbtn.compact:hover,.favbtn.compact.on{background:var(--raise)}
  .favbtn:hover,.favbtn.on{background:var(--raise);color:var(--ink)}
  .favbtn :global(.ic){color:var(--accent-2)}
  .badge{min-width:18px;height:18px;padding:0 5px;border-radius:9px;background:color-mix(in srgb,var(--accent) 28%,transparent);color:var(--accent-2);font-size:11px;font-weight:650;display:grid;place-items:center;font-variant-numeric:tabular-nums}
  .favwrap.compact .favdrop{top:36px}
  .favdrop{position:absolute;top:44px;right:0;width:360px;max-height:min(420px,calc(100vh - 80px));overflow-y:auto;background:var(--card);border:1px solid var(--border);border-radius:12px;box-shadow:var(--shadow);padding:6px;z-index:30;display:flex;flex-direction:column;gap:1px}
  .favrow{display:flex;align-items:center;gap:2px;border-radius:9px;transition:background .12s}
  .favrow:hover{background:var(--raise)}
  .favgo{flex:1;min-width:0;display:flex;align-items:center;gap:10px;padding:8px 8px;text-align:left}
  .fmeta{display:flex;flex-direction:column;min-width:0}
  .fmeta b{font-size:13.5px;color:var(--ink);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;font-weight:550}
  .fmeta small{font-size:11.5px;color:var(--muted)}
  .pin,.rm{width:28px;height:28px;border-radius:7px;display:grid;place-items:center;color:var(--faint);flex:none}
  .pin:hover,.rm:hover{background:var(--border);color:var(--ink)}
  .pin.on{color:var(--accent-2)}
  .favempty{padding:22px 16px;text-align:center;color:var(--muted);font-size:13px;line-height:1.6}
</style>
