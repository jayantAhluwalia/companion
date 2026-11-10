// Server URL and bearer token live in SecureStore so they survive rebuilds
// and never ship inside the bundle. Phase 0 enters them by hand on the spike
// screen; later they can be baked in via EAS environment variables.
import * as SecureStore from 'expo-secure-store';

const KEY_URL = 'companion.apiUrl';
const KEY_TOKEN = 'companion.apiToken';

export type Settings = { apiUrl: string; apiToken: string };

export async function loadSettings(): Promise<Settings> {
  const [apiUrl, apiToken] = await Promise.all([
    SecureStore.getItemAsync(KEY_URL),
    SecureStore.getItemAsync(KEY_TOKEN),
  ]);
  return { apiUrl: apiUrl ?? '', apiToken: apiToken ?? '' };
}

export async function saveSettings(s: Settings): Promise<void> {
  await Promise.all([
    SecureStore.setItemAsync(KEY_URL, s.apiUrl.trim().replace(/\/+$/, '')),
    SecureStore.setItemAsync(KEY_TOKEN, s.apiToken.trim()),
  ]);
}
