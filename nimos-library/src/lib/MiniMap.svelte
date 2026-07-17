<script>
  import { onMount } from 'svelte';
  import { serverFetch, serverUrl } from './connection.js';
  import { loadMapLibre } from './maplibreLoader.js';

  let { result, selectedPoi = null } = $props();
  let container;
  let map;
  let maplibregl;
  let marker;
  let ready = $state(false);
  let failed = $state(false);

  const emptyCollection = () => ({ type: 'FeatureCollection', features: [] });
  function pointFeature(p, properties = {}) {
    return { type: 'Feature', properties, geometry: { type: 'Point', coordinates: [p.lon, p.lat] } };
  }
  function radiusPolygon(lat, lon, meters) {
    if (!meters) return emptyCollection();
    const coordinates = [];
    const earth = 6371000;
    const latRad = lat * Math.PI / 180;
    for (let i = 0; i <= 64; i++) {
      const angle = i / 64 * Math.PI * 2;
      const dLat = Math.sin(angle) * meters / earth * 180 / Math.PI;
      const dLon = Math.cos(angle) * meters / (earth * Math.cos(latRad)) * 180 / Math.PI;
      coordinates.push([lon + dLon, lat + dLat]);
    }
    return { type: 'FeatureCollection', features: [{ type: 'Feature', properties: {}, geometry: { type: 'Polygon', coordinates: [coordinates] } }] };
  }
  function setSource(id, data) {
    const source = map?.getSource(id);
    if (source) source.setData(data);
  }
  function mapAssetUrl(path) {
    const resolved = serverUrl(path);
    try {
      return new URL(resolved, window.location.href).href
        .replaceAll('%7B', '{').replaceAll('%7D', '}');
    }
    catch (e) { return resolved; }
  }
  function updateMap() {
    if (!ready || !map || !result?.location) return;
    const location = result.location;
    const pois = result.pois || [];
    setSource('search-radius', radiusPolygon(location.lat, location.lon, result.radius || 0));
    setSource('search-pois', { type: 'FeatureCollection', features: pois.map((p) => pointFeature(p, { selected: selectedPoi?.name === p.name })) });
    marker?.setLngLat([location.lon, location.lat]);
    const zoom = result.radius >= 5000 ? 11.8 : result.radius >= 2500 ? 12.8 : result.radius >= 1000 ? 13.8 : 15;
    const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
    map.easeTo({ center: [location.lon, location.lat], zoom: Math.min(zoom, (result.map?.maxZoom || 14) + 1), duration: reduced ? 0 : 260 });
  }

  $effect(() => {
    result;
    selectedPoi;
    if (ready) updateMap();
  });

  onMount(() => {
    let cancelled = false;
    (async () => {
      try {
        maplibregl = await loadMapLibre();
        const layers = await (await serverFetch(result.map.style)).json();
        if (cancelled) return;
        const style = {
          version: 8,
          glyphs: mapAssetUrl('/maps/fonts/{fontstack}/{range}.pbf'),
          sprite: mapAssetUrl('/maps/sprites/light'),
          sources: {
            protomaps: {
              type: 'vector', tiles: [mapAssetUrl(`${result.map.tiles}?v=10`)], minzoom: 0,
              maxzoom: Math.min(14, Number(result.map.maxZoom) || 13),
              attribution: '© OpenStreetMap · Protomaps',
            },
          },
          layers,
        };
        map = new maplibregl.Map({
          container, style, center: [result.location.lon, result.location.lat], zoom: 13,
          interactive: false, attributionControl: false, fadeDuration: 0,
        });
        map.addControl(new maplibregl.AttributionControl({ compact: true }), 'bottom-right');
        const markerEl = document.createElement('div');
        markerEl.className = 'nimos-location-marker';
        markerEl.innerHTML = '<span></span>';
        marker = new maplibregl.Marker({ element: markerEl, anchor: 'bottom' }).setLngLat([result.location.lon, result.location.lat]).addTo(map);
        map.on('load', () => {
          if (cancelled) return;
          map.addSource('search-radius', { type: 'geojson', data: emptyCollection() });
          map.addLayer({ id: 'search-radius-fill', type: 'fill', source: 'search-radius', paint: { 'fill-color': '#7c6cf0', 'fill-opacity': 0.1 } });
          map.addLayer({ id: 'search-radius-line', type: 'line', source: 'search-radius', paint: { 'line-color': '#7c6cf0', 'line-opacity': 0.48, 'line-width': 1.5 } });
          map.addSource('search-pois', { type: 'geojson', data: emptyCollection() });
          map.addLayer({ id: 'search-pois', type: 'circle', source: 'search-pois', paint: {
            'circle-radius': ['case', ['boolean', ['get', 'selected'], false], 7, 4.5],
            'circle-color': ['case', ['boolean', ['get', 'selected'], false], '#7c6cf0', '#23a986'],
            'circle-stroke-color': '#ffffff', 'circle-stroke-width': 1.5,
          } });
          ready = true;
          updateMap();
          requestAnimationFrame(() => map?.resize());
        });
      } catch (e) {
        failed = true;
      }
    })();
    return () => {
      cancelled = true;
      marker?.remove();
      map?.remove();
    };
  });
</script>

<div class="mini-map" class:failed bind:this={container} aria-label={`Mapa de ${result?.location?.name || 'la ubicación buscada'}`}></div>

<style>
  .mini-map{position:absolute;inset:0;background:var(--raise)}
  .mini-map.failed{display:none}
  .mini-map :global(.maplibregl-canvas){filter:saturate(.76) contrast(.92)}
  .mini-map :global(.maplibregl-ctrl-attrib){font-size:9px;background:color-mix(in srgb,#fff 72%,transparent)}
  .mini-map :global(.nimos-location-marker){width:34px;height:42px;display:grid;place-items:start center;filter:drop-shadow(0 4px 8px rgba(30,25,90,.28))}
  .mini-map :global(.nimos-location-marker::before){content:"";width:27px;height:27px;border-radius:50% 50% 50% 0;background:var(--accent);transform:rotate(-45deg);border:2px solid #fff}
  .mini-map :global(.nimos-location-marker span){position:absolute;top:8px;width:7px;height:7px;border-radius:50%;background:#fff}
  :global(:root:not([data-theme="light"])) .mini-map::after{content:"";position:absolute;inset:0;background:color-mix(in srgb,var(--ground) 17%,transparent);pointer-events:none}
</style>
