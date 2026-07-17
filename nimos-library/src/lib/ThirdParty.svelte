<script>
  import Icon from './Icon.svelte';
  import { t } from './i18n.svelte.js';
  import { serverFetch } from './connection.js';

  let noticesOpen = $state(false);
  let noticesLoading = $state(false);
  let noticesText = $state('');
  let noticesError = $state('');

  async function openNotices() {
    noticesOpen = true;
    if (noticesText || noticesLoading) return;
    noticesLoading = true;
    noticesError = '';
    try {
      const response = await serverFetch('/THIRD-PARTY-NOTICES.txt', { cache: 'no-store' });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      noticesText = await response.text();
    } catch (error) {
      noticesError = error?.message || String(error);
    } finally {
      noticesLoading = false;
    }
  }

  const GROUPS = [
    {
      labelKey: 'thirdParty.core',
      items: [
        { name: 'Svelte', license: 'MIT', descKey: 'thirdParty.svelte', url: 'https://github.com/sveltejs/svelte' },
        { name: 'Wails', license: 'MIT', descKey: 'thirdParty.wails', url: 'https://github.com/wailsapp/wails' },
        { name: 'Go', license: 'BSD-3-Clause', descKey: 'thirdParty.go', url: 'https://go.dev/' },
        { name: 'SQLite / modernc.org/sqlite', license: 'BSD-3-Clause', descKey: 'thirdParty.sqlite', url: 'https://gitlab.com/cznic/sqlite' },
      ],
    },
    {
      labelKey: 'thirdParty.content',
      items: [
        { name: 'Bleve', license: 'Apache-2.0', descKey: 'thirdParty.bleve', url: 'https://github.com/blevesearch/bleve' },
        { name: 'PDF.js', license: 'Apache-2.0', descKey: 'thirdParty.pdfjs', url: 'https://github.com/mozilla/pdf.js' },
        { name: 'MapLibre GL JS', license: 'BSD-3-Clause', descKey: 'thirdParty.maplibre', url: 'https://github.com/maplibre/maplibre-gl-js' },
        { name: 'PMTiles', license: 'BSD-3-Clause', descKey: 'thirdParty.pmtiles', url: 'https://github.com/protomaps/PMTiles' },
        { name: 'OpenStreetMap', license: 'ODbL 1.0', descKey: 'thirdParty.osm', url: 'https://www.openstreetmap.org/copyright' },
      ],
    },
    {
      labelKey: 'thirdParty.translation',
      items: [
        { name: 'translateLocally', license: 'MIT', descKey: 'thirdParty.translateLocally', url: 'https://github.com/XapaJIaMnu/translateLocally' },
        { name: 'Bergamot Translator', license: 'MPL-2.0', descKey: 'thirdParty.bergamot', url: 'https://github.com/browsermt/bergamot-translator' },
        { name: 'Marian NMT', license: 'MIT', descKey: 'thirdParty.marian', url: 'https://github.com/marian-nmt/marian-dev' },
      ],
    },
  ];
</script>

