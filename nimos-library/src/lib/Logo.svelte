<script>
  // Marca de NimOS Library: libro abierto (sin aro), en el MISMO tono metálico que
  // el wordmark "Library" (theme-aware via --lg-hi/--lg-lo). Escala a cualquier size.
  // ID de gradiente único por instancia para que no colisione entre logos.
  let { size = 24 } = $props();
  const uid = 'nl' + Math.random().toString(36).slice(2, 8);
</script>

<svg class="logo" viewBox="0 0 100 100" width={size} height={size} role="img" aria-label="Library">
  <defs>
    <linearGradient id="{uid}b" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0" style="stop-color:var(--lg-hi)" />
      <stop offset="1" style="stop-color:var(--lg-lo)" />
    </linearGradient>
  </defs>
  <g fill="url(#{uid}b)" transform="translate(50 50) scale(.43) translate(-256 -255.5)">
    <path d="M150 178 C150 168 158 162 168 162 C198 162 223 171 244 191 C250 197 253 205 253 214 L253 349 C228 326 198 314 161 314 C154 314 150 309 150 302 Z" />
    <path d="M362 178 C362 168 354 162 344 162 C314 162 289 171 268 191 C262 197 259 205 259 214 L259 349 C284 326 314 314 351 314 C358 314 362 309 362 302 Z" />
  </g>
</svg>

<style>
  .logo{display:block;flex:none;user-select:none;--lg-hi:#f3f4f7;--lg-lo:#aeb2bd}
  :global(:root[data-theme="light"]) .logo{--lg-hi:#63666f;--lg-lo:#3a3d48}
</style>
