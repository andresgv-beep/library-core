<script>
  // Ficha de vídeo de MOMENTS — maquetación propia,
  // watch view. App aparte de Archives: aquí NO se usa la plantilla de Archives.
  // Solo el CONTENIDO (stage + relacionados); el chrome (pestañas/navbar) lo pone
  // el reader, igual que ItemPage — nada de "navegador dentro de navegador".
  import { getItem, getCollectionItems, getCollections, translateSegments } from './libraryApi.js';
  import { saveVideoProgress, getVideoProgress } from './videoProgress.svelte.js';
  import { toggleFave, isFave } from './channelFaves.svelte.js';
  import { t } from './i18n.svelte.js';
  import { downloadMedia } from './auth.svelte.js';
  import Icon from './Icon.svelte';
  import MomentsHeader from './MomentsHeader.svelte';
  import { untrack } from 'svelte';
  import { tstate, targetLang, nativeLang, norm2, detectLang } from './translate.svelte.js';

  let { tab, onOpenItem, onOpenView } = $props();

  let item = $state(null);
  let loading = $state(true);
  let related = $state([]);
  // true cuando "Relacionados" no encontró nada afín y solo estamos rellenando
  // con recientes → la cabecera pasa a "Más vídeos" (no mentir con "Relacionados").
  let relFallback = $state(false);
  let videoEl = $state(null);
  // Reproductor custom (barra de progreso con capítulos).
  let playerEl = $state(null), seekEl = $state(null);
  let playing = $state(false), curTime = $state(0), dur = $state(0);
  // buffering: el <video> disparó play (playing=true) pero está esperando datos/
  // decodificando (evento `waiting`) → sin esto el botón queda en "pausa" y el
  // frame congelado parece un cuelgue. Mostramos spinner mientras dura.
  let buffering = $state(false);
  let muted = $state(false), vol = $state(1), ccOn = $state(false), isFs = $state(false);
  let seeking = $state(false), hoverPct = $state(null);

  async function load(id) {
    loading = true;
    try {
      item = await getItem(id);
    } catch (e) {
      item = tab.open ? { title: tab.open.title, open: tab.open } : null;
    }
    loading = false;
    loadRelated();
  }
  // Palabras genéricas que NO deben crear afinidad (si no, dos títulos con "official
  // video" saldrían "relacionados" sin serlo). es+en.
  const STOP = new Set(['official', 'oficial', 'video', 'vídeo', 'audio', 'lyric', 'lyrics',
    'letra', 'feat', 'featuring', 'live', 'full', 'the', 'and', 'for', 'with', 'con', 'los',
    'las', 'del', 'que', 'this', 'that', 'from', 'http', 'https', 'www', 'com']);
  const words = (s) => (s || '').toLowerCase().split(/[^a-záéíóúñü0-9]+/)
    .filter((w) => w.length > 2 && !STOP.has(w));
  // Relacionados por RELEVANCIA (no "todo"): puntúa cada vídeo por tags
  // compartidas + palabras del título + autor/canal común. Solo entran los que tienen
  // relación (score>0); si no hubiera ninguno, cae a recientes para no quedar vacío,
  // y en ese caso la cabecera pasa a "Más vídeos" (relFallback).
  async function loadRelated() {
    relFallback = false;
    if (!item?.id) { related = []; return; }
    try {
      const cols = (await getCollections()).filter((c) => c.kind === 'media');
      const groups = await Promise.all(cols.map((c) =>
        getCollectionItems(c.id).then((list) => list.map((it) => ({ ...it, _cid: c.id }))).catch(() => [])
      ));
      const myTags = new Set((item.tags || []).map((t) => t.toLowerCase()));
      const myWords = new Set(words(item.title));
      const myAuthors = new Set((item.authors || []).flatMap((a) => words(a)));
      const seen = new Set([item.id]);
      const cands = [];
      for (const r of groups.flat()) {
        if (r.source?.provider !== 'moments' || seen.has(r.id)) continue;
        seen.add(r.id);
        const tagOverlap = (r.tags || []).reduce((n, t) => n + (myTags.has(t.toLowerCase()) ? 1 : 0), 0);
        const titleOverlap = words(r.title).reduce((n, w) => n + (myWords.has(w) ? 1 : 0), 0);
        const authorOverlap = (r.authors || []).flatMap((a) => words(a)).reduce((n, w) => n + (myAuthors.has(w) ? 1 : 0), 0);
        const sameChannel = r._cid === item.collectionId ? 1 : 0;
        const score = tagOverlap * 10 + authorOverlap * 8 + sameChannel * 6 + titleOverlap * 3;
        cands.push({ r, score, date: r.date || '' });
      }
      const byScore = (a, b) => (b.score - a.score) || b.date.localeCompare(a.date);
      const byDate = (a, b) => b.date.localeCompare(a.date);
      const relevant = cands.filter((c) => c.score > 0).sort(byScore);
      relFallback = relevant.length === 0;
      const chosen = relevant.length ? relevant : cands.sort(byDate);
      related = chosen.slice(0, 40).map((c) => c.r);
    } catch (e) { related = []; relFallback = false; }
  }

  let lastId = '';
  $effect(() => {
    const id = tab.itemId || tab.open?.itemId || '';
    if (id && id !== lastId) { lastId = id; load(id); }
  });

  const open = $derived(item?.open || tab.open || {});
  const url = $derived(open.url || '');

  // Al cambiar de vídeo (p. ej. clic en Relacionados) el <source src> cambia pero
  // el <video> NO recarga solo → hay que llamar a load(). Sin esto seguía el vídeo
  // anterior aunque el título/descripción sí cambiaban.
  let lastURL = '';
  $effect(() => {
    const u = url;
    if (videoEl && u && u !== lastURL) { lastURL = u; videoEl.load(); }
  });
  const title = $derived(item?.title || open.title || tab.title || '');
  const author = $derived((item?.authors || []).join(', '));
  const date = $derived(item?.date || '');
  const description = $derived(item?.description || '');
  const tags = $derived(item?.tags || []);
  const duration = $derived(item?.duration || 0);
  const cover = $derived(item?.preview?.kind === 'image' ? item.preview.url : '');
  const subtitles = $derived(item?.subtitles || []);
  const chapters = $derived(item?.chapters || []);
  const sourceURL = $derived(item?.source?.originalUrl || '');
  const channelAvatar = $derived(item?.channelAvatar || '');

  const initials = (s) => (s || '?').trim().split(/\s+/).slice(0, 2).map((w) => w[0] || '').join('').toUpperCase();
  function hms(s) {
    s = Math.max(0, Math.floor(s || 0));
    const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), ss = String(s % 60).padStart(2, '0');
    return h ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`;
  }
  // Clic en un capítulo → salta a su segundo en el vídeo.
  function seek(sec) {
    if (videoEl) { videoEl.currentTime = sec; videoEl.play?.().catch(() => {}); }
  }

  // Progreso ("Seguir viendo"): guarda mientras se ve (throttle 4s) y en pausa/fin;
  // restaura la posición al cargar los metadatos.
  let lastSave = 0;
  function onTime() {
    const v = videoEl; if (!v) return;
    curTime = v.currentTime;
    if (!v.duration) return;
    const now = Date.now();
    if (now - lastSave > 4000) { lastSave = now; saveVideoProgress(item?.id, v.currentTime, v.duration); }
  }
  function flushProgress() {
    const v = videoEl; if (v && v.duration) saveVideoProgress(item?.id, v.currentTime, v.duration);
  }
  function onMeta() {
    dur = videoEl?.duration || 0;
    const p = getVideoProgress(item?.id);
    if (p && videoEl && videoEl.duration && p.t < videoEl.duration - 2) videoEl.currentTime = p.t;
  }

  // ── Controles propios: la barra nativa del <video> no admite marcas de capítulo
  // (shadow DOM), así que montamos un seek segmentado por capítulos + play/tiempo/
  // volumen/subtítulos/pantalla completa. Cada tramo = un capítulo; se rellena solo.
  const pct = $derived(dur ? (curTime / dur) * 100 : 0);
  const chapterSegs = $derived.by(() => {
    const d = dur || 0;
    if (!d) return [];
    const cs = (chapters && chapters.length) ? chapters : [{ start: 0, title: '' }];
    return cs.map((c, i) => {
      const start = Math.max(0, c.start || 0);
      const end = i + 1 < cs.length ? cs[i + 1].start : d;
      return { title: c.title, start, end, leftPct: (start / d) * 100, widthPct: (Math.max(0, end - start) / d) * 100 };
    });
  });
  const segFill = (s) => Math.min(100, Math.max(0, ((curTime - s.start) / Math.max(0.001, s.end - s.start)) * 100));
  const curChapter = $derived(chapters.length ? (chapters.filter((c) => (c.start || 0) <= curTime + 0.25).slice(-1)[0] || null) : null);
  const hoverChapter = $derived(hoverPct == null ? null : (chapterSegs.find((s) => hoverPct >= s.leftPct && hoverPct <= s.leftPct + s.widthPct) || null));

  function togglePlay() { const v = videoEl; if (!v) return; if (v.paused) v.play().catch(() => {}); else v.pause(); }
  function pctFromEvent(e) { const r = seekEl.getBoundingClientRect(); return Math.min(1, Math.max(0, (e.clientX - r.left) / r.width)); }
  function seekDown(e) { if (!videoEl || !dur) return; seeking = true; seekEl.setPointerCapture?.(e.pointerId); const f = pctFromEvent(e); videoEl.currentTime = f * dur; curTime = f * dur; }
  function seekMove(e) { if (!seekEl) return; hoverPct = pctFromEvent(e) * 100; if (seeking && videoEl && dur) { const f = hoverPct / 100; videoEl.currentTime = f * dur; curTime = f * dur; } }
  function seekUp(e) { seeking = false; try { seekEl.releasePointerCapture?.(e.pointerId); } catch (err) {} }
  function toggleMute() { const v = videoEl; if (!v) return; v.muted = !v.muted; muted = v.muted; }
  function setVol(e) { const v = videoEl; if (!v) return; vol = +e.currentTarget.value; v.volume = vol; v.muted = vol === 0; muted = v.muted; }
  function toggleCC() { const tt = videoEl?.textTracks; if (tt && tt[0]) { tt[0].mode = tt[0].mode === 'showing' ? 'disabled' : 'showing'; ccOn = tt[0].mode === 'showing'; } }
  function toggleFs() { if (!playerEl) return; if (!document.fullscreenElement) playerEl.requestFullscreen?.(); else document.exitFullscreen?.(); }
  $effect(() => {
    const onFs = () => (isFs = !!document.fullscreenElement);
    document.addEventListener('fullscreenchange', onFs);
    return () => document.removeEventListener('fullscreenchange', onFs);
  });

  const relImg = (r) => (r?.preview?.kind === 'image' ? r.preview.url : '');
  const relSub = (r) => [(r.authors || []).join(', '), r.date].filter(Boolean).join(' · ');

  // ── Traducción del contenido (título + descripción), igual que en los ZIM ──
  // Origen = idioma de los subtítulos si los hay; si no, el shim asume "en".
  let tActive = $state(false), tBusy = $state(false), badgeOn = $state(false);
  let tTitle = $state(''), tDesc = $state('');
  let tCtrl = null, badgeTimer = null, lastRun = 0, lastOrig = 0, lastTid = '';
  const displayTitle = $derived(tActive && tTitle ? tTitle : title);
  const displayDesc = $derived(tActive && tDesc ? tDesc : description);
  const srcLang = $derived(norm2((subtitles[0]?.lang || '').slice(0, 2)));
  function flashBadge() { badgeOn = true; clearTimeout(badgeTimer); badgeTimer = setTimeout(() => (badgeOn = false), 3000); }
  async function runTranslate(to, auto = false) {
    const segs = [];
    if (title) segs.push({ id: 'title', text: title });
    if (description) segs.push({ id: 'desc', text: description });
    if (!to || !segs.length) return;
    const src = srcLang || detectLang(`${title} ${description}`);
    if (src && src === to) return;   // ya está en el idioma destino
    if (auto && !src) return;        // en modo auto solo si conocemos el origen
    tCtrl?.abort(); tCtrl = new AbortController();
    tBusy = true;
    try {
      const out = await translateSegments({ lib: '', path: item?.id || '', to, sourceHint: src, html: false, segments: segs }, { signal: tCtrl.signal });
      const by = new Map(out.map((x) => [x.id, x.text]));
      tTitle = by.get('title') || ''; tDesc = by.get('desc') || '';
      tActive = true; flashBadge();
    } catch (e) { /* abortada o error → se queda el original */ }
    tBusy = false;
  }
  function showOriginal() { tCtrl?.abort(); tActive = false; }
  $effect(() => {
    const id = item?.id || '';
    untrack(() => {
      if (id === lastTid) return; lastTid = id;
      tActive = false; tTitle = ''; tDesc = ''; tBusy = false;
      if (id && tstate.auto) runTranslate(nativeLang(), true);
    });
  });
  $effect(() => { const r = tstate.run; untrack(() => { if (r === lastRun) return; lastRun = r; runTranslate(targetLang()); }); });
  $effect(() => { const o = tstate.original; untrack(() => { if (o === lastOrig) return; lastOrig = o; showOriginal(); }); });
</script>

{#if loading && !item}
  <div class="yw-state">{t('common.loading')}</div>
{:else if !url}
  <div class="yw-state">{title || t('watch.unavailable')}</div>
{:else}
  <div class="yw scroll">
    <div class="tbadge-anchor">
      {#if tBusy}
        <div class="tbadge"><span class="tspin"></span> {t('translate.working')}</div>
      {:else if tActive && badgeOn}
        <div class="tbadge done"><Icon name="translate" size={13} /> {t('translate.done')}</div>
      {/if}
    </div>
    <div class="ywhead">
      <MomentsHeader onHome={() => onOpenView?.('moments')} onSubmit={() => onOpenView?.('moments')} />
    </div>
    <div class="watch">
      <div class="stage">
        <div class="player" bind:this={playerEl} class:playing>
          <!-- svelte-ignore a11y_media_has_caption -->
          <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
          <video bind:this={videoEl} poster={cover} preload="auto"
                 ontimeupdate={onTime} onloadedmetadata={onMeta} onclick={togglePlay}
                 onplay={() => (playing = true)} onpause={() => { playing = false; buffering = false; flushProgress(); }}
                 onwaiting={() => (buffering = true)} onplaying={() => (buffering = false)}
                 oncanplay={() => (buffering = false)} onstalled={() => (buffering = true)}
                 onended={() => { playing = false; buffering = false; flushProgress(); }}>
            <source src={url} type="video/mp4" />
            {#each subtitles as s (s.lang)}
              <track kind="subtitles" src={s.url} srclang={s.lang} label={s.lang.toUpperCase()} />
            {/each}
          </video>

          {#if buffering}
            <div class="vspin-wrap" aria-hidden="true"><span class="vspin"></span></div>
          {:else if !playing}
            <button class="bigplay" onclick={togglePlay} aria-label="Play">
              <svg viewBox="0 0 24 24" width="30" height="30" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>
            </button>
          {/if}

          <div class="vctrl">
            <div class="seek" bind:this={seekEl} onpointerdown={seekDown} onpointermove={seekMove} onpointerup={seekUp} onpointerleave={() => (hoverPct = null)}
                 role="slider" tabindex="0" aria-label={t('item.trackPosition')} aria-valuemin="0" aria-valuemax="100" aria-valuenow={Math.round(pct)}>
              {#each chapterSegs as s}
                <div class="seg" class:cur={curChapter && s.title === curChapter.title} style="left:{s.leftPct}%;width:calc({s.widthPct}% - 3px)">
                  <div class="seg-fill" style="width:{segFill(s)}%"></div>
                </div>
              {/each}
              <div class="playhead" style="left:{pct}%"></div>
              {#if hoverChapter && hoverChapter.title}
                <div class="chaptip" style="left:{Math.min(92, Math.max(8, hoverPct))}%">{hoverChapter.title}</div>
              {/if}
            </div>
            <div class="vrow">
              <button class="vbtn" onclick={togglePlay} aria-label="Play/Pausa">
                {#if playing}<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M6 5h4v14H6zM14 5h4v14h-4z"/></svg>
                {:else}<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>{/if}
              </button>
              <button class="vbtn" onclick={toggleMute} aria-label="Silenciar">
                {#if muted || vol === 0}<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M4 9v6h4l5 5V4L8 9z"/><path d="M16 9l5 5M21 9l-5 5" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round"/></svg>
                {:else}<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M4 9v6h4l5 5V4L8 9z"/><path d="M16 8a5 5 0 010 8" stroke="currentColor" stroke-width="2" fill="none"/></svg>{/if}
              </button>
              <input class="vvol" type="range" min="0" max="1" step="0.05" value={muted ? 0 : vol} oninput={setVol} aria-label="Volumen" />
              <span class="vtime">{hms(curTime)} / {hms(dur)}</span>
              {#if curChapter && curChapter.title}<span class="vchap">· {curChapter.title}</span>{/if}
              <span class="vspring"></span>
              {#if subtitles.length}
                <button class="vbtn" class:on={ccOn} onclick={toggleCC} aria-label="Subtítulos">
                  <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.6"><rect x="3" y="5" width="18" height="14" rx="2"/><path d="M7 12h3M13 12h4M7 15h5M13 15h3" stroke-width="1.5" stroke-linecap="round"/></svg>
                </button>
              {/if}
              <button class="vbtn" onclick={toggleFs} aria-label="Pantalla completa">
                <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5"/></svg>
              </button>
            </div>
          </div>
        </div>

        <h1 class="v-title">{displayTitle}</h1>

        <div class="v-bar">
          <div class="chan">
            <div class="av" class:img={channelAvatar}>
              {#if channelAvatar}<img src={channelAvatar} alt={author} />{:else}{initials(author || 'Moments')}{/if}
            </div>
            <div>
              <div class="cn">{author || 'Moments'}</div>
              <div class="cs">{[date, duration ? hms(duration) : ''].filter(Boolean).join(' · ')}</div>
            </div>
            {#if author}
              <button class="chfav" class:on={isFave(author)} onclick={() => toggleFave(author)}
                      title={isFave(author) ? t('moments.unfave') : t('moments.saveChannel')} aria-label={t('moments.saveChannel')}>
                <svg viewBox="0 0 24 24" width="19" height="19" fill={isFave(author) ? 'currentColor' : 'none'} stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"><path d="M12 4l2.4 5 5.6.6-4.2 3.8 1.2 5.6L12 16.8 6.9 19l1.2-5.6L4 9.6 9.6 9z"/></svg>
              </button>
            {/if}
          </div>
          <div class="acts">
            <button class="act pri" type="button" onclick={() => downloadMedia(url, (title || 'video') + '.mp4')}>
              <svg class="ic" viewBox="0 0 24 24"><path d="M12 3v12M7 10l5 5 5-5M5 21h14"/></svg>{t('item.download')}
            </button>
          </div>
        </div>

        <div class="desc">
          {#if date || duration}
            <div class="m">{#if date}{date}{/if}{#if duration}<span>·</span> {hms(duration)}{/if}</div>
          {/if}
          {#if description}<p>{displayDesc}</p>{/if}
          {#if tags.length}<div class="tags">{#each tags as tg}<span class="tag">{tg}</span>{/each}</div>{/if}
        </div>

      </div>

      <aside class="side">
        {#if related.length}
          <div class="relframe">
            <h2>{relFallback ? t('item.moreVideos') : t('item.related')}</h2>
            <div class="rellist scroll">
              {#each related as r (r.id)}
                <button class="rel" onclick={() => onOpenItem?.(r.id)}>
                  <div class="rp" class:img={relImg(r)}>
                    {#if relImg(r)}<img src={relImg(r)} alt={r.title} loading="lazy" />{/if}
                  </div>
                  <div>
                    <h3>{r.title}</h3>
                    <div class="rby">{relSub(r)}</div>
                  </div>
                </button>
              {/each}
            </div>
          </div>
        {/if}
      </aside>
    </div>
  </div>
{/if}

<style>
  .yw-state { flex: 1; display: flex; align-items: center; justify-content: center; color: var(--muted); }
  .yw { flex: 1; min-width: 0; height: 100%; overflow-y: auto; background: var(--ground); color: var(--ink); }
  /* Badge de traducción (flota arriba-derecha, visible al hacer scroll) */
  .tbadge-anchor { position: sticky; top: 0; z-index: 6; height: 0; }
  .tbadge { position: absolute; top: 12px; right: 18px; display: flex; align-items: center; gap: 7px; padding: 7px 12px; border-radius: 20px; background: var(--card); border: 1px solid var(--border); box-shadow: var(--shadow); color: var(--ink-dim); font-size: 12.5px; font-weight: 520; }
  .tbadge.done { color: var(--accent-2); }
  .tbadge.done :global(.ic) { color: var(--accent-2); }
  .tspin { width: 13px; height: 13px; border-radius: 50%; border: 2px solid var(--border); border-top-color: var(--accent); animation: tspin .7s linear infinite; }
  @keyframes tspin { to { transform: rotate(360deg); } }
  .ywhead { max-width: 1600px; margin: 0 auto; padding: 18px 26px 0; }
  .watch { max-width: 1600px; margin: 0 auto; padding: 12px 26px 60px; display: grid; grid-template-columns: minmax(0, 1fr) 372px; gap: 30px; align-items: start; }
  @media (max-width: 1080px) { .watch { grid-template-columns: 1fr; } .side { position: static; } }

  .ic { width: 16px; height: 16px; stroke: currentColor; stroke-width: 1.7; fill: none; stroke-linecap: round; stroke-linejoin: round; flex: none; }

  /* Player real 16:9 */
  .player { position: relative; aspect-ratio: 16/9; border-radius: 16px; overflow: hidden; border: 1px solid var(--border); box-shadow: var(--shadow); background: #000; }
  .player video { width: 100%; height: 100%; display: block; background: #000; object-fit: contain; cursor: pointer; }

  /* Botón grande de play cuando está pausado */
  .bigplay { position: absolute; inset: 0; margin: auto; width: 66px; height: 66px; border-radius: 999px; background: rgba(0,0,0,.5); color: #fff; display: grid; place-items: center; border: 1px solid rgba(255,255,255,.28); cursor: pointer; transition: background .14s, transform .14s; }
  .bigplay:hover { background: rgba(0,0,0,.68); transform: scale(1.05); }

  /* Spinner de buffering: el <video> disparó play pero está esperando datos. */
  .vspin-wrap { position: absolute; inset: 0; display: grid; place-items: center; pointer-events: none; }
  .vspin { width: 46px; height: 46px; border-radius: 50%; border: 3px solid rgba(255,255,255,.25); border-top-color: #fff; animation: vspin .8s linear infinite; }
  @keyframes vspin { to { transform: rotate(360deg); } }

  /* Barra de controles propia (aparece al pasar el ratón; siempre visible en pausa) */
  .vctrl { position: absolute; left: 0; right: 0; bottom: 0; padding: 6px 14px 9px; background: linear-gradient(transparent, rgba(0,0,0,.78)); display: flex; flex-direction: column; gap: 4px; opacity: 0; transition: opacity .18s; }
  .player:hover .vctrl, .player:focus-within .vctrl, .player:not(.playing) .vctrl { opacity: 1; }

  .seek { position: relative; height: 16px; display: flex; align-items: center; cursor: pointer; touch-action: none; }
  .seg { position: absolute; top: 50%; transform: translateY(-50%); height: 4px; background: rgba(255,255,255,.32); border-radius: 2px; overflow: hidden; transition: height .1s; }
  .seek:hover .seg { height: 6px; }
  .seg.cur { background: rgba(255,255,255,.42); }
  .seg-fill { height: 100%; background: var(--accent); }
  .playhead { position: absolute; top: 50%; width: 13px; height: 13px; border-radius: 50%; background: var(--accent); transform: translate(-50%, -50%); box-shadow: 0 0 0 2px rgba(0,0,0,.35); pointer-events: none; opacity: 0; transition: opacity .1s; }
  .seek:hover .playhead { opacity: 1; }
  .chaptip { position: absolute; bottom: 20px; transform: translateX(-50%); background: rgba(0,0,0,.88); color: #fff; font-size: 12px; font-weight: 550; padding: 4px 9px; border-radius: 7px; white-space: nowrap; pointer-events: none; max-width: 260px; overflow: hidden; text-overflow: ellipsis; }

  .vrow { display: flex; align-items: center; gap: 8px; color: #fff; }
  .vbtn { width: 32px; height: 32px; display: grid; place-items: center; color: #fff; background: none; border: 0; cursor: pointer; border-radius: 7px; transition: background .12s; }
  .vbtn:hover { background: rgba(255,255,255,.16); }
  .vbtn.on { color: var(--accent-2); }
  .vvol { width: 78px; accent-color: var(--accent); cursor: pointer; }
  .vtime { font-size: 12.5px; font-variant-numeric: tabular-nums; color: #fff; margin-left: 2px; }
  .vchap { font-size: 12.5px; color: rgba(255,255,255,.75); max-width: 240px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .vspring { flex: 1; }

  .v-title { font-size: 21px; font-weight: 720; letter-spacing: -.3px; line-height: 1.3; margin: 18px 0 12px; }

  .v-bar { display: flex; flex-wrap: wrap; align-items: center; gap: 14px; justify-content: space-between; }
  .chan { display: flex; align-items: center; gap: 11px; }
  .chan .av { width: 42px; height: 42px; flex: none; border-radius: 999px; display: grid; place-items: center; color: #fff; font-weight: 650; font-size: 15px; background: linear-gradient(150deg, var(--accent), var(--accent-2)); overflow: hidden; }
  .chan .av.img { background: none; }
  .chan .av img { width: 100%; height: 100%; object-fit: cover; }
  .chan .cn { font-size: 14px; font-weight: 620; }
  .chan .cs { font-size: 12px; color: var(--muted); }
  .chan .chfav { margin-left: 6px; width: 36px; height: 36px; flex: none; border-radius: 999px; display: grid; place-items: center; background: none; border: 0; color: var(--muted); cursor: pointer; transition: color .14s, background .14s; }
  .chan .chfav:hover { background: var(--raise); color: var(--accent); }
  .chan .chfav.on { color: var(--accent); }
  .acts { display: flex; gap: 8px; flex-wrap: wrap; }
  .act { display: inline-flex; align-items: center; gap: 7px; padding: 9px 14px; border-radius: 999px; background: var(--card); border: 1px solid var(--border); font-size: 13px; font-weight: 550; color: var(--ink-dim); }
  .act:hover { background: var(--raise); border-color: color-mix(in srgb, var(--accent) 35%, var(--border)); }
  .act.pri { background: color-mix(in srgb, var(--accent) 16%, transparent); border-color: color-mix(in srgb, var(--accent) 40%, var(--border)); color: var(--ink); }
  .act.pri .ic { color: var(--accent); }

  .desc { margin-top: 16px; background: var(--card); border: 1px solid var(--border); border-radius: 14px; padding: 15px 17px; }
  .desc .m { display: flex; gap: 14px; flex-wrap: wrap; font-size: 12.5px; font-weight: 600; color: var(--ink-dim); margin-bottom: 9px; }
  .desc .m span { color: var(--muted); font-weight: 500; }
  .desc p { color: var(--ink-dim); font-size: 13.5px; line-height: 1.6; white-space: pre-wrap; word-break: break-word; }
  .tags { display: flex; flex-wrap: wrap; gap: 7px; margin-top: 13px; }
  .tag { font-size: 11.5px; font-weight: 600; color: var(--accent-2); background: color-mix(in srgb, var(--accent) 13%, transparent); border: 1px solid color-mix(in srgb, var(--accent) 26%, transparent); padding: 4px 9px; border-radius: 7px; }


  /* Relacionados: marco fijo (acotado a la ventana, así siempre ≥ el vídeo) con
     la lista scrolleando DENTRO — si no, con muchos vídeos la lista no tendría fin. */
  .side { position: sticky; top: 20px; align-self: start; }
  .relframe { display: flex; flex-direction: column; min-height: 0; height: calc(100vh - 130px); border: 1px solid var(--border); border-radius: 14px; background: var(--card); padding: 12px 8px 10px 12px; box-shadow: var(--shadow); }
  .side h2 { flex: none; font-size: 13px; font-weight: 650; color: var(--ink); margin: 2px 2px 10px; }
  .rellist { flex: 1; min-height: 0; overflow-y: auto; display: flex; flex-direction: column; gap: 6px; padding-right: 4px; }
  .rellist::-webkit-scrollbar { width: 8px; }
  .rellist::-webkit-scrollbar-thumb { background: var(--border); border-radius: 8px; }
  .rel { display: grid; grid-template-columns: 132px 1fr; gap: 11px; padding: 7px; border-radius: 12px; cursor: pointer; background: none; border: 0; color: inherit; text-align: left; }
  .rel:hover { background: var(--raise); }
  .rel .rp { position: relative; aspect-ratio: 16/9; border-radius: 9px; overflow: hidden; border: 1px solid var(--border); background: linear-gradient(150deg, #2d496a, #111822); }
  .rel .rp.img { background: #000; }
  .rel .rp img { width: 100%; height: 100%; object-fit: cover; }
  .rel h3 { font-size: 13px; font-weight: 600; line-height: 1.32; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
  .rel .rby { font-size: 11.5px; color: var(--muted); margin-top: 4px; }
</style>
