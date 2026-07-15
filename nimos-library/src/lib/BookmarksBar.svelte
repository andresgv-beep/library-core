<script>
  // Barra de marcadores (páginas guardadas ancladas al inicio). Vive a NIVEL DE APP
  // (bajo el navbar, en la columna principal) → persiste en todas las pestañas:
  // inicio, búsqueda, artículos ZIM, items, vistas. Sin scroll: muestra solo los
  // chips que caben y el resto los cuenta en "+N" (están en el menú Favoritos).
  import Icon from './Icon.svelte';
  import ZimIcon from './ZimIcon.svelte';
  import BrandIcon from './BrandIcon.svelte';
  import FavMenu from './FavMenu.svelte';
  import { sourceOfItemId } from './brand.js';
  import { t } from './i18n.svelte.js';

  let { favorites = [], libraries = [], onOpen, onToggleHome, onRemoveFav } = $props();
  const bookName = (id) => libraries.find((l) => l.id === id)?.name || id;
  const iconOf = (id) => libraries.find((l) => l.id === id)?.icon;
  let candidates = $derived(favorites.filter((f) => f.onHome));
  let chipsEl = $state(null);
  let hiddenCount = $state(0);

  // Los chips de páginas ancladas se miden dentro de su contenedor (chipsEl), que
  // es flex:1 y ya excluye el ancho del menú Favoritos (sibling flex:none al final).
  function measureBar() {
    const el = chipsEl; if (!el) return;
    const chips = [...el.querySelectorAll('.bm')];
    chips.forEach((c) => (c.style.display = ''));
    const n = chips.length;
    const total = favorites.length;
    if (n === 0) { hiddenCount = total; return; }
    const MORE_W = 44;
    const right = el.getBoundingClientRect().right;
    const allFit = chips[n - 1].getBoundingClientRect().right <= right;
    let fit;
    if (allFit && total <= n) {
      fit = n; // todo cabe y no hay extras → sin "+N"
    } else {
      const lim = right - MORE_W; // reserva hueco para el "+N"
      fit = 0;
      for (const c of chips) { if (c.getBoundingClientRect().right <= lim) fit++; else break; }
      if (fit < 1) fit = 1;
    }
    chips.forEach((c, i) => { if (i >= fit) c.style.display = 'none'; });
    hiddenCount = total - fit;
  }
  $effect(() => {
    candidates.length; // recalcular al cambiar los chips
    if (!chipsEl) return;
    measureBar();
    // El ancho cambia al plegar el sidebar (RO) o al redimensionar (resize).
    const ro = new ResizeObserver(() => measureBar());
    ro.observe(chipsEl);
    const onResize = () => measureBar();
    window.addEventListener('resize', onResize);
    return () => { ro.disconnect(); window.removeEventListener('resize', onResize); };
  });
</script>

<div class="bookmarks">
  <div class="bmchips" bind:this={chipsEl}>
    {#each candidates as f}
      {@const src = sourceOfItemId(f.itemId)}
      <div class="bm">
        <button class="bmopen" onclick={() => onOpen?.(f)} title={`${f.title} — ${f.book || bookName(f.lib)}`}>
          {#if src}<BrandIcon kind={src} size={18} radius={5} />{:else}<ZimIcon icon={iconOf(f.lib)} name={f.book || bookName(f.lib)} size={18} radius={5} />{/if}
          <span class="bmt">{f.title}</span>
        </button>
        <button class="bmx" title={t('fav.removeHome')} onclick={() => onToggleHome?.(f)}><Icon name="close" size={12} /></button>
      </div>
    {/each}
    {#if hiddenCount > 0}<span class="bmmore" title={t('home.favMore', { n: hiddenCount })}>+{hiddenCount}</span>{/if}
  </div>
  <FavMenu {favorites} {libraries} onOpenFav={onOpen} {onToggleHome} {onRemoveFav} compact />
</div>

<style>
  .bookmarks{display:flex;align-items:center;gap:8px;padding:6px 14px;background:var(--panel-2);border-bottom:1px solid var(--border);flex:none;min-width:0}
  .bmchips{display:flex;align-items:center;gap:4px;flex:1;min-width:0;overflow:hidden;white-space:nowrap}
  .bm{display:flex;align-items:center;border-radius:8px;flex:none;transition:background .12s}
  .bm:hover{background:var(--raise)}
  .bmopen{display:flex;align-items:center;gap:7px;padding:5px 4px 5px 8px;max-width:200px;min-width:0;text-align:left}
  .bmt{font-size:12.5px;color:var(--ink-dim);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .bm:hover .bmt{color:var(--ink)}
  .bmx{width:20px;height:20px;margin-right:4px;border-radius:5px;display:grid;place-items:center;color:var(--faint);flex:none;opacity:0;transition:opacity .12s,background .12s}
  .bm:hover .bmx{opacity:1}
  .bmx:hover{background:var(--border);color:var(--ink)}
  .bmmore{flex:none;padding:5px 10px;font-size:12px;color:var(--muted);font-variant-numeric:tabular-nums}
</style>
