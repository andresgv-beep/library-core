const encPath = (value) => String(value || '').split('/').map(encodeURIComponent).join('/');
const decPath = (value) => String(value || '').split('/').map(decodeURIComponent).join('/');

export function formatLibraryAddress(tab) {
  if (!tab || tab.kind === 'home') return 'library://home';
  if (tab.kind === 'view') return `library://view/${encodeURIComponent(tab.view || 'home')}`;
  if (tab.kind === 'article') return `library://zim/${encodeURIComponent(tab.lib || '')}/${encPath(tab.path)}`;
  if (tab.kind === 'item') {
    const provider = tab.source?.provider || tab.open?.provider || 'item';
    const sourceId = tab.source?.providerItemId || tab.itemId || '';
    if (provider === 'moments' || provider === 'cabinet') {
      return `library://${provider}/${encodeURIComponent(sourceId)}`;
    }
    return `library://item/${encodeURIComponent(tab.itemId || sourceId)}`;
  }
  return 'library://home';
}

export function parseLibraryAddress(raw) {
  const text = String(raw || '').trim();
  if (!text.toLowerCase().startsWith('library://')) return { kind: 'search', query: text };
  const rest = text.slice('library://'.length);
  const parts = rest.split('/');
  const head = decodeURIComponent(parts.shift() || '').toLowerCase();
  if (!head || head === 'home' || head === 'inicio') return { kind: 'home' };
  if (head === 'view') return { kind: 'view', view: decodeURIComponent(parts.join('/') || 'home') };
  if (head === 'zim') return { kind: 'article', lib: decodeURIComponent(parts.shift() || ''), path: decPath(parts.join('/')) };
  if (head === 'moments' || head === 'cabinet') {
    return { kind: 'provider', provider: head, sourceId: decodeURIComponent(parts.join('/')) };
  }
  if (head === 'item') return { kind: 'item', itemId: decodeURIComponent(parts.join('/')) };
  return { kind: 'invalid' };
}
