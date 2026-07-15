<script>
  import Icon from './Icon.svelte';
  import { t, relTime } from './i18n.svelte.js';

  // Editor de notas (modal). Controlado por App: recibe el objetivo con su body ya
  // cargado; emite onSave(texto) y onClose(). Una nota por artículo (lib+path).
  let { note, onSave, onClose } = $props();

  let text = $state('');
  let saving = $state(false);
  let ta;

  $effect(() => { text = note?.body || ''; });

  $effect(() => { if (ta) { ta.focus(); const n = ta.value.length; ta.setSelectionRange(n, n); } });

  async function save() {
    saving = true;
    await onSave?.(text);
    saving = false;
  }
  function onKey(e) {
    if (e.key === 'Escape') { e.preventDefault(); onClose?.(); }
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') { e.preventDefault(); save(); }
  }
</script>

<div class="scrim" onclick={() => onClose?.()} role="presentation">
  <div class="card" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-label={t('note.aria')}>
    <header class="nhead">
      <span class="ntile"><Icon name="note" size={17} /></span>
      <div class="ntitle">
        <b>{note?.title || note?.path || t('note.title')}</b>
        <small>{note?.book || note?.lib}{note?.updated ? ' · ' + t('note.edited', { when: relTime(note.updated) }) : ''}</small>
      </div>
      <button class="x" title={t('note.close')} onclick={() => onClose?.()}><Icon name="close" size={17} /></button>
    </header>

    <textarea bind:this={ta} bind:value={text} onkeydown={onKey}
      placeholder={t('note.placeholder')}></textarea>

    <footer class="nfoot">
      <span class="tip">{t('note.tip')}</span>
      <div class="btns">
        <button class="btn ghost" onclick={() => onClose?.()}>{t('common.cancel')}</button>
        <button class="btn primary" disabled={saving} onclick={save}>
          <Icon name="check" size={15} /> {saving ? t('common.saving') : t('common.save')}
        </button>
      </div>
    </footer>
  </div>
</div>

<style>
  .scrim{position:fixed;inset:0;z-index:100;background:color-mix(in srgb,#000 55%,transparent);backdrop-filter:blur(2px);display:grid;place-items:center;padding:24px;animation:fade .12s ease}
  @keyframes fade{from{opacity:0}to{opacity:1}}
  .card{width:100%;max-width:620px;background:var(--panel);border:1px solid var(--border);border-radius:16px;box-shadow:0 24px 70px rgba(0,0,0,.5);display:flex;flex-direction:column;overflow:hidden;animation:pop .14s ease}
  @keyframes pop{from{transform:translateY(8px) scale(.98);opacity:0}to{transform:none;opacity:1}}
  .nhead{display:flex;align-items:center;gap:12px;padding:16px 16px 14px;border-bottom:1px solid var(--border)}
  .ntile{width:34px;height:34px;border-radius:9px;flex:none;display:grid;place-items:center;background:color-mix(in srgb,var(--accent) 16%,transparent);color:var(--accent-2)}
  .ntitle{flex:1;min-width:0;display:flex;flex-direction:column;gap:2px}
  .ntitle b{font-size:15px;color:var(--ink);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .ntitle small{font-size:12px;color:var(--muted)}
  .x{width:30px;height:30px;border-radius:8px;display:grid;place-items:center;color:var(--muted);flex:none}
  .x:hover{background:var(--raise);color:var(--ink)}
  textarea{margin:16px;height:280px;resize:vertical;background:var(--ground);border:1px solid var(--border);border-radius:11px;padding:14px 16px;color:var(--ink);font-size:14.5px;line-height:1.6;outline:none;font-family:inherit;transition:border-color .12s}
  textarea:focus{border-color:color-mix(in srgb,var(--accent) 55%,var(--border))}
  textarea::placeholder{color:var(--muted)}
  .nfoot{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:0 16px 16px}
  .tip{font-size:12px;color:var(--faint)}
  .btns{display:flex;gap:8px}
  .btn{display:flex;align-items:center;gap:7px;height:36px;padding:0 16px;border-radius:9px;font-size:13.5px;font-weight:550;transition:background .12s,color .12s,opacity .12s}
  .btn.ghost{color:var(--ink-dim)}
  .btn.ghost:hover{background:var(--raise);color:var(--ink)}
  .btn.primary{background:linear-gradient(140deg,var(--accent),var(--accent-2));color:#fff}
  .btn.primary:hover{filter:brightness(1.08)}
  .btn.primary:disabled{opacity:.6}
</style>
