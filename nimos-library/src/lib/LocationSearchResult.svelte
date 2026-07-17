<script>
  import Icon from './Icon.svelte';
  import MiniMap from './MiniMap.svelte';
  import { t } from './i18n.svelte.js';

  let { locationState, onRadiusChange } = $props();
  let radiusKm = $state(2.5);
  let selectedPoi = $state(null);
  let radiusTimer;
  let lastRequested = 2500;

  let result = $derived(locationState?.result);
  let visiblePois = $derived((result?.pois || []).filter((p) => p.distance <= radiusKm * 1000).slice(0, 6));
  let mapResult = $derived(result ? { ...result, radius: Math.round(radiusKm * 1000) } : null);

  $effect(() => {
    if (locationState?.radius == null) return;
    radiusKm = locationState.radius / 1000;
    lastRequested = locationState.radius;
  });

  function radiusLabel(value) {
    return value === 0 ? '0 km' : `${String(value).replace('.', ',')} km`;
  }
  function distanceLabel(meters) {
    return meters < 1000 ? `${meters} m` : `${(meters / 1000).toFixed(1).replace('.', ',')} km`;
  }
  function categoryLabel(poi) {
    const key = `home.map.category.${poi.categoryCode || 'other'}`;
    const translated = t(key);
    return translated === key ? poi.category : translated;
  }
  function requestRadius(value) {
    const meters = Math.round(Number(value) * 1000);
    if (meters === lastRequested) return;
    lastRequested = meters;
    onRadiusChange?.(meters);
  }
  function onRadiusInput(event) {
    radiusKm = Number(event.currentTarget.value);
    clearTimeout(radiusTimer);
    radiusTimer = setTimeout(() => requestRadius(radiusKm), 180);
  }
  function onRadiusCommit() {
    clearTimeout(radiusTimer);
    requestRadius(radiusKm);
  }
</script>

