<script>
  // Icono de una colección: el logo REAL incrustado en el ZIM (ilustración que sirve
  // kiwix). Si no hay o falla la carga, cae a la baldosa con inicial+gradiente.
  let { icon = null, name = '', size = 28, radius = null } = $props();
  let rad = $derived(radius ?? Math.round(size * 0.27));
  let broken = $state(false);
  // reintenta si cambia el icono (p.ej. al cargar libraries)
  $effect(() => { icon; broken = false; });

  function tile(n) {
    const c = { wik:'linear-gradient(140deg,#5a6bd8,#8b5cf0)', arch:'var(--arch)', stack:'var(--so)', docker:'var(--dk)', diablo:'var(--diablo)', synology:'var(--syn)', gutenberg:'var(--book)' };
    const s = (n || '').toLowerCase();
    for (const k in c) if (s.includes(k)) return c[k];
    return 'linear-gradient(140deg,var(--accent),var(--accent-2))';
  }
  const initial = (n) => (n || '?').trim()[0]?.toUpperCase() || '?';
</script>

{#if icon && !broken}
  <span class="zt img" style="width:{size}px;height:{size}px;border-radius:{rad}px">
    <img src={icon} alt={name} onerror={() => (broken = true)} />
  </span>
{:else}
  <span class="zt letter" style="width:{size}px;height:{size}px;border-radius:{rad}px;background:{tile(name)};font-size:{Math.round(size * 0.42)}px">{initial(name)}</span>
{/if}

<style>
  .zt{flex:none;display:grid;place-items:center;overflow:hidden}
  .zt.img{background:var(--icon-surface);box-shadow:inset 0 0 0 1px rgba(0,0,0,.08)}
  .zt.img img{width:100%;height:100%;object-fit:contain;padding:13%;display:block}
  .zt.letter{color:#fff;font-weight:700;box-shadow:inset 0 0 0 1px rgba(255,255,255,.08)}
</style>
