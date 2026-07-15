<script>
  // Ficha de contenido de CABINET (archivo documental) — maquetación editorial
  // (readerpage / videopage / audiopage): CABECERA a lo
  // ancho (título serif + autor + resumen + descargar) → REJILLA visor | sideinfo
  // (Sobre esta pieza / Datos / Temas / Formatos / Procedencia). El audiolibro usa su
  // banda propia (portada + motor con onda + lista de pistas). Identidad editorial
  // propia (serif + paleta cálida). Moments es OTRA app; aquí NO se mezcla.
  import Icon from './Icon.svelte';
  import { tick, untrack } from 'svelte';
  import { getItem, getCollectionItems, fmtSize, translateSegments } from './libraryApi.js';
  import { t } from './i18n.svelte.js';
  import { downloadMedia } from './auth.svelte.js';
  import { tstate, targetLang, nativeLang, norm2, detectLang } from './translate.svelte.js';

  let { tab, onOpenItem } = $props();

  let item = $state(null);
  let loading = $state(true);
  let related = $state([]);
  let selectedFile = $state(null);
  let trackIdx = $state(0);   // pista activa del audiolibro
  let audioEl = $state(null); // <audio> del reproductor de pistas
  let videoEl = $state(null); // <video> (para saltar a capítulos)
  let viewerEl = $state(null); // contenedor del visor (pantalla completa)

  async function load(id) {
    loading = true;
    try {
      item = await getItem(id);
      selectedFile = null;
      trackIdx = 0;
    } catch (e) {
      item = tab.open ? { title: tab.open.title, open: tab.open, kind: tab.open.mode } : null;
    }
    loading = false;
    loadRelated();
  }
  async function loadRelated() {
    const cid = item?.collectionId;
    if (!cid) { related = []; return; }
    try {
      const list = await getCollectionItems(cid);
      const seen = new Set([item.id]);
      related = list.filter((r) => (seen.has(r.id) ? false : (seen.add(r.id), true))).slice(0, 8);
    } catch (e) { related = []; }
  }

  let lastId = '';
  $effect(() => {
    const id = tab.itemId || tab.open?.itemId || '';
    if (id && id !== lastId) { lastId = id; load(id); }
  });

  const open = $derived(item?.open || tab.open || {});
  const baseMode = $derived(open.mode || 'unsupported');
  // Solo lo descargado: la app se consume 100% offline, así que los formatos que se
  // quedaron en una fuente remota no se listan — verlos o bajarlos
  // exigiría internet (y el lector pdf.js además choca con CORS). Lo que existe en la
  // fuente sigue accesible por "Ver fuente" en Procedencia.
  const files = $derived((item?.files || []).filter((f) => f.local));
  const fileMode = (f) => {
    const name = ((f?.name || '') + ' ' + (f?.url || '')).toLowerCase();
    if (/\.(mp4|webm|m4v|mov)$/.test(name)) return 'video';
    if (/\.(mp3|ogg|oga|flac|m4a|wav)$/.test(name)) return 'audio';
    if (/\.(jpe?g|png|gif|webp)$/.test(name)) return 'image';
    if (/\.pdf$/.test(name)) return 'pdf';
    return 'unsupported';
  };
  const mode = $derived(selectedFile ? fileMode(selectedFile) : baseMode);
  const url = $derived(selectedFile?.url || open.url || '');
  const title = $derived(item?.title || open.title || tab.title || '');
  const authors = $derived(item?.authors || []);
  const author = $derived(authors.join(', '));
  const date = $derived(item?.date || '');
  const description = $derived(item?.description || '');
  const tags = $derived(item?.tags || []);
  const source = $derived(item?.source?.provider || '');
  const sourceURL = $derived(item?.source?.originalUrl || '');
  const kind = $derived(item?.kind || mode);
  const language = $derived(item?.language || '');
  const contributor = $derived(item?.contributor || '');
  const license = $derived(item?.license || '');
  const duration = $derived(item?.duration || 0);
  const chapters = $derived(item?.chapters || []);

  // ── Audiolibro multi-pista (portada + motor con onda + lista) ──
  const coverURL = $derived(item?.preview?.kind === 'image' ? item.preview.url : '');
  const tracks = $derived(item?.tracks || []);
  const isAudiobook = $derived(isAudio && tracks.length > 1);
  const curTrack = $derived(tracks[trackIdx] || null);
  const trackURL = $derived(curTrack?.url || url);
  const trackWave = $derived(curTrack?.waveform || '');
  let progress = $state(0);
  async function selectTrack(i) {
    trackIdx = i;
    progress = 0;
    await tick();
    audioEl?.play?.().catch(() => {});
  }
  function onAudioTime() {
    const a = audioEl;
    progress = a && a.duration ? a.currentTime / a.duration : 0;
  }
  function seekWave(e) {
    const rect = e.currentTarget.getBoundingClientRect();
    const frac = Math.min(1, Math.max(0, (e.clientX - rect.left) / rect.width));
    if (audioEl && audioEl.duration) { audioEl.currentTime = frac * audioEl.duration; progress = frac; }
  }
  function viewFile(f) {
    if (isAudiobook) {
      const i = tracks.findIndex((tr) => tr.url === f.url);
      if (i >= 0) { selectTrack(i); return; }
    }
    selectedFile = f;
  }
  function seekVideo(sec) { if (videoEl) { videoEl.currentTime = sec; videoEl.play?.().catch(() => {}); } }
  function toggleFs() { const el = viewerEl; if (!el) return; if (!document.fullscreenElement) el.requestFullscreen?.(); else document.exitFullscreen?.(); }
  function hms(s) {
    s = Math.max(0, Math.floor(s || 0));
    const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), ss = String(s % 60).padStart(2, '0');
    return h ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`;
  }

  const isVideo = $derived(mode === 'video');
  const isAudio = $derived(mode === 'audio');
  const isImage = $derived(mode === 'image');
  const isPdf = $derived(mode === 'pdf' || /\.pdf($|\?)/i.test(url));
  const isDoc = $derived(!isVideo && !isAudio && !isImage);

  // Lector PDF (PDF.js) cargado bajo demanda → no infla el bundle principal; solo se
  // descarga su chunk al abrir un PDF.
  let PdfComp = $state(null);
  $effect(() => { if (isPdf && !PdfComp) import('./PdfReader.svelte').then((m) => (PdfComp = m.default)).catch(() => {}); });

  const KIND_KEY = { video: 'cabinet.kind.video', audio: 'cabinet.kind.audio', image: 'item.kind.image', pdf: 'item.kind.text', document: 'item.kind.text' };
  const kindLabel = (k) => t(KIND_KEY[k] || 'cabinet.kind.doc');
  const fileName = $derived(selectedFile?.name || decodeURIComponent((url.split('/').pop() || '').replace(/\+/g, ' ')));
  const viewable = (f) => fileMode(f) !== 'unsupported';
  const sourceLabel = $derived(source === 'cabinet' || source === 'local' || source === 'manual' ? '' : source);
  const summary = $derived([kindLabel(kind), language, sourceLabel].filter(Boolean).join(' · '));

  // Relacionados
  const relMode = (r) => r?.open?.mode || r?.kind;
  const relImg = (r) => (r?.preview?.kind === 'image' ? r.preview.url : '');
  const relArt = (r) => (relMode(r) === 'video' ? 'wide' : relMode(r) === 'image' ? 'sq' : 'book');
  const relSub = (r) => [(r.authors || []).join(', '), r.date].filter(Boolean).join(' · ');

  // ── Traducción del contenido (título + descripción) ──
  let tActive = $state(false), tBusy = $state(false), badgeOn = $state(false);
  let tTitle = $state(''), tDesc = $state('');
  let tCtrl = null, badgeTimer = null, lastRun = 0, lastOrig = 0, lastTid = '';
  const displayTitle = $derived(tActive && tTitle ? tTitle : title);
  const displayDesc = $derived(tActive && tDesc ? tDesc : description);
  function langHint(v) {
    const s = (v || '').toLowerCase();
    if (/engl|ingl|eng\b/.test(s) || s === 'en') return 'en';
    if (/span|espa|castell|spa\b/.test(s) || s === 'es') return 'es';
    return norm2(v);
  }
  function flashBadge() { badgeOn = true; clearTimeout(badgeTimer); badgeTimer = setTimeout(() => (badgeOn = false), 3000); }
  async function runTranslate(to, auto = false) {
    const segs = [];
    if (title) segs.push({ id: 'title', text: title });
    if (description) segs.push({ id: 'desc', text: description });
    if (!to || !segs.length) return;
    const src = langHint(language) || detectLang(`${title} ${description}`);
    if (src && src === to) return;
    if (auto && !src) return;
    tCtrl?.abort(); tCtrl = new AbortController();
    tBusy = true;
    try {
      const out = await translateSegments({ lib: '', path: item?.id || '', to, sourceHint: src, html: false, segments: segs }, { signal: tCtrl.signal });
      const by = new Map(out.map((x) => [x.id, x.text]));
      tTitle = by.get('title') || ''; tDesc = by.get('desc') || '';
      tActive = true; flashBadge();
    } catch (e) { /* abortada o error */ }
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

{#snippet sideinfo()}
  <aside class="sideinfo">
    {#if description}
      <section class="infosection"><h3>{t('item.aboutPiece')}</h3><p class="description">{displayDesc}</p></section>
    {/if}
    <section class="infosection">
      <h3>{t('item.biblioData')}</h3>
      <dl class="metagrid">
        <dt>{t('cabinet.type')}</dt><dd>{kindLabel(kind)}</dd>
        {#if author}<dt>{t('item.author')}</dt><dd>{author}</dd>{/if}
        {#if contributor}<dt>{t('item.contributor')}</dt><dd>{contributor}</dd>{/if}
        {#if date}<dt>{t('cabinet.date')}</dt><dd>{date}</dd>{/if}
        {#if language}<dt>{t('item.language')}</dt><dd>{language}</dd>{/if}
        {#if duration}<dt>{t('item.duration')}</dt><dd>{hms(duration)}</dd>{/if}
        {#if license}<dt>{t('item.license')}</dt><dd>{license}</dd>{/if}
      </dl>
    </section>
    {#if tags.length}
      <section class="infosection"><h3>{t('cabinet.themes')}</h3><div class="tags">{#each tags as tg}<span class="tag">{tg}</span>{/each}</div></section>
    {/if}
    {#if files.length}
      <section class="infosection">
        <h3>{t('item.filesFormats')}</h3>
        <div class="filelist">
          {#each files as f (f.name)}
            <div class="filechoice" class:active={f.url === url}>
              <div class="fc-meta"><b>{f.format || f.name.split('.').pop()?.toUpperCase()}</b><small>{f.name}{#if f.size} · {fmtSize(f.size)}{/if}{#if f.local} · {t('item.local')}{/if}</small></div>
              <div class="fc-actions">
                {#if viewable(f)}<button onclick={() => viewFile(f)}>{f.url === url ? t('item.viewing') : t('item.view')}</button>{/if}
                <button class="dlbtn" onclick={() => downloadMedia(f.url, f.name)} title={t('item.download')}>↓</button>
              </div>
            </div>
          {/each}
        </div>
      </section>
    {/if}
    {#if source || sourceURL}
      <section class="infosection">
        <h3>{t('item.provenance')}</h3>
        <div class="source"><span>{sourceLabel || '—'}</span>{#if sourceURL}<a href={sourceURL} target="_blank" rel="noopener">{t('item.viewSourceArrow')}</a>{/if}</div>
      </section>
    {/if}
  </aside>
{/snippet}

{#if loading && !item}
  <div class="ip-state">{t('common.loading')}</div>
{:else if !url}
  <div class="ip-state"><Icon name="close" size={22} /><p>{title || t('item.unavailable')}</p></div>
{:else}
  <div class="ip scroll">
    <div class="tbadge-anchor">
      {#if tBusy}
        <div class="tbadge"><span class="tspin"></span> {t('translate.working')}</div>
      {:else if tActive && badgeOn}
        <div class="tbadge done"><Icon name="translate" size={13} /> {t('translate.done')}</div>
      {/if}
    </div>
    <div class="wrap">
      <!-- ══ CABECERA a lo ancho ══ -->
      <header class="itemhead">
        <div class="ih-main">
          <h1>{displayTitle}</h1>
          {#if author}<div class="ih-author">{date ? `${author} · ${date}` : author}</div>{/if}
          {#if summary}<div class="ih-summary">{summary}</div>{/if}
        </div>
      </header>

      {#if isAudiobook}
        <!-- ══ AUDIOLIBRO: portada + motor con onda ══ -->
        <section class="audiohero">
          <div class="albumcover" class:img={coverURL}>
            {#if coverURL}<img src={coverURL} alt={title} />{:else}<b>{title}</b>{#if author}<span>{author}</span>{/if}{/if}
          </div>
          <div class="audioengine">
            <span class="now">{t('item.nowPlaying')} · {trackIdx + 1}/{tracks.length}</span>
            <h2>{curTrack?.title || title}</h2>
            {#if author}<p>{author}</p>{/if}
            <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
            <div class="wave" style={trackWave ? `background-image:url('${trackWave}')` : ''} class:nowave={!trackWave} onclick={seekWave} role="slider" aria-label={t('item.trackPosition')} aria-valuenow={Math.round(progress * 100)} aria-valuemin="0" aria-valuemax="100" tabindex="0">
              {#if !trackWave}<Icon name="note" size={30} />{/if}
              <div class="wave-played" style="width:{progress * 100}%"></div>
              <div class="wave-head" style="left:{progress * 100}%"></div>
            </div>
            <!-- svelte-ignore a11y_media_has_caption -->
            <audio class="engine-audio" bind:this={audioEl} controls src={trackURL} ontimeupdate={onAudioTime}></audio>
          </div>
        </section>
        <!-- ══ Lista de pistas | sideinfo ══ -->
        <div class="trackarea">
          <main class="tracklist">
            <h2>{t('item.tracksHeading')}</h2>
            <div class="tracks-scroll scroll">
              {#each tracks as tr, i (i)}
                <button class="track" class:current={i === trackIdx} onclick={() => selectTrack(i)}>
                  <span class="tracknum">{#if i === trackIdx}▶{:else}{String(i + 1).padStart(2, '0')}{/if}</span>
                  <span class="tk-title">{tr.title}</span>
                  <span class="tk-fmt">MP3</span>
                </button>
              {/each}
            </div>
          </main>
          {@render sideinfo()}
        </div>
      {:else}
        <!-- ══ VISOR | sideinfo ══ -->
        <div class="readgrid">
          <div class="vcol">
            {#if isPdf}
              <!-- Lector propio (PDF.js) con barra editorial, cargado bajo demanda -->
              {#if PdfComp}{#key url}<PdfComp {url} title={fileName} />{/key}
              {:else}<div class="viewer"><div class="pload"><span class="pspin"></span> {t('common.loading')}</div></div>{/if}
            {:else}
            <section class="viewer" class:media={isVideo || isImage} bind:this={viewerEl}>
              {#if isVideo}
                <!-- svelte-ignore a11y_media_has_caption -->
                <video bind:this={videoEl} controls src={url}></video>
              {:else if isImage}
                <div class="paperstage"><img src={url} alt={title} /></div>
              {:else if isAudio}
                <div class="paperstage"><div class="audiosingle"><Icon name="note" size={30} /><audio controls src={url}></audio></div></div>
              {:else}
                <div class="viewerbar">
                  <span class="vfile">{fileName}</span>
                  <span class="pages"></span>
                  <button onclick={toggleFs} title={t('item.fullscreen')} aria-label={t('item.fullscreen')}>⛶</button>
                </div>
                <iframe title={title} src={url}></iframe>
              {/if}
            </section>
            {/if}
            {#if isVideo && chapters.length}
              <section class="chapters">
                <h2>{t('watch.chapters')}</h2>
                {#each chapters as c}
                  <button class="chapter" onclick={() => seekVideo(c.start)}>
                    <time>{hms(c.start)}</time>
                    <b>{c.title}</b>
                  </button>
                {/each}
              </section>
            {/if}
          </div>
          {@render sideinfo()}
        </div>
      {/if}

      {#if related.length}
        <h2 class="row">{t('item.related')}</h2>
        <div class="rail">
          {#each related as r (r.id)}
            <button class="card" onclick={() => onOpenItem?.(r.id)}>
              <div class="art {relArt(r)}" class:img={relImg(r)}>
                {#if relImg(r)}<img src={relImg(r)} alt={r.title} loading="lazy" />{:else}<span class="type">{kindLabel(relMode(r))}</span>{/if}
              </div>
              <div class="meta"><div class="t">{r.title}</div><div class="s">{relSub(r)}</div></div>
            </button>
          {/each}
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  /* Identidad editorial de Cabinet (paleta cálida propia + serif) */
  .ip {
    --a-bg: #101114; --a-panel: #17181d; --a-panel2: #1d1f25; --a-line: #2a2d35;
    --a-ink: #f2efe8; --a-dim: #aaa69e; --a-faint: #77746f;
    --a-accent: #e0a867; --a-accent2: #89a898;
    --a-serif: Georgia, "Times New Roman", serif;
    flex: 1; min-width: 0; height: 100%; overflow-y: auto; background: var(--a-bg); color: var(--a-ink); font-size: 14px;
  }
  .ip-state { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; color: var(--muted); }
  .ip-state :global(.ic) { color: #e88; }

  /* Badge de traducción */
  .tbadge-anchor { position: sticky; top: 0; z-index: 6; height: 0; }
  .tbadge { position: absolute; top: 12px; right: 18px; display: flex; align-items: center; gap: 7px; padding: 7px 12px; border-radius: 20px; background: var(--a-panel); border: 1px solid var(--a-line); box-shadow: 0 10px 26px rgba(0,0,0,.4); color: var(--a-dim); font-size: 12.5px; font-weight: 520; }
  .tbadge.done { color: var(--a-accent2); }
  .tbadge.done :global(.ic) { color: var(--a-accent2); }
  .tspin { width: 13px; height: 13px; border-radius: 50%; border: 2px solid var(--a-line); border-top-color: var(--a-accent); animation: tspin .7s linear infinite; }
  @keyframes tspin { to { transform: rotate(360deg); } }

  .wrap { max-width: 1480px; margin: 0 auto; padding: 20px 28px 70px; }

  /* ── Cabecera ── */
  .itemhead { margin: 6px 0 24px; padding-bottom: 20px; border-bottom: 1px solid var(--a-line); }
  .itemhead h1 { font-family: var(--a-serif); font-weight: 400; font-size: clamp(26px,3vw,34px); line-height: 1.1; margin: 0 0 8px; }
  .ih-author { color: var(--a-accent); font-size: 13.5px; }
  .ih-summary { color: var(--a-faint); font-size: 12px; margin-top: 9px; }

  /* ── Rejilla visor | sideinfo ── */
  .readgrid { display: grid; grid-template-columns: minmax(0,1fr) 320px; gap: 24px; align-items: start; }
  @media (max-width: 900px) { .readgrid { grid-template-columns: 1fr; } .trackarea { grid-template-columns: 1fr; } .sideinfo { position: static; } }
  .vcol { min-width: 0; }

  .viewer { min-width: 0; background: #292b30; border: 1px solid var(--a-line); border-radius: 4px; overflow: hidden; }
  .pload { height: 78vh; min-height: 460px; display: flex; align-items: center; justify-content: center; gap: 10px; color: var(--a-dim); font-size: 13.5px; }
  .pspin { width: 15px; height: 15px; border-radius: 50%; border: 2px solid var(--a-line); border-top-color: var(--a-accent); animation: tspin .7s linear infinite; }
  .viewer.media { background: #000; }
  .viewer video { width: 100%; max-height: 74vh; display: block; background: #000; }
  .viewerbar { height: 48px; display: flex; align-items: center; gap: 9px; padding: 0 13px; background: #1b1c21; border-bottom: 1px solid var(--a-line); color: var(--a-dim); font-size: 12px; }
  .vfile { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .viewerbar .pages { flex: 1; }
  .viewerbar button { width: 32px; height: 30px; border: 1px solid var(--a-line); background: var(--a-panel); color: var(--a-ink); border-radius: 6px; cursor: pointer; }
  .viewer iframe { width: 100%; height: 78vh; min-height: 460px; border: 0; display: block; background: #fff; }
  .paperstage { min-height: 440px; padding: 24px; display: grid; place-items: center; background: radial-gradient(circle at center,#3c3f46,#26282d); }
  .paperstage img { max-width: 100%; max-height: 76vh; object-fit: contain; display: block; box-shadow: 0 18px 42px rgba(0,0,0,.45); }
  .audiosingle { display: flex; align-items: center; gap: 16px; background: var(--a-panel); border: 1px solid var(--a-line); border-radius: 10px; padding: 22px 24px; width: min(560px,100%); color: var(--a-accent); }
  .audiosingle audio { flex: 1; min-width: 0; }

  /* Capítulos (vídeo) */
  .chapters { margin-top: 22px; }
  .chapters h2 { font-family: var(--a-serif); font-weight: 400; font-size: 22px; margin: 0 0 12px; }
  .chapter { width: 100%; display: grid; grid-template-columns: 84px 1fr; gap: 14px; align-items: center; padding: 12px 8px; border-top: 1px solid var(--a-line); cursor: pointer; background: none; border-left: 0; border-right: 0; border-bottom: 0; text-align: left; color: var(--a-ink); }
  .chapter:hover { background: linear-gradient(90deg,transparent,var(--a-panel),transparent); }
  .chapter time { color: var(--a-accent); font-size: 12px; }
  .chapter b { font-size: 13.5px; font-weight: 600; }

  /* ── Sideinfo ── */
  .sideinfo { position: sticky; top: 14px; min-width: 0; }
  .infosection { padding: 0 0 18px; margin-bottom: 18px; border-bottom: 1px solid var(--a-line); }
  .infosection:last-child { border-bottom: 0; }
  .infosection h3 { font-size: 12.5px; text-transform: uppercase; letter-spacing: .14em; color: var(--a-accent); margin: 0 0 12px; }
  .description { color: var(--a-dim); font-size: 15px; line-height: 1.65; white-space: pre-wrap; }
  .metagrid { display: grid; grid-template-columns: 92px 1fr; gap: 9px 12px; font-size: 15px; }
  .metagrid dt { color: var(--a-faint); } .metagrid dd { margin: 0; color: var(--a-dim); min-width: 0; word-break: break-word; }
  .tags { display: flex; flex-wrap: wrap; gap: 6px; }
  .tag { font-size: 13.5px; color: var(--a-dim); border: 1px solid var(--a-line); background: var(--a-panel); border-radius: 99px; padding: 6px 10px; }
  .filelist { display: flex; flex-direction: column; }
  .filechoice { display: grid; grid-template-columns: 1fr auto; gap: 10px; align-items: center; padding: 10px 0; border-top: 1px solid var(--a-line); }
  .filechoice:first-child { border-top: 0; }
  .filechoice.active .fc-meta b { color: var(--a-accent); }
  .fc-meta { min-width: 0; } .fc-meta b { display: block; font-size: 14.5px; color: var(--a-ink); } .fc-meta small { color: var(--a-faint); font-size: 13.5px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; display: block; }
  .fc-actions { display: flex; align-items: center; gap: 6px; }
  .fc-actions button, .fc-actions a { border: 1px solid var(--a-line); background: var(--a-panel); color: var(--a-dim); border-radius: 6px; padding: 7px 9px; font-size: 13.5px; cursor: pointer; }
  .fc-actions button:hover, .fc-actions a:hover { border-color: var(--a-accent); color: var(--a-ink); }
  .source { font-size: 15px; color: var(--a-dim); display: flex; justify-content: space-between; gap: 10px; align-items: center; }
  .source a { color: var(--a-accent); white-space: nowrap; }

  /* ── Audiolibro ── */
  .audiohero { display: grid; grid-template-columns: minmax(180px,220px) minmax(0,1fr); gap: 26px; align-items: stretch; margin-bottom: 24px; isolation: isolate; overflow: hidden; }
  @media (max-width: 680px) { .audiohero { grid-template-columns: 1fr; } .albumcover { width: 180px; } }
  .albumcover { aspect-ratio: 1; width: 100%; max-width: 220px; min-width: 0; border: 1px solid #716657; border-radius: 4px; overflow: hidden; box-shadow: 0 14px 30px rgba(0,0,0,.35); background: linear-gradient(155deg,#e7dfcb,#c9bea3); color: #8c2428; padding: 22px; text-align: center; font-family: var(--a-serif); display: flex; flex-direction: column; justify-content: center; }
  .albumcover.img { padding: 0; }
  .albumcover img { width: 100%; height: 100%; object-fit: cover; }
  .albumcover b { font-size: 21px; line-height: 1.05; }
  .albumcover span { color: #283e63; font-size: 15px; margin-top: 8px; }
  .audioengine { min-width: 0; max-width: 100%; overflow: hidden; border: 1px solid var(--a-line); background: var(--a-panel); border-radius: 4px; display: flex; flex-direction: column; justify-content: center; padding: 26px; }
  .audioengine > * { min-width: 0; max-width: 100%; }
  .now { color: var(--a-accent); font-size: 11px; text-transform: uppercase; letter-spacing: .13em; }
  .audioengine h2 { font-family: var(--a-serif); font-size: 25px; font-weight: 400; margin: 8px 0 4px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .audioengine p { color: var(--a-faint); font-size: 12px; margin: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .wave { position: relative; height: 118px; margin: 22px 0 16px; width: 100%; border-radius: 8px; background: #0e0f14 center/100% 86% no-repeat; border: 1px solid var(--a-line); display: grid; place-items: center; color: var(--a-faint); cursor: pointer; overflow: hidden; }
  .wave.nowave { background: linear-gradient(150deg,var(--a-accent),var(--a-accent2)); color: rgba(255,255,255,.85); cursor: default; }
  .wave-played { position: absolute; left: 0; top: 0; bottom: 0; background: color-mix(in srgb, var(--a-accent) 30%, transparent); pointer-events: none; }
  .wave.nowave .wave-played, .wave.nowave .wave-head { display: none; }
  .wave-head { position: absolute; top: 0; bottom: 0; width: 2px; margin-left: -1px; background: var(--a-accent); box-shadow: 0 0 6px var(--a-accent); pointer-events: none; }
  .engine-audio { width: 100%; }

  .trackarea { display: grid; grid-template-columns: minmax(0,1fr) 320px; gap: 24px; align-items: start; }
  .tracklist { min-width: 0; }
  .tracklist h2 { font-family: var(--a-serif); font-size: 23px; font-weight: 400; margin: 0 0 11px; }
  .tracks-scroll { display: flex; flex-direction: column; max-height: 60vh; overflow-y: auto; padding-right: 10px; scrollbar-gutter: stable; scrollbar-width: thin; scrollbar-color: color-mix(in srgb, var(--a-accent) 55%, transparent) transparent; }
  .tracks-scroll::-webkit-scrollbar { width: 8px; }
  .tracks-scroll::-webkit-scrollbar-track { background: transparent; }
  .tracks-scroll::-webkit-scrollbar-thumb { background: color-mix(in srgb, var(--a-accent) 55%, transparent); border-radius: 8px; }
  .tracks-scroll::-webkit-scrollbar-thumb:hover { background: color-mix(in srgb, var(--a-accent) 80%, transparent); }
  .track { width: 100%; display: grid; grid-template-columns: 36px minmax(0,1fr) auto; gap: 12px; align-items: center; padding: 12px 8px; border-top: 1px solid var(--a-line); cursor: pointer; background: none; border-left: 0; border-right: 0; border-bottom: 0; text-align: left; color: var(--a-ink); }
  .track:first-child { border-top: 0; }
  .track:hover { background: var(--a-panel); }
  .track.current { background: var(--a-panel); border-left: 2px solid var(--a-accent); }
  .tracknum { color: var(--a-faint); font-size: 11px; font-variant-numeric: tabular-nums; text-align: center; }
  .track.current .tracknum { color: var(--a-accent); }
  .tk-title { font-size: 13px; line-height: 1.3; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
  .tk-fmt { color: var(--a-faint); font-size: 11px; }

  /* ── Relacionados ── */
  h2.row { font-family: var(--a-serif); font-size: 22px; font-weight: 400; margin: 34px 2px 14px; }
  .rail { display: flex; align-items: flex-start; gap: 16px; overflow-x: auto; padding: 4px 2px 12px; scrollbar-width: none; }
  .rail::-webkit-scrollbar { display: none; }
  /* flex column + rail flex-start → el póster (.art) queda pegado ARRIBA en todas las
     cards aunque el título/autor de abajo tenga distinto nº de líneas (el <button> por
     defecto centraba su contenido y desalineaba las carátulas). */
  .card { width: 158px; flex: none; display: flex; flex-direction: column; text-align: left; cursor: pointer; background: none; border: none; padding: 0; color: inherit; }
  .art { position: relative; border-radius: 3px; overflow: hidden; border: 1px solid var(--a-line); transition: transform .14s ease; }
  .card:hover .art { transform: translateY(-4px); }
  .art.book { aspect-ratio: 3/4; background: linear-gradient(150deg,#6d5334,#241a12); }
  .art.wide { aspect-ratio: 16/10; background: linear-gradient(150deg,#3a5236,#141d16); }
  .art.sq { aspect-ratio: 1/1; background: linear-gradient(150deg,#4a4636,#16150f); }
  .art.img { background: #0b0c0e; }
  .art.img img { width: 100%; height: 100%; object-fit: cover; }
  .art .type { position: absolute; top: 9px; left: 9px; font-size: 10px; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; padding: 3px 8px; border-radius: 20px; background: rgba(0,0,0,.42); color: #fff; }
  .meta { padding: 9px 3px 0; }
  .meta .t { font-size: 13px; font-weight: 600; color: var(--a-ink); line-height: 1.3; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
  .meta .s { font-size: 11.5px; color: var(--a-faint); margin-top: 3px; }
</style>
