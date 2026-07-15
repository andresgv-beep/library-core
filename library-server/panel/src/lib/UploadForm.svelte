<script>
  // Formulario de carga manual / edición. source define la app y qué campos se
  // muestran: moments (vídeo) · cabinet (documento). Crea vía /api/admin/upload;
  // si recibe `item`, edita su ficha vía /api/admin/media/update (sin cambiar el fichero).
  import { uploadContent, updateContent } from './api.js'

  let { source = 'moments', item = null, onDone } = $props()
  const editing = !!item
  const src = editing ? item.source : source
  const isVideo = $derived(src === 'moments')

  let file = $state(null)
  let cover = $state(null)
  let channelAvatar = $state(null)
  let fileName = $state('')
  let title = $state(item?.title || '')
  let author = $state(item?.author || '')
  let collection = $state('')
  let tags = $state((item?.tags || []).join(', '))
  let description = $state(item?.description || '')
  let date = $state(item?.date || '')
  let duration = $state(item?.duration || '')
  let language = $state(item?.language || '')
  let contributor = $state(item?.contributor || '')
  let license = $state(item?.license || '')
  let access = $state(editing ? '' : 'open') // '' en edición = mantener nivel actual

  let busy = $state(false)
  let err = $state('')

  function pickFile(e) {
    file = e.currentTarget.files?.[0] || null
    fileName = file?.name || ''
    if (file && !title.trim()) title = file.name.replace(/\.[^.]+$/, '').replace(/[_-]+/g, ' ').trim()
  }
  function pickCover(e) { cover = e.currentTarget.files?.[0] || null }
  function pickAvatar(e) { channelAvatar = e.currentTarget.files?.[0] || null }

  async function submit(e) {
    e?.preventDefault()
    if (!editing && !file) { err = 'Elige un fichero'; return }
    if (!title.trim()) { err = 'Falta el título'; return }
    busy = true; err = ''
    const fields = { source: src, title, author, tags, description, access }
    if (!editing) fields.collection = collection
    if (isVideo) { fields.duration = duration }
    else { fields.date = date; fields.language = language; fields.contributor = contributor; fields.license = license }
    try {
      const files = { cover, channel_avatar: channelAvatar }
      const resp = editing
        ? await updateContent(item.id, fields, files)
        : await uploadContent(fields, { ...files, file })
      if (!resp.ok) { const b = await resp.json().catch(() => ({})); throw new Error(b.error || 'no se pudo guardar') }
      onDone?.()
    } catch (e) { err = e.message } finally { busy = false }
  }
</script>

