<script>
  // Icono de las "apps" de contenido propio: Moments (vídeos) y Cabinet (archivo
  // documental). Mismo API que ZimIcon (kind/size/radius) para poder sustituirlo
  // donde antes salía la baldosa con la inicial. Se usa en los tiles del inicio,
  // el sidebar y los favoritos de esos ítems. Iconografía propia, sin marcas.
  let { kind = '', size = 28, radius = null } = $props();
  let rad = $derived(radius ?? Math.round(size * 0.27));
</script>

{#if kind === 'moments'}
  <span class="bi mo" style="width:{size}px;height:{size}px;border-radius:{rad}px">
    <svg viewBox="0 0 24 24" width="62%" height="62%" aria-hidden="true">
      <circle cx="12" cy="12" r="10" fill="none" stroke="#fff" stroke-width="1.6" opacity=".85" />
      <path d="M10 8.4v7.2c0 .5.55.8.97.53l5.5-3.6a.63.63 0 0 0 0-1.06l-5.5-3.6a.63.63 0 0 0-.97.53z" fill="#fff" />
    </svg>
  </span>
{:else if kind === 'cabinet'}
  <span class="bi cb" style="width:{size}px;height:{size}px;border-radius:{rad}px">
    <svg viewBox="0 0 24 24" width="62%" height="62%" aria-hidden="true">
      <rect x="4" y="3" width="16" height="18" rx="1.6" fill="none" stroke="#f2efe8" stroke-width="1.6" />
      <line x1="4" y1="12" x2="20" y2="12" stroke="#f2efe8" stroke-width="1.4" />
      <rect x="9.6" y="6.4" width="4.8" height="1.7" rx=".85" fill="#f2efe8" />
      <rect x="9.6" y="15.4" width="4.8" height="1.7" rx=".85" fill="#f2efe8" />
    </svg>
  </span>
{/if}

<style>
  .bi{flex:none;display:grid;place-items:center;overflow:hidden}
  .bi.mo{background:linear-gradient(135deg,#7c6cf0,#4f46b8);box-shadow:inset 0 0 0 1px rgba(255,255,255,.10)}
  .bi.cb{background:#2b2620;box-shadow:inset 0 0 0 1px rgba(255,255,255,.10)}
  .bi svg{display:block}
</style>