{#if result?.location && result?.map}
  <section class="geo" aria-labelledby="geo-title">
    <div class="geo-copy">
      <div class="geo-kind"><Icon name="pin" size={15} /> {t('home.map.location')}</div>
      <h2 id="geo-title">{result.location.name}{result.location.houseNumber ? ` ${result.location.houseNumber}` : ''}</h2>
      {#if result.location.context}<p>{result.location.context}</p>{/if}
      {#if result.location.approximate}<span class="approx">{t('home.map.approximate')}</span>{/if}
      <label class="radius-label" for="library-map-radius">
        <span>{t('home.map.nearby')} <output for="library-map-radius">{radiusLabel(radiusKm)}</output></span>
        <input id="library-map-radius" type="range" min="0" max="5" step="0.5" value={radiusKm}
          oninput={onRadiusInput} onchange={onRadiusCommit} />
        <span class="range-ends"><span>0 km</span><span>5 km</span></span>
      </label>
    </div>
    <div class="map-wrap">
      <MiniMap result={mapResult} {selectedPoi} />
      <div class="map-fade"></div>
    </div>
  </section>

  <section class="nearby" aria-labelledby="nearby-title">
    <div class="nearby-head">
      <h3 id="nearby-title">{t('home.map.places')}</h3>
      <span>{#if locationState.status === 'loading'}{t('home.map.updating')}{:else}{t('home.map.placeCount', { n: visiblePois.length, radius: radiusLabel(radiusKm) })}{/if}</span>
    </div>
    {#if radiusKm === 0}
      <p class="nearby-empty">{t('home.map.zeroRadius')}</p>
    {:else if visiblePois.length}
      <div class="poi-grid">
        {#each visiblePois as poi}
          <button class="poi" class:selected={selectedPoi === poi} onclick={() => selectedPoi = poi} aria-pressed={selectedPoi === poi}>
            <span class="poi-icon"><Icon name="pin" size={15} /></span>
            <span class="poi-copy"><b>{poi.name}</b><small>{categoryLabel(poi)} · {distanceLabel(poi.distance)}</small></span>
          </button>
        {/each}
      </div>
    {:else if locationState.status === 'loading'}
      <p class="nearby-empty">{t('home.map.updating')}</p>
    {:else}
      <p class="nearby-empty">{t('home.map.noPlaces')}</p>
    {/if}
    {#if selectedPoi}
      <p class="selected-info"><Icon name="map" size={14} /> {t('home.map.selected', { name: selectedPoi.name, distance: distanceLabel(selectedPoi.distance) })}</p>
    {/if}
  </section>
{/if}

<style>
  .geo{position:relative;width:calc(100% - 80px);max-width:1300px;min-height:310px;margin:14px auto 0;display:grid;grid-template-columns:minmax(260px,38%) 1fr;overflow:hidden;isolation:isolate}
  .geo-copy{position:relative;z-index:3;align-self:center;padding:28px 10px 28px 54px}
  .geo-kind{display:flex;align-items:center;gap:7px;color:var(--accent-2);font-size:11px;font-weight:650;letter-spacing:.8px;text-transform:uppercase;margin-bottom:8px}
  h2{font-size:26px;line-height:1.2;font-weight:650;color:var(--ink);letter-spacing:-.3px}
  .geo-copy p{color:var(--muted);font-size:13.5px;margin-top:6px}
  .approx{display:inline-block;margin-top:8px;padding:4px 8px;border-radius:7px;background:color-mix(in srgb,var(--accent) 12%,transparent);color:var(--accent-2);font-size:10.5px}
  .radius-label{display:block;max-width:260px;margin-top:24px;color:var(--ink-dim);font-size:12.5px}
  .radius-label>span:first-child{display:flex;align-items:baseline;justify-content:space-between;gap:12px}
  .radius-label output{color:var(--ink);font-weight:600}
  .radius-label input{width:100%;accent-color:var(--accent);cursor:pointer;margin-top:9px}
  .range-ends{display:flex;justify-content:space-between;color:var(--faint);font-size:10.5px;margin-top:1px}
  .map-wrap{position:absolute;inset:0 0 0 36%;overflow:hidden;border-radius:16px;
    -webkit-mask-image:linear-gradient(90deg,transparent 0,#000 10px,#000 calc(100% - 10px),transparent 100%),linear-gradient(180deg,transparent 0,#000 10px,#000 calc(100% - 10px),transparent 100%);
    -webkit-mask-composite:source-in;
    mask-image:linear-gradient(90deg,transparent 0,#000 10px,#000 calc(100% - 10px),transparent 100%),linear-gradient(180deg,transparent 0,#000 10px,#000 calc(100% - 10px),transparent 100%);
    mask-composite:intersect}
  .map-fade{display:none}
  .nearby{max-width:1300px;margin:-5px auto 0;padding:0 54px 18px;position:relative;z-index:2}
  .nearby-head{display:flex;align-items:baseline;justify-content:space-between;gap:14px;margin-bottom:8px}
  .nearby-head h3{font-size:14px;font-weight:650;color:var(--ink)}
  .nearby-head span{font-size:11px;color:var(--faint)}
  .poi-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:4px 8px}
  .poi{min-width:0;display:flex;align-items:center;gap:9px;text-align:left;padding:8px;border-radius:9px;transition:background .12s}
  .poi:hover,.poi.selected{background:var(--card)}
  .poi-icon{display:grid;place-items:center;width:31px;height:31px;flex:none;border-radius:10px;background:color-mix(in srgb,var(--accent) 12%,transparent);color:var(--accent-2)}
  .poi-copy{min-width:0;display:flex;flex-direction:column}
  .poi-copy b,.poi-copy small{white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .poi-copy b{font-size:12.5px;font-weight:600;color:var(--ink-dim)}
  .poi-copy small{font-size:10.5px;color:var(--muted)}
  .nearby-empty{color:var(--muted);font-size:12.5px;padding:9px 0}
  .selected-info{display:flex;align-items:center;gap:6px;color:var(--muted);font-size:11px;padding:8px 4px 0}
  @media(max-width:720px){
    .geo{width:100%;min-height:440px;grid-template-columns:1fr}.geo-copy{align-self:start;padding:16px 24px}.map-wrap{inset:145px 0 0}
    .nearby{padding:0 24px 18px}.poi-grid{grid-template-columns:1fr}
  }
</style>
