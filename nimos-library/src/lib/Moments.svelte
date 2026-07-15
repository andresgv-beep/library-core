<script>
  // Moments.svelte — superficie de vídeos propios. Dos vistas:
  //  • INICIO: tira de canales en miniatura (rota cuando se llena) + "Seguir
  //    viendo" + "Recientes". El botón cuadrado "Todos" va al lado de la tira.
  //  • TODOS LOS CANALES: cuadrícula de todos los canales (esconde tira y vídeos);
  //    el botón pasa a "Volver" y regresa al inicio tal cual.
  // App aparte de "Archivo local" (Archives). Un vídeo se abre como PESTAÑA
  // (MomentsWatch) vía onOpenItem.
  import { onMount } from 'svelte';
  import { getCollections, getCollectionItems } from './libraryApi.js';
  import { videoProgress, clearAllProgress } from './videoProgress.svelte.js';
  import { channelFaves, toggleFave, isFave } from './channelFaves.svelte.js';
  import { t } from './i18n.svelte.js';
  import { videoSearch } from './videoSearch.svelte.js';
  import MomentsHeader from './MomentsHeader.svelte';

  let { onOpenItem } = $props();

  let items = $state([]);
  let loading = $state(true);
  let errMsg = $state('');
  let channel = $state('');        // canal filtrado ('' = todos)
  let channelsView = $state(false); // true = cuadrícula de todos los canales

  async function load() {
    loading = true; errMsg = '';
    try {
      const cols = (await getCollections()).filter((c) => c.kind === 'media');
      const groups = await Promise.all(cols.map(async (c) => {
        const list = await getCollectionItems(c.id);
        return list.map((it) => ({ ...it, sectionName: c.title }));
      }));
      const seen = new Set();
      items = groups.flat()
        .filter((it) => it.source?.provider === 'moments')
        .filter((it) => (seen.has(it.id) ? false : (seen.add(it.id), true)));
    } catch (e) {
      errMsg = e.message || t('moments.error');
    } finally {
      loading = false;
    }
  }
  onMount(load);

  const chanOf = (it) => ((it.authors || []).join(', ') || it.sectionName || '');
  const channels = $derived([...new Set(items.map(chanOf).filter(Boolean))]);

  const byDate = (a, b) => (b.date || '').localeCompare(a.date || ''); // recientes primero
  // El buscador (en la cabecera) filtra vídeos por título/descr/autor/tags.
  function vmatch(it) {
    const q = videoSearch.q.trim().toLowerCase();
    if (!q) return true;
    return [it.title, it.description, ...(it.authors || []), ...(it.tags || [])]
      .filter(Boolean).join(' ').toLowerCase().includes(q);
  }
  const recentVideos = $derived(items.filter(vmatch).sort(byDate));
  const channelVideos = $derived(items.filter((it) => chanOf(it) === channel && vmatch(it)).sort(byDate));

  // Seguir viendo: vídeos con progreso guardado, del más reciente visto al más antiguo.
  const continueWatching = $derived(
    items
      .filter((it) => videoProgress.map[it.id])
      .sort((a, b) => (videoProgress.map[b.id]?.at || 0) - (videoProgress.map[a.id]?.at || 0))
  );

  const initials = (s) => (s || '?').trim().split(/\s+/).slice(0, 2).map((w) => w[0] || '').join('').toUpperCase();
  function hms(s) {
    s = Math.max(0, Math.floor(s || 0));
    const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), ss = String(s % 60).padStart(2, '0');
    return h ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`;
  }
  const cover = (it) => (it?.preview?.kind === 'image' ? it.preview.url : '');
  const avatar = (it) => it?.channelAvatar || '';
  const prog = (it) => videoProgress.map[it.id];
  const pct = (p) => (p && p.d ? Math.min(100, Math.max(2, (p.t / p.d) * 100)) : 0);

  const chanAvatars = $derived(
    Object.fromEntries(items.map((it) => [chanOf(it), it.channelAvatar]).filter(([k, v]) => k && v))
  );
  const chanAvatar = (c) => chanAvatars[c] || '';

  const channelCards = $derived(
    channels.map((c) => ({ name: c, avatar: chanAvatar(c), count: items.filter((it) => chanOf(it) === c).length }))
  );
  const shownChannels = $derived(
    channelCards.filter((c) => { const q = videoSearch.q.trim().toLowerCase(); return !q || c.name.toLowerCase().includes(q); })
  );

  let scroller = $state(null);
  function openChannels() { channelsView = true; videoSearch.q = ''; }
  function backHome() { channelsView = false; videoSearch.q = ''; }
  function pickChannel(c) { channel = c; channelsView = false; videoSearch.q = ''; }
  function clearChannel() { channel = ''; }
  // Clic en el logo + "Vídeos" → vuelve al inicio (limpia vista/filtro/búsqueda y sube arriba).
  function goInicio() {
    channelsView = false; channel = ''; videoSearch.q = '';
    if (scroller) scroller.scrollTop = 0;
  }

  // Favoritos: canales guardados que aún existen en el pool (avatar + nombre).
  const favedChannels = $derived(
    channelFaves.list.filter((n) => channels.includes(n)).map((n) => ({ name: n, avatar: chanAvatar(n) }))
  );
  function clearHistory() {
    if (typeof confirm === 'undefined' || confirm(t('moments.clearConfirm'))) {
      clearAllProgress();
    }
  }

  // Tira de creadores: si no caben, se mueve en bucle infinito (marquee).
  let viewportEl = $state(null);
  let firstSetEl = $state(null);
  let animate = $state(false);
  function measure() {
    animate = !!(viewportEl && firstSetEl && firstSetEl.scrollWidth > viewportEl.clientWidth + 8);
  }
  $effect(() => {
    channels.length; channelsView;
    if (typeof requestAnimationFrame !== 'undefined') requestAnimationFrame(measure);
  });
</script>

<svelte:window onresize={() => { if (typeof requestAnimationFrame !== 'undefined') requestAnimationFrame(measure); }} />

{#snippet videoCard(it)}
  {@const p = prog(it)}
  <button class="card" onclick={() => onOpenItem?.(it.id)}>
    <div class="poster" class:img={cover(it)}>
      {#if cover(it)}<img src={cover(it)} alt={it.title} loading="lazy" />{/if}
      {#if it.duration}<div class="dur">{hms(it.duration)}</div>{/if}
      <div class="play"><span><svg viewBox="0 0 24 24" width="20" height="20" fill="#fff"><path d="M8 6l10 6-10 6z"/></svg></span></div>
      {#if p}<div class="prog"><i style="width:{pct(p)}%"></i></div>{/if}
    </div>
    <div class="cmeta">
      <div class="av" class:img={avatar(it)}>
        {#if avatar(it)}<img src={avatar(it)} alt="" />{:else}{initials(chanOf(it) || 'YT')}{/if}
      </div>
      <div class="ct">
        <h3>{it.title}</h3>
        <div class="by">{chanOf(it)}</div>
        {#if it.date}<div class="st">{it.date}</div>{/if}
      </div>
    </div>
  </button>
{/snippet}

<div class="yp scroll" bind:this={scroller}>
  {#if errMsg}
    <div class="pane"><div class="err">{errMsg}</div></div>
  {:else if loading}
    <div class="pane"><div class="hint">{t('common.loading')}</div></div>
  {:else if items.length === 0}
    <div class="pane"><div class="empty">
      <div class="ei">📺</div>
      <div class="et">{t('moments.emptyTitle')}</div>
      <div class="eb">{t('moments.emptyBody')}</div>
    </div></div>
  {:else}
    <div class="pane">
      <div class="pheadwrap">
        <MomentsHeader
          subtitle={channelsView ? t('moments.allChannels') : channel}
          placeholder={channelsView ? t('moments.searchChannel') : t('moments.searchPlaceholder')}
          onHome={goInicio} />
      </div>

      {#if channels.length > 1}
        <!-- Tira de canales en miniatura + botón cuadrado a la izquierda -->
        <div class="creators">
          <button class="tggl" onclick={() => (channelsView ? backHome() : openChannels())}
                  title={channelsView ? t('moments.backHome') : t('moments.viewAllChannels')}>
            {#if channelsView}
              <svg class="ic" viewBox="0 0 24 24" width="15" height="15"><path d="M15 5l-7 7 7 7"/></svg>
              {t('cabinet.back')}
            {:else}
              <svg class="ic" viewBox="0 0 24 24" width="14" height="14"><rect x="3" y="3" width="7" height="7" rx="1.6"/><rect x="14" y="3" width="7" height="7" rx="1.6"/><rect x="3" y="14" width="7" height="7" rx="1.6"/><rect x="14" y="14" width="7" height="7" rx="1.6"/></svg>
              {t('moments.all')}
            {/if}
          </button>

          {#if !channelsView}
            <div class="mviewport" class:on={animate} bind:this={viewportEl}>
              <div class="mtrack" class:animate style="animation-duration:{Math.max(24, channels.length * 4)}s">
                <div class="mset" bind:this={firstSetEl}>
                  {#each channels as c (c)}
                    <button class="creator" class:dim={channel && channel !== c} onclick={() => (channel === c ? clearChannel() : pickChannel(c))} title={c}>
                      <span class="cav">{#if chanAvatar(c)}<img src={chanAvatar(c)} alt={c} />{:else}<span class="ci">{initials(c)}</span>{/if}</span>
                      <span class="cname" class:strong={channel === c}>{c}</span>
                    </button>
                  {/each}
                </div>
                {#if animate}
                  <div class="mset" aria-hidden="true">
                    {#each channels as c (c + '#dup')}
                      <button class="creator" class:dim={channel && channel !== c} tabindex="-1" onclick={() => (channel === c ? clearChannel() : pickChannel(c))}>
                        <span class="cav">{#if chanAvatar(c)}<img src={chanAvatar(c)} alt="" />{:else}<span class="ci">{initials(c)}</span>{/if}</span>
                        <span class="cname" class:strong={channel === c}>{c}</span>
                      </button>
                    {/each}
                  </div>
                {/if}
              </div>
            </div>
          {/if}
        </div>
      {/if}

      {#if channelsView}
        <!-- ══ TODOS LOS CANALES (cuadrícula; sin tira ni vídeos) ══ -->
        {#if shownChannels.length}
          <div class="chgrid">
            {#each shownChannels as c (c.name)}
              <div class="chwrap">
                <button class="chcard" onclick={() => pickChannel(c.name)}>
                  <span class="chav">
                    {#if c.avatar}<img src={c.avatar} alt={c.name} loading="lazy" />{:else}<span class="chi">{initials(c.name)}</span>{/if}
                  </span>
                  <span class="chn">{c.name}</span>
                  <span class="chc">{c.count} {c.count === 1 ? t('moments.count.one') : t('moments.count.other')}</span>
                </button>
                <button class="fav-star" class:on={isFave(c.name)} title={isFave(c.name) ? t('moments.unfave') : t('moments.saveChannel')} aria-label={t('moments.saveChannel')} onclick={() => toggleFave(c.name)}>
                  <svg viewBox="0 0 24 24" width="16" height="16" fill={isFave(c.name) ? 'currentColor' : 'none'} stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"><path d="M12 4l2.4 5 5.6.6-4.2 3.8 1.2 5.6L12 16.8 6.9 19l1.2-5.6L4 9.6 9.6 9z"/></svg>
                </button>
              </div>
            {/each}
          </div>
        {:else}
          <div class="hint">{t('moments.noChannels')}</div>
        {/if}
      {:else}
        <!-- ══ INICIO: contenido + card lateral (Favoritos / Limpiar historial) ══ -->
        <div class="home">
          <div class="home-main">
            {#if channel !== ''}
              <div class="chead">
                <h2 class="sec">{channel}</h2>
                <button class="savebtn" class:on={isFave(channel)} onclick={() => toggleFave(channel)}>
                  <svg viewBox="0 0 24 24" width="15" height="15" fill={isFave(channel) ? 'currentColor' : 'none'} stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"><path d="M12 4l2.4 5 5.6.6-4.2 3.8 1.2 5.6L12 16.8 6.9 19l1.2-5.6L4 9.6 9.6 9z"/></svg>
                  {isFave(channel) ? t('moments.saved') : t('moments.save')}
                </button>
              </div>
              {#if channelVideos.length}
                <div class="cards">{#each channelVideos as it (it.id)}{@render videoCard(it)}{/each}</div>
              {:else}<div class="hint">{t('moments.noneInChannel')}</div>{/if}
            {:else}
              {#if continueWatching.length && !videoSearch.q.trim()}
                <h2 class="sec">{t('moments.continueWatching')}</h2>
                <div class="cards">{#each continueWatching as it (it.id)}{@render videoCard(it)}{/each}</div>
              {/if}
              <h2 class="sec">{t('moments.recent')}</h2>
              {#if recentVideos.length}
                <div class="cards">{#each recentVideos as it (it.id)}{@render videoCard(it)}{/each}</div>
              {:else}<div class="hint">{t('moments.noVideos')}</div>{/if}
            {/if}
          </div>

          <aside class="home-side">
            <div class="sidecard">
              <h3>{t('moments.favorites')}</h3>
              {#if favedChannels.length}
                <div class="favlist">
                  {#each favedChannels as f (f.name)}
                    <button class="favrow" onclick={() => pickChannel(f.name)}>
                      <span class="favav">{#if f.avatar}<img src={f.avatar} alt="" />{:else}<span class="favi">{initials(f.name)}</span>{/if}</span>
                      <span class="favname">{f.name}</span>
                    </button>
                  {/each}
                </div>
              {:else}
                <p class="favhint">{t('moments.favHint')}</p>
              {/if}
            </div>
            <button class="clearbtn" onclick={clearHistory} disabled={!continueWatching.length}>
              <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6M10 11v5M14 11v5"/></svg>
              {t('moments.clearHistory')}
            </button>
          </aside>
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .yp { flex: 1; min-width: 0; height: 100%; overflow-y: auto; background: var(--ground); color: var(--ink); }
  .pane { max-width: 1600px; margin: 0 auto; padding: 24px 26px 60px; }
  .err { font-size: 13px; color: #da6b74; background: color-mix(in srgb,#da6b74 12%,transparent); border: 1px solid color-mix(in srgb,#da6b74 30%,var(--border)); border-radius: 8px; padding: 10px 12px; margin-top: 20px; }
  .hint { color: var(--muted); font-size: 14px; padding: 40px 4px; text-align: center; }
  .empty { text-align: center; padding: 64px 20px; color: var(--muted); }
  .empty .ei { font-size: 46px; } .empty .et { font-size: 17px; font-weight: 700; color: var(--ink); margin: 12px 0 6px; }
  .empty .eb { font-size: 13.5px; max-width: 440px; margin: 0 auto; line-height: 1.55; }

  .ic { stroke: currentColor; stroke-width: 1.7; fill: none; stroke-linecap: round; stroke-linejoin: round; flex: none; }

  .pheadwrap { margin-bottom: 22px; }

  /* ── Tira de canales + botón cuadrado "Todos"/"Volver" ── */
  .creators { display: flex; align-items: flex-start; gap: 16px; margin: 18px 0 26px; }
  /* Botón píldora con el texto dentro: fondo oscuro (--ink, gris muy oscuro en
     claro, NO negro puro) + texto claro (--ground), como la original. */
  .tggl { flex: none; display: inline-flex; align-items: center; gap: 7px; padding: 9px 16px; border-radius: 999px; background: var(--ink); border: 1px solid var(--ink); color: var(--ground); font-size: 12.5px; font-weight: 600; white-space: nowrap; cursor: pointer; align-self: flex-start; margin-top: 21px; transition: filter .14s; }
  .tggl:hover { filter: brightness(1.12); }

  .mviewport { flex: 1; min-width: 0; overflow: hidden; padding-top: 8px; }
  .mviewport.on { -webkit-mask-image: linear-gradient(to right, transparent, #000 5%, #000 95%, transparent); mask-image: linear-gradient(to right, transparent, #000 5%, #000 95%, transparent); }
  .mtrack { display: flex; width: max-content; }
  .mtrack.animate { animation-name: creatorsflow; animation-timing-function: linear; animation-iteration-count: infinite; }
  .mviewport:hover .mtrack.animate { animation-play-state: paused; }
  @keyframes creatorsflow { from { transform: translateX(0); } to { transform: translateX(-50%); } }
  @media (prefers-reduced-motion: reduce) { .mtrack.animate { animation: none; } }
  .mset { display: flex; gap: 22px; padding-right: 22px; flex: none; }
  .creator { flex: none; width: 66px; display: flex; flex-direction: column; align-items: center; gap: 7px; background: none; border: 0; padding: 0; color: inherit; cursor: pointer; transition: opacity .18s, transform .14s; }
  .creator:hover { transform: translateY(-2px); }
  .creator.dim { opacity: .45; }
  .cav { width: 54px; height: 54px; border-radius: 999px; overflow: hidden; display: grid; place-items: center; background: linear-gradient(150deg, var(--accent), var(--accent-2)); }
  .cav img { width: 100%; height: 100%; object-fit: cover; }
  .ci { color: #fff; font-weight: 650; font-size: 17px; }
  .cname { max-width: 66px; font-size: 11px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; text-align: center; }
  .cname.strong { color: var(--ink); font-weight: 600; }

  .sec { font-size: 16px; font-weight: 720; letter-spacing: -.2px; margin: 28px 2px 14px; }
  .sec:first-of-type { margin-top: 4px; }

  /* ── Inicio: dos columnas (card lateral IZQUIERDA + contenido) ── */
  .home { display: grid; grid-template-columns: 250px minmax(0, 1fr); gap: 28px; align-items: start; }
  @media (max-width: 1040px) { .home { grid-template-columns: 1fr; } .home-side { position: static; order: 0; } }
  .home-main { min-width: 0; }
  /* order:-1 la coloca a la izquierda aunque vaya después en el DOM (en móvil, tras el contenido).
     margin-top alinea el borde superior de la card con el primer vídeo (bajo el título de sección). */
  .home-side { order: -1; position: sticky; top: 8px; margin-top: 42px; display: flex; flex-direction: column; gap: 14px; }
  @media (max-width: 1040px) { .home-side { margin-top: 0; } }
  .sidecard { border: 1px solid var(--border); border-radius: 14px; background: var(--card); padding: 14px 12px 10px; box-shadow: var(--shadow); }
  .sidecard h3 { font-size: 11px; letter-spacing: .06em; text-transform: uppercase; color: var(--faint); font-weight: 650; margin: 2px 4px 10px; }
  .favlist { display: flex; flex-direction: column; gap: 2px; }
  .favrow { display: flex; align-items: center; gap: 10px; width: 100%; padding: 6px 8px; border-radius: 9px; background: none; border: 0; color: inherit; cursor: pointer; text-align: left; }
  .favrow:hover { background: var(--raise); }
  .favav { width: 30px; height: 30px; flex: none; border-radius: 999px; overflow: hidden; display: grid; place-items: center; background: linear-gradient(150deg, var(--accent), var(--accent-2)); }
  .favav img { width: 100%; height: 100%; object-fit: cover; }
  .favi { color: #fff; font-weight: 650; font-size: 12px; }
  .favname { font-size: 13px; color: var(--ink-dim); font-weight: 550; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .favhint { font-size: 12px; color: var(--muted); line-height: 1.5; padding: 2px 4px 8px; }
  .favhint .star { color: var(--accent); }
  .clearbtn { display: inline-flex; align-items: center; justify-content: center; gap: 8px; padding: 11px; border-radius: 12px; background: var(--card); border: 1px solid var(--border); color: var(--ink-dim); font-size: 13px; font-weight: 600; cursor: pointer; transition: border-color .14s, color .14s; }
  .clearbtn:hover:not(:disabled) { border-color: color-mix(in srgb,#da6b74 50%,var(--border)); color: #da6b74; }
  .clearbtn:disabled { opacity: .5; cursor: default; }

  .chead { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin: 4px 2px 14px; }
  .chead .sec { margin: 0; }
  .savebtn { display: inline-flex; align-items: center; gap: 7px; padding: 8px 14px; border-radius: 999px; background: var(--card); border: 1px solid var(--border); color: var(--ink-dim); font-size: 12.5px; font-weight: 600; cursor: pointer; }
  .savebtn:hover { border-color: color-mix(in srgb,var(--accent) 45%,var(--border)); color: var(--accent); }
  .savebtn.on { background: color-mix(in srgb,var(--accent) 14%,transparent); border-color: color-mix(in srgb,var(--accent) 40%,var(--border)); color: var(--accent); }

  /* ── Cuadrícula de canales ── */
  .chgrid { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 20px 14px; }
  .chwrap { position: relative; }
  .fav-star { position: absolute; top: 8px; right: 8px; width: 30px; height: 30px; display: grid; place-items: center; border-radius: 999px; background: color-mix(in srgb,var(--ground) 65%,transparent); border: 0; color: var(--muted); cursor: pointer; opacity: 0; transition: opacity .14s, color .14s; }
  .chwrap:hover .fav-star, .fav-star.on { opacity: 1; }
  .fav-star.on, .fav-star:hover { color: var(--accent); }
  .chcard { display: flex; flex-direction: column; align-items: center; gap: 10px; padding: 18px 10px; border-radius: 14px; background: none; border: 0; color: inherit; cursor: pointer; transition: background .14s, transform .14s; }
  .chcard:hover { background: var(--card); transform: translateY(-3px); }
  .chav { width: 76px; height: 76px; border-radius: 999px; overflow: hidden; display: grid; place-items: center; background: linear-gradient(150deg, var(--accent), var(--accent-2)); box-shadow: var(--shadow); }
  .chav img { width: 100%; height: 100%; object-fit: cover; }
  .chi { color: #fff; font-weight: 650; font-size: 26px; }
  .chn { font-size: 13.5px; font-weight: 620; color: var(--ink); text-align: center; max-width: 140px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .chc { font-size: 11.5px; color: var(--muted); }

  /* ── Tarjetas de vídeo ── */
  .cards { display: grid; grid-template-columns: repeat(auto-fill,minmax(258px,1fr)); gap: 22px 18px; }
  .card { cursor: pointer; display: flex; flex-direction: column; gap: 11px; text-align: left; background: none; border: 0; padding: 0; color: inherit; }
  .poster { position: relative; aspect-ratio: 16/9; border-radius: 13px; overflow: hidden; border: 1px solid var(--border); box-shadow: var(--shadow); transition: transform .14s, border-color .14s; background: radial-gradient(130% 130% at 28% 18%,#3a2f6e 0%,#1a1740 42%,#07070f 100%); }
  .poster.img { background: #000; }
  .poster img { width: 100%; height: 100%; object-fit: cover; }
  .card:hover .poster { transform: translateY(-3px); border-color: color-mix(in srgb,var(--accent) 45%,var(--border)); }
  .poster .dur { position: absolute; right: 8px; bottom: 8px; background: rgba(0,0,0,.82); color: #fff; font-size: 11.5px; font-weight: 600; padding: 2px 6px; border-radius: 6px; font-variant-numeric: tabular-nums; }
  .poster .play { position: absolute; inset: 0; display: grid; place-items: center; opacity: 0; transition: opacity .14s; }
  .card:hover .poster .play { opacity: 1; }
  .poster .play span { width: 52px; height: 52px; border-radius: 999px; background: rgba(10,10,15,.55); display: grid; place-items: center; border: 1px solid rgba(255,255,255,.25); }
  .poster .prog { position: absolute; left: 0; right: 0; bottom: 0; height: 4px; background: rgba(255,255,255,.28); }
  .poster .prog i { display: block; height: 100%; background: var(--accent); }
  .cmeta { display: flex; gap: 11px; }
  .av { width: 34px; height: 34px; flex: none; border-radius: 999px; display: grid; place-items: center; color: #fff; font-weight: 650; font-size: 13px; background: linear-gradient(150deg, var(--accent), var(--accent-2)); overflow: hidden; }
  .av.img { background: none; }
  .av img { width: 100%; height: 100%; object-fit: cover; }
  .ct h3 { font-size: 14.5px; font-weight: 600; line-height: 1.33; letter-spacing: -.15px; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
  .ct .by { color: var(--muted); font-size: 12.5px; margin-top: 4px; }
  .ct .st { color: var(--faint); font-size: 12px; margin-top: 1px; }
</style>
