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
    <!-- Sin marco: solo el play. viewBox centrado sobre el bbox real (252.4,256.2)
         con leve nudge óptico a la derecha (el play apunta a la derecha, un
         centrado geométrico se ve corrido a la izquierda). -->
    <svg viewBox="60 73 366 366" width="100%" height="100%" fill="none" aria-hidden="true">
      <defs>
        <linearGradient id="bi_mo_outer" x1="110" y1="90" x2="385" y2="408" gradientUnits="userSpaceOnUse">
          <stop offset="0" stop-color="#ff4f8b"/><stop offset="0.42" stop-color="#ff7a4f"/><stop offset="0.72" stop-color="#a62bd8"/><stop offset="1" stop-color="#5a2de0"/>
        </linearGradient>
        <linearGradient id="bi_mo_inner" x1="190" y1="178" x2="294" y2="284" gradientUnits="userSpaceOnUse">
          <stop offset="0" stop-color="#ffd0ba"/><stop offset="1" stop-color="#ffb084"/>
        </linearGradient>
      </defs>
      <path fill="url(#bi_mo_outer)" d="M145 89c-21 0-39 17-39 39v258c0 29 31 47 56 32l218-129c25-15 25-50 0-65L162 95c-5-4-11-6-17-6Z"/>
      <path fill="#7A18D8" opacity="0.35" d="M123 330 320 282c17-4 20-28 5-36L145 89c-21 0-39 17-39 39v202c0 0 7 0 17 0Z"/>
      <path fill="url(#bi_mo_inner)" d="M188 171c0-10 10-16 19-11l113 65c9 5 9 18 0 23l-113 65c-9 5-19-1-19-11V171Z"/>
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
  .bi.mo{background:none;box-shadow:none}
  .bi.cb{background:#2b2620;box-shadow:inset 0 0 0 1px rgba(255,255,255,.10)}
  .bi svg{display:block}
</style>