<div class="view scroll">
  <header class="vhead">
    <span class="vtile"><Icon name="info" size={21} /></span>
    <div class="vtitle">
      <h1>{t('thirdParty.title')}</h1>
      <span class="vsub">{t('thirdParty.subtitle')}</span>
    </div>
  </header>

  <main class="body">
    <p class="intro">{t('thirdParty.intro')}</p>

    {#each GROUPS as group}
      <section class="group">
        <h2>{t(group.labelKey)}</h2>
        <div class="card">
          {#each group.items as item}
            <article class="software">
              <div class="mark">{item.name.slice(0, 1)}</div>
              <div class="details">
                <div class="name"><b>{item.name}</b><span>{item.license}</span></div>
                <p>{t(item.descKey)}</p>
              </div>
              <a href={item.url} target="_blank" rel="noreferrer" title={t('thirdParty.openSource')} aria-label={`${item.name}: ${t('thirdParty.openSource')}`}>
                <Icon name="forward" size={16} />
              </a>
            </article>
          {/each}
        </div>
      </section>
    {/each}

    <aside class="notice"><Icon name="info" size={17} /><span>{t('thirdParty.modelNotice')}</span></aside>

    <button class="all-notices" type="button" onclick={openNotices}>
      <span><Icon name="note" size={18} /></span>
      <span><b>{t('thirdParty.allNotices')}</b><small>{t('thirdParty.allNoticesDesc')}</small></span>
      <Icon name="forward" size={16} />
    </button>
  </main>
</div>

{#if noticesOpen}
  <div class="notices-overlay" role="presentation">
    <dialog class="notices-dialog" open aria-labelledby="third-party-notices-title">
      <header>
        <div>
          <h2 id="third-party-notices-title">{t('thirdParty.allNotices')}</h2>
          <span>{t('thirdParty.allNoticesDesc')}</span>
        </div>
        <button type="button" class="close-notices" onclick={() => noticesOpen = false} aria-label={t('common.close')} title={t('common.close')}><Icon name="close" size={18} /></button>
      </header>
      <div class="notices-content">
        {#if noticesLoading}
          <p class="notices-state">{t('common.loading')}</p>
        {:else if noticesError}
          <p class="notices-state error">{t('thirdParty.noticesError')} ({noticesError})</p>
        {:else}
          <pre>{noticesText}</pre>
        {/if}
      </div>
    </dialog>
  </div>
{/if}

<style>
  .view{background:var(--ground);min-height:100%;overflow-y:auto}
  .vhead{max-width:820px;margin:0 auto;padding:34px 40px 18px;display:flex;align-items:center;gap:14px}
  .vtile{width:44px;height:44px;border-radius:12px;flex:none;display:grid;place-items:center;background:color-mix(in srgb,var(--accent) 16%,transparent);color:var(--accent-2)}
  .vtitle{display:flex;flex-direction:column;gap:2px}.vtitle h1{font-size:24px;font-weight:680;letter-spacing:-.4px}.vsub{font-size:13px;color:var(--muted)}
  .body{max-width:820px;margin:0 auto;padding:6px 40px 90px;display:flex;flex-direction:column;gap:22px}
  .intro{max-width:680px;color:var(--ink-dim);font-size:13.5px;line-height:1.65}
  .group{display:flex;flex-direction:column;gap:9px}.group h2{font-size:11px;font-weight:700;letter-spacing:.7px;text-transform:uppercase;color:var(--faint);padding-left:2px}
  .card{background:var(--card);border:1px solid var(--border);border-radius:14px;overflow:hidden}
  .software{display:grid;grid-template-columns:38px minmax(0,1fr) 34px;align-items:center;gap:13px;padding:14px 16px}
  .software+.software{border-top:1px solid var(--border)}
  .mark{width:36px;height:36px;border-radius:10px;display:grid;place-items:center;background:color-mix(in srgb,var(--accent) 13%,var(--ground));color:var(--accent-2);font-size:14px;font-weight:750}
  .details{min-width:0}.name{display:flex;align-items:center;gap:8px;flex-wrap:wrap}.name b{font-size:14px;font-weight:620;color:var(--ink)}.name span{font-size:10px;font-weight:650;letter-spacing:.25px;padding:3px 7px;border-radius:20px;color:var(--muted);background:var(--ground);border:1px solid var(--border)}
  .details p{margin-top:3px;color:var(--muted);font-size:12.5px;line-height:1.45}
  .software a{width:32px;height:32px;border-radius:8px;display:grid;place-items:center;color:var(--muted);transition:background .12s,color .12s}.software a:hover{background:var(--raise);color:var(--accent-2)}
  .notice{display:flex;align-items:flex-start;gap:10px;padding:14px 16px;border-radius:12px;background:color-mix(in srgb,var(--accent) 8%,var(--card));border:1px solid color-mix(in srgb,var(--accent) 20%,var(--border));color:var(--muted);font-size:12.5px;line-height:1.5}.notice :global(.ic){flex:none;margin-top:1px;color:var(--accent-2)}
  .all-notices{width:100%;display:grid;grid-template-columns:36px minmax(0,1fr) 24px;align-items:center;gap:12px;padding:14px 16px;border:1px solid var(--border);border-radius:12px;background:var(--card);color:var(--ink);text-align:left;transition:border-color .12s,background .12s}.all-notices:hover{border-color:color-mix(in srgb,var(--accent) 35%,var(--border));background:var(--raise)}.all-notices>span:first-child{width:36px;height:36px;border-radius:10px;display:grid;place-items:center;background:color-mix(in srgb,var(--accent) 13%,var(--ground));color:var(--accent-2)}.all-notices>span:nth-child(2){display:flex;flex-direction:column;gap:2px}.all-notices b{font-size:13.5px;font-weight:650}.all-notices small{font-size:12px;color:var(--muted);line-height:1.4}.all-notices>:global(.ic){color:var(--muted)}
  .notices-overlay{position:fixed;inset:0;z-index:1000;display:grid;place-items:center;padding:28px;background:color-mix(in srgb,#000 62%,transparent);backdrop-filter:blur(5px)}
  .notices-dialog{width:min(960px,100%);height:min(760px,calc(100vh - 56px));display:flex;flex-direction:column;overflow:hidden;border:1px solid var(--border);border-radius:18px;background:var(--ground);box-shadow:0 24px 80px rgba(0,0,0,.45)}
  .notices-dialog>header{display:flex;align-items:center;justify-content:space-between;gap:20px;padding:18px 20px;border-bottom:1px solid var(--border);background:var(--card)}.notices-dialog>header div{display:flex;flex-direction:column;gap:3px}.notices-dialog h2{font-size:16px;font-weight:680;color:var(--ink)}.notices-dialog header span{font-size:12px;color:var(--muted)}
  .close-notices{width:34px;height:34px;flex:none;display:grid;place-items:center;border:1px solid var(--border);border-radius:9px;background:var(--ground);color:var(--muted)}.close-notices:hover{background:var(--raise);color:var(--ink)}
  .notices-content{min-height:0;flex:1;overflow:auto;padding:20px;background:var(--ground)}.notices-content pre{margin:0;white-space:pre-wrap;overflow-wrap:anywhere;color:var(--ink-dim);font:11.5px/1.6 ui-monospace,SFMono-Regular,Consolas,monospace}.notices-state{display:grid;place-items:center;height:100%;color:var(--muted);font-size:13px}.notices-state.error{color:#da6b74}
  @media(max-width:620px){.vhead{padding:26px 22px 16px}.body{padding:4px 22px 70px}.software{padding:13px 12px;grid-template-columns:36px minmax(0,1fr) 30px}}
</style>
