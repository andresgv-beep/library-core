import { resolveServerPayload } from './connection.js';

export const JSONH = { 'Content-Type': 'application/json' };

export async function jsonOrError(response, fallback = `error ${response.status}`) {
  if (response.ok) return resolveServerPayload(await response.json());
  const err = await response.json().catch(() => ({}));
  throw new Error(err.error || fallback);
}
