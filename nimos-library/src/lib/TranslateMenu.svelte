<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import { t, LANGS } from './i18n.svelte.js';
  import { translateLanguages } from './libraryApi.js';
  import { tstate, setAuto, setTarget, targetLang, requestTranslate, requestOriginal } from './translate.svelte.js';

  let open = $state(false);
  let available = $state(false);
  let btnEl;
  // El dropdown se posiciona con position:fixed (calculado desde el botón) para
  // escapar el overflow:hidden de la barra de dirección, donde vive el icono.
  let pos = $state({ top: 0, right: 0 });

  // Detección tipo Maps: si no hay motor, el dropdown lo dice y no ofrece idiomas.
  onMount(async () => {
    const d = await translateLanguages();
    available = !!d.available;
  });

  function toggle() {
    if (!open && btnEl) {
      const r = btnEl.getBoundingClientRect();
      pos = { top: r.bottom + 8, right: window.innerWidth - r.right };
    }
    open = !open;
  }

  function pickLang(code) {
    setTarget(code);
    requestTranslate();
    open = false;
  }
  function toggleAuto() {
    const now = !tstate.auto;
    setAuto(now);
    if (now) requestTranslate();
  }
  function seeOriginal() {
    requestOriginal();
    open = false;
  }
</script>

<div class="twrap" onfocusout={(e) => { if (!e.currentTarget.contains(e.relatedTarget)) open = false; }}>
  <button bind:this={btnEl} class="tbtn" class:on={open || tstate.auto} onclick={toggle} title={t('nav.translate')}>
    <Icon name="translate" size={15} />
  </button>

  {#if open}
    <div class="tdrop" style="top:{pos.top}px; right:{pos.right}px">
      {#if !available}
        <div class="unavail">{t('translate.unavailable')}</div>
      {:else}
        <button class="auto" class:on={tstate.auto} onclick={toggleAuto}>
          <span class="atext">
            <b>{t('translate.auto')}</b>
            <small>{t('translate.autoDesc')}</small>
          </span>
          <span class="switch" class:on={tstate.auto}><span class="knob"></span></span>
        </button>

        <div class="sep"></div>
        <div class="tlabel">{t('translate.to')}</div>
        {#each LANGS as l}
          <button class="lrow" class:on={!tstate.auto && targetLang() === l.code} onclick={() => pickLang(l.code)}>
            <span class="flag">{l.flag}</span>
            <span class="lname">{l.label}</span>
            {#if !tstate.auto && targetLang() === l.code}<Icon name="check" size={15} />{/if}
          </button>
        {/each}

        <div class="sep"></div>
        <button class="lrow" onclick={seeOriginal}>
          <span class="oico"><Icon name="reload" size={14} /></span>
          <span class="lname">{t('translate.showOriginal')}</span>
        </button>
      {/if}
    </div>
  {/if}
</div>

<style>
  .twrap{position:relative;flex:none;display:grid}
  .tbtn{width:26px;height:26px;border-radius:7px;display:grid;place-items:center;color:var(--muted);flex:none;transition:background .12s,color .12s}
  .tbtn:hover{background:var(--raise);color:var(--ink)}
  .tbtn.on{color:var(--accent-2)}

  /* position:fixed → no lo recorta el overflow:hidden de la barra de dirección. */
  .tdrop{position:fixed;width:266px;background:var(--card);border:1px solid var(--border);border-radius:12px;box-shadow:var(--shadow);padding:6px;z-index:60;display:flex;flex-direction:column;gap:1px}

  .auto{display:flex;align-items:center;gap:10px;padding:9px 10px;border-radius:9px;text-align:left;transition:background .12s}
  .auto:hover{background:var(--raise)}
  .atext{flex:1;min-width:0;display:flex;flex-direction:column;gap:2px}
  .atext b{font-size:13.5px;color:var(--ink);font-weight:580}
  .atext small{font-size:11.5px;color:var(--muted);line-height:1.4}
  .switch{flex:none;width:34px;height:20px;border-radius:11px;background:var(--border);position:relative;transition:background .15s}
  .switch.on{background:var(--accent)}
  .knob{position:absolute;top:2px;left:2px;width:16px;height:16px;border-radius:50%;background:#fff;transition:transform .15s}
  .switch.on .knob{transform:translateX(14px)}

  .sep{height:1px;background:var(--border);margin:5px 4px}
  .tlabel{font-size:11px;font-weight:650;letter-spacing:.5px;text-transform:uppercase;color:var(--faint);padding:4px 10px 6px}
  .lrow{display:flex;align-items:center;gap:11px;padding:9px 10px;border-radius:9px;color:var(--ink-dim);text-align:left;transition:background .12s,color .12s}
  .lrow:hover{background:var(--raise);color:var(--ink)}
  .lrow.on{background:color-mix(in srgb,var(--accent) 12%,transparent);color:var(--ink)}
  .lrow .flag{font-size:18px;line-height:1}
  .lrow .oico{display:grid;place-items:center;color:var(--muted)}
  .lrow .lname{flex:1;font-size:13.5px;font-weight:520}
  .lrow.on :global(.ic){color:var(--accent-2)}
  .unavail{padding:16px 12px;text-align:center;color:var(--muted);font-size:12.5px;line-height:1.5}
</style>