<form class="uform" onsubmit={submit}>
  <div class="ulabel">{editing ? 'Editar' : 'Nueva importación'} · {isVideo ? 'Moments' : 'Cabinet'}</div>

  {#if editing}
    <div class="editfile"><b>Fichero:</b> {item.media}<span class="hint"> · el fichero no se cambia (para eso, elimina y vuelve a subir)</span></div>
  {:else}
    <label class="drop">
      <input type="file" accept={isVideo ? 'video/*' : '.pdf,.epub,.mp3,.ogg,.flac,.m4a,.wav,.jpg,.jpeg,.png,.gif,.webp,.txt,.md'} onchange={pickFile} />
      <svg class="ic" viewBox="0 0 24 24" style="width:22px;height:22px"><path d="M12 15V3M7 8l5-5 5 5M4 15v4a2 2 0 002 2h12a2 2 0 002-2v-4" /></svg>
      <span>{fileName || (isVideo ? 'Arrastra un vídeo o pulsa para elegir' : 'Arrastra un documento o pulsa para elegir')}</span>
    </label>
  {/if}

  <div class="ugrid">
    <label class="fld" style="grid-column:1 / -1">{isVideo ? 'Nombre' : 'Título'}<input type="text" bind:value={title} placeholder={isVideo ? 'Título del vídeo' : 'Título del documento'} /></label>

    <label class="fld">{isVideo ? 'Canal / autor' : 'Autor'}<input type="text" bind:value={author} /></label>
    {#if !editing}
      <label class="fld">Colección<input type="text" bind:value={collection} placeholder="General" /></label>
    {/if}

    {#if isVideo}
      <label class="fld">Duración (segundos)<input type="number" min="0" bind:value={duration} placeholder="opcional" /></label>
      <label class="fld">Tags<input type="text" bind:value={tags} placeholder="separados por comas" /></label>
    {:else}
      <label class="fld">Año<input type="text" bind:value={date} placeholder="p. ej. 1998" /></label>
      <label class="fld">Idioma<input type="text" bind:value={language} placeholder="Español" /></label>
      <label class="fld">Categorías<input type="text" bind:value={tags} placeholder="separadas por comas" /></label>
      <label class="fld">Licencia<input type="text" bind:value={license} placeholder="CC BY, dominio público…" /></label>
      <label class="fld">Contribuidor<input type="text" bind:value={contributor} placeholder="p. ej. una biblioteca" /></label>
    {/if}

    <label class="fld" style="grid-column:1 / -1">Descripción<textarea rows="2" bind:value={description}></textarea></label>

    <label class="fld">Visibilidad
      <select bind:value={access}>
        {#if editing}<option value="">(mantener actual)</option>{/if}
        <option value="open">Abierto · todos</option>
        <option value="login">Sesión · con cuenta</option>
        <option value="blocked">Bloqueado · solo admin</option>
      </select>
    </label>
    <label class="fld">{isVideo ? 'Miniatura del vídeo' : 'Portada'} {editing ? '(cambiar)' : '(opcional)'}
      <input type="file" accept="image/*" onchange={pickCover} />
    </label>
    {#if isVideo}
      <label class="fld">Logo del canal / autor {editing ? '(cambiar)' : '(opcional)'}
        <input type="file" accept="image/*" onchange={pickAvatar} />
      </label>
    {/if}
  </div>

  {#if err}<div class="uerr">{err}</div>{/if}

  <div class="uactions">
    <button type="button" class="btn" onclick={() => onDone?.()}>Cancelar</button>
    <button type="submit" class="btn btn-primary" disabled={busy}>
      {busy ? 'Guardando…' : editing ? 'Guardar cambios' : `Publicar en ${isVideo ? 'Moments' : 'Cabinet'}`}
    </button>
  </div>
</form>

<style>
  .uform { background: var(--canvas); border: 1px solid var(--signal-border); border-radius: 11px; padding: 16px; margin-bottom: 14px; }
  .ulabel { font-size: 12.5px; font-weight: 600; color: var(--ink-dim); margin-bottom: 14px; }
  .drop { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 18px; border: 1px dashed var(--line-strong); border-radius: 9px; color: var(--ink-mute); font-size: 12.5px; text-align: center; cursor: pointer; margin-bottom: 14px; }
  .drop:hover { border-color: var(--signal-border); color: var(--ink-dim); }
  .drop input[type=file] { display: none; }
  .drop .ic { color: var(--signal); }
  .editfile { font-size: 12px; color: var(--ink-dim); background: var(--window-bg); border: 1px solid var(--line-bright); border-radius: 8px; padding: 9px 11px; margin-bottom: 14px; }
  .editfile .hint { color: var(--ink-faint); }
  .ugrid { display: grid; grid-template-columns: 1fr 1fr; gap: 11px; }
  .fld { display: flex; flex-direction: column; gap: 5px; font-size: 11.5px; color: var(--ink-mute); }
  .fld input, .fld textarea, .fld select { background: var(--window-bg); border: 1px solid var(--line-bright); border-radius: 7px; padding: 8px 10px; font-size: 12.5px; color: var(--ink); font-family: inherit; resize: vertical; }
  .fld input:focus, .fld textarea:focus, .fld select:focus { border-color: var(--signal-border); outline: none; }
  .fld select { cursor: pointer; }
  .fld input[type=file] { padding: 6px; color: var(--ink-mute); font-size: 12px; }
  .uerr { margin-top: 12px; font-size: 12px; color: var(--crit); }
  .uactions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 16px; }
  @media (max-width: 620px) { .ugrid { grid-template-columns: 1fr; } }
</style>
