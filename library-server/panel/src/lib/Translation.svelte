<script>
  import { onMount } from 'svelte'
  import { getLanguages, getStorage, translateAvailable, translateDownload, translateRemove } from './api.js'
  import { bytes } from './fmt.js'

  let langs = $state(null)
  let modelsSection = $state(null)
  let loading = $state(true)
  let flash = $state('')

  // panel "añadir idioma"
  let showAvail = $state(false)
  let avail = $state([])
  let availLoading = $state(false)
  let busy = $state({}) // id → true

  const NAME = {
    en: 'inglés', es: 'español', ca: 'catalán', fr: 'francés', de: 'alemán',
    it: 'italiano', pt: 'portugués', gl: 'gallego', eu: 'euskera', nl: 'neerlandés',
    ru: 'ruso', uk: 'ucraniano', pl: 'polaco', ar: 'árabe', zh: 'chino', cs: 'checo',
    bg: 'búlgaro', el: 'griego', et: 'estonio', is: 'islandés', mk: 'macedonio',
    mt: 'maltés', nb: 'noruego', nn: 'noruego', sl: 'esloveno', sq: 'albanés',
    tr: 'turco', hbs: 'serbocroata', eng: 'inglés',
  }
  const nm = (c) => NAME[c] || c

  async function load() {
    loading = true
    langs = await getLanguages()
    try {
      const st = await getStorage()
      modelsSection = (st.sections || []).find((s) => s.key === 'models') || null
    } catch { modelsSection = null }
    loading = false
  }
  onMount(load)

  async function toggleAvail() {
    showAvail = !showAvail
    if (showAvail && !avail.length) {
      availLoading = true
      try {
        const r = await translateAvailable()
        avail = r.models || []
      } catch (e) {
        flash = 'No se pudieron listar modelos disponibles: ' + (e.message || '')
      } finally {
        availLoading = false
      }
    }
  }

  async function doDownload(m) {
    busy = { ...busy, [m.id]: true }; flash = `Descargando ${m.id}…`
    try {
      const r = await translateDownload(m.id)
      if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || 'error')
      flash = `Añadido: ${m.id}`
      await load()
      avail = avail.map((x) => (x.id === m.id ? { ...x, installed: true } : x))
    } catch (e) {
      flash = `No se pudo descargar ${m.id}: ${e.message}`
    } finally {
      busy = { ...busy, [m.id]: false }
    }
  }

  async function doRemove(pair) {
    const id = `${pair.from}-${pair.to}-tiny` // pares instalados hoy son tiny
    if (!confirm(`Quitar el modelo ${pair.from} → ${pair.to}?`)) return
    busy = { ...busy, [id]: true }; flash = ''
    try {
      const r = await translateRemove(id)
      if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || 'error')
      flash = `Quitado: ${pair.from}→${pair.to}`
      await load()
      avail = avail.map((x) => (x.id === id ? { ...x, installed: false } : x))
    } catch (e) {
      flash = `No se pudo quitar: ${e.message}`
    } finally {
      busy = { ...busy, [id]: false }
    }
  }

  const pairs = $derived(langs?.pairs || [])
  const availSorted = $derived([...avail].sort((a, b) => Number(a.installed) - Number(b.installed) || a.id.localeCompare(b.id)))
</script>

<div class="toolbar">
  <span class="cnt">
    Traducción offline (Bergamot) ·
    {#if langs?.available}<b>{pairs.length}</b> modelos instalados{:else}motor no disponible{/if}
  </span>
  <span class="grow"></span>
  {#if flash}<span style="font-size:12px;color:var(--ink-mute)">{flash}</span>{/if}
  {#if langs?.available}
    <button class="btn btn-primary" onclick={toggleAvail}>{showAvail ? 'Cerrar' : '＋ Añadir idioma'}</button>
  {/if}
  <button class="btn" onclick={load} disabled={loading}>↻</button>
</div>

{#if loading}
  <div class="empty">Leyendo modelos…</div>
{:else if !langs?.available}
  <div class="empty">
    <div class="big">Motor de traducción desactivado</div>
    Arranca <code>translate-wrap</code> y define <code>TRANSLATE_URL</code> en el shim para gestionar modelos aquí.
  </div>
{:else}
  {#if showAvail}
    <div class="setcard">
      <h4>Modelos disponibles (online)</h4>
      {#if availLoading}
        <div style="color:var(--ink-faint);font-size:12.5px">Consultando catálogo…</div>
      {:else}
        {#each availSorted as m (m.id)}
          <div class="row" style="grid-template-columns:1fr auto;margin-bottom:6px;padding:9px 11px">
            <div style="min-width:0">
              <div class="cname" style="font-size:13px">{nm(m.from)} → {nm(m.to)} <span class="badge b-mute">{m.type}</span></div>
              <div class="cpath">{m.id}</div>
            </div>
            {#if m.installed}
              <span class="badge b-signal">instalado</span>
            {:else}
              <button class="btn btn-primary" disabled={busy[m.id]} onclick={() => doDownload(m)}>
                {busy[m.id] ? 'Descargando…' : 'Descargar'}
              </button>
            {/if}
          </div>
        {/each}
      {/if}
    </div>
  {/if}

  <div class="label">Instalados</div>
  {#if pairs.length}
    {#each pairs as p (p.from + '-' + p.to)}
      <div class="row" style="grid-template-columns:40px 1fr auto">
        <div class="cic" style="background:var(--magenta);color:#0a0a0c">文</div>
        <div style="min-width:0">
          <div class="cname">{nm(p.from)} → {nm(p.to)}</div>
          <div class="cpath">{p.from}-{p.to}-tiny</div>
        </div>
        <button class="btn" disabled={busy[`${p.from}-${p.to}-tiny`]} onclick={() => doRemove(p)}>
          {busy[`${p.from}-${p.to}-tiny`] ? '…' : 'Quitar'}
        </button>
      </div>
    {/each}
  {:else}
    <div class="empty">Sin modelos. Usa "Añadir idioma" para descargar un par.</div>
  {/if}

  <div class="setcard" style="margin-top:12px">
    <div class="setrow">
      <span>En disco (modelos + binario)</span>
      <code>{modelsSection?.exists ? bytes(modelsSection.bytes) : '—'}</code>
    </div>
  </div>
{/if}
