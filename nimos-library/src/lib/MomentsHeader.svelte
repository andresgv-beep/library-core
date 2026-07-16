<script>
  // Cabecera reutilizable de la app Moments (logo + nombre + buscador).
  // Se usa en la superficie de Vídeos (Moments) y en la ficha de vídeo
  // (MomentsWatch) para que salga en todos los apartados. El buscador vive en el
  // store compartido videoSearch → filtra la cuadrícula aunque busques desde la ficha.
  import { videoSearch } from './videoSearch.svelte.js';
  import { t } from './i18n.svelte.js';
  import BrandIcon from './BrandIcon.svelte';
  let { subtitle = '', placeholder = '', onHome, onSubmit } = $props();
  function onKey(e) { if (e.key === 'Enter') onSubmit?.(); }
</script>

<div class="phead">
  <button class="phtitle" onclick={() => onHome?.()} title={t('moments.backHome')}>
    <BrandIcon kind="moments" size={44} radius={12} />
    <div class="pttext">
      <h1>{t('menu.moments')}</h1>
      {#if subtitle}<p class="sub">{subtitle}</p>{/if}
    </div>
  </button>
  <div class="phsearch">
    <div class="bigsearch">
      <svg class="ic" viewBox="0 0 24 24" width="18" height="18"><circle cx="11" cy="11" r="7"/><path d="M21 21l-4.3-4.3"/></svg>
      <input placeholder={placeholder || t('moments.searchPlaceholder')} bind:value={videoSearch.q} onkeydown={onKey} />
    </div>
  </div>
  <div class="phspacer"></div>
</div>

<style>
  .phead { display: grid; grid-template-columns: 1fr auto 1fr; align-items: center; gap: 16px; }
  @media (max-width: 780px) { .phead { grid-template-columns: 1fr; } .phspacer { display: none; } .phsearch { width: 100%; } }
  .phtitle { display: flex; flex-direction: row; align-items: center; gap: 12px; justify-self: start; background: none; border: 0; padding: 0; color: inherit; text-align: left; cursor: pointer; }
  .phtitle :global(.bi) { transition: transform .14s; }
  .phtitle:hover :global(.bi) { transform: scale(1.06); }
  .phtitle:hover h1 { color: var(--accent); }
  .pttext { display: flex; flex-direction: column; }
  .phead h1 { font-size: 24px; font-weight: 720; letter-spacing: -.4px; color: var(--ink); transition: color .14s; }
  .phead .sub { color: var(--muted); font-size: 13px; margin-top: 2px; }
  .phsearch { justify-self: center; width: min(460px, 42vw); }
  .bigsearch { display: flex; align-items: center; gap: 9px; height: 42px; width: 100%; padding: 0 14px; border-radius: 11px; background: var(--card); border: 1px solid var(--border); color: var(--muted); }
  .bigsearch:focus-within { border-color: color-mix(in srgb,var(--accent) 55%,var(--border)); }
  .bigsearch input { flex: 1; min-width: 0; font-size: 13.5px; color: var(--ink); background: none; border: none; outline: none; }
  .ic { stroke: currentColor; stroke-width: 1.7; fill: none; stroke-linecap: round; stroke-linejoin: round; flex: none; }
</style>
