const API_BASE = '';

async function fetchJSON<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${url}`, options);
  if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`);
  return res.json();
}

export interface Status {
  state: string;
  version: string;
  temperature: number;
  disk_total?: number;
  disk_free?: number;
  disk_used?: number;
  last_archive: string;
  last_error: string;
  archive_clips: number;
  archive_bytes: number;
}

export interface FileEntry {
  name: string;
  is_dir: boolean;
  size: number;
  path: string;
}

export interface Config {
  nfs: { server: string; share: string };
  keep_awake: { method: string; vin: string; webhook_url: string };
  notifications: { webhook_url: string };
  temperature: { warning_celsius: number; caution_celsius: number };
}

export interface BLEStatus {
  keys_exist: boolean;
  paired: boolean;
}

export const api = {
  getStatus: () => fetchJSON<Status>('/api/status'),
  getFiles: (path = 'TeslaCam') => fetchJSON<FileEntry[]>(`/api/files?path=${encodeURIComponent(path)}`),
  downloadURL: (path: string) => `/api/files/download?path=${encodeURIComponent(path)}`,
  deleteFile: (path: string) => fetchJSON<{status: string}>('/api/files/delete', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path }),
  }),
  getConfig: () => fetchJSON<Config>('/api/config'),
  saveConfig: (config: Config) => fetchJSON<{status: string}>('/api/config', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(config),
  }),
  triggerArchive: () => fetchJSON<{status: string}>('/api/archive/trigger', { method: 'POST' }),
  pairBLE: (vin: string) => fetchJSON<{status: string}>('/api/ble/pair', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ vin }),
  }),
  getBLEStatus: () => fetchJSON<BLEStatus>('/api/ble/status'),
  getLogs: () => fetchJSON<string[]>('/api/logs'),
};
