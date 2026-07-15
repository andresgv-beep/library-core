const STORE_KEY = 'nimos-profile';

export const PROFILE_COLORS = [
  { id: 'violet', a: '#6a58e0', b: '#9a8cff' },
  { id: 'blue', a: '#2f7de1', b: '#43b4f2' },
  { id: 'green', a: '#258a68', b: '#6dbf7a' },
  { id: 'rose', a: '#cf4e7f', b: '#f08a62' },
  { id: 'amber', a: '#b7791f', b: '#e7b84b' },
];

function loadProfile() {
  try {
    const saved = JSON.parse(localStorage.getItem(STORE_KEY) || '{}');
    return {
      name: typeof saved.name === 'string' && saved.name.trim() ? saved.name : 'Usuario',
      color: PROFILE_COLORS.some((c) => c.id === saved.color) ? saved.color : 'violet',
    };
  } catch (e) {
    return { name: 'Usuario', color: 'violet' };
  }
}

function saveProfile() {
  try {
    localStorage.setItem(STORE_KEY, JSON.stringify({ name: profile.name, color: profile.color }));
  } catch (e) {}
}

export const profile = $state(loadProfile());

export function setProfileName(name) {
  const clean = (name || '').replace(/\s+/g, ' ').trimStart().slice(0, 40);
  profile.name = clean;
  saveProfile();
}

export function setProfileColor(color) {
  if (!PROFILE_COLORS.some((c) => c.id === color)) return;
  profile.color = color;
  saveProfile();
}

export function profileInitials(name) {
  const parts = (name || 'Usuario').trim().split(/\s+/).filter(Boolean);
  const letters = parts.length > 1 ? parts[0][0] + parts[1][0] : (parts[0] || 'U').slice(0, 2);
  return letters.toUpperCase();
}

export function profileGradient(color = profile.color) {
  const c = PROFILE_COLORS.find((x) => x.id === color) || PROFILE_COLORS[0];
  return `linear-gradient(140deg, ${c.a}, ${c.b})`;
}
