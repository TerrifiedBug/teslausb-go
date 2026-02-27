import { useEffect, useState } from 'react';
import { api } from '../lib/api';
import type { Config as ConfigType } from '../lib/api';

export function Config() {
  const [config, setConfig] = useState<ConfigType | null>(null);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState('');

  const [bleStatus, setBleStatus] = useState<{keys_exist: boolean; paired: boolean} | null>(null);

  useEffect(() => {
    api.getConfig().then(setConfig).catch(console.error);
    api.getBLEStatus().then(setBleStatus).catch(console.error);
  }, []);

  const save = async () => {
    if (!config) return;
    setSaving(true);
    setMessage('');
    try {
      await api.saveConfig(config);
      setMessage('Saved');
      setTimeout(() => setMessage(''), 3000);
    } catch (e: any) {
      setMessage(`Error: ${e.message}`);
    }
    setSaving(false);
  };

  const [testing, setTesting] = useState(false);

  const testNFS = async () => {
    if (!config?.nfs.server || !config?.nfs.share) {
      setMessage('Enter server and share first');
      return;
    }
    setTesting(true);
    setMessage('');
    try {
      const result = await api.testNFS(config.nfs.server, config.nfs.share);
      if (result.ok) {
        setMessage(result.message || 'NFS connection successful');
      } else {
        setMessage(`Error: ${result.error}`);
      }
    } catch (e: any) {
      setMessage(`Error: ${e.message}`);
    }
    setTesting(false);
  };

  const pairBLE = async () => {
    if (!config?.keep_awake.vin) return;
    try {
      await api.pairBLE(config.keep_awake.vin);
      setMessage('Pairing request sent — tap NFC card on center console');
    } catch (e: any) {
      setMessage(`Pair error: ${e.message}`);
    }
  };

  if (!config) return <div className="text-gray-500">Loading...</div>;

  const update = (section: string, key: string, value: string | number) => {
    setConfig(prev => prev ? { ...prev, [section]: { ...(prev as any)[section], [key]: value } } : prev);
  };

  return (
    <div className="space-y-6">
      <section className="bg-gray-900 rounded-lg p-4 border border-gray-800 space-y-3">
        <h2 className="text-sm font-medium text-gray-300">NFS Server</h2>
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="text-xs text-gray-500">Server</label>
            <input
              value={config.nfs.server}
              onChange={e => update('nfs', 'server', e.target.value)}
              className="w-full mt-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
              placeholder="192.168.1.100"
            />
          </div>
          <div>
            <label className="text-xs text-gray-500">Share</label>
            <input
              value={config.nfs.share}
              onChange={e => update('nfs', 'share', e.target.value)}
              className="w-full mt-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
              placeholder="/volume1/TeslaCam"
            />
          </div>
        </div>
        <button
          onClick={testNFS}
          disabled={testing}
          className="px-3 py-1.5 bg-gray-800 hover:bg-gray-700 disabled:opacity-50 border border-gray-700 rounded text-sm text-gray-300"
        >
          {testing ? 'Testing...' : 'Test Connection'}
        </button>
      </section>

      <section className="bg-gray-900 rounded-lg p-4 border border-gray-800 space-y-3">
        <h2 className="text-sm font-medium text-gray-300">Keep Awake</h2>
        <div className="flex gap-2">
          {['ble', 'webhook'].map(method => (
            <button
              key={method}
              onClick={() => update('keep_awake', 'method', method)}
              className={`px-3 py-1.5 rounded text-sm ${
                config.keep_awake.method === method ? 'bg-blue-600' : 'bg-gray-800 text-gray-400'
              }`}
            >
              {method.toUpperCase()}
            </button>
          ))}
        </div>
        {config.keep_awake.method === 'ble' ? (
          <div>
            <label className="text-xs text-gray-500">VIN</label>
            <div className="flex gap-2 mt-1">
              <input
                value={config.keep_awake.vin}
                onChange={e => update('keep_awake', 'vin', e.target.value)}
                className="flex-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                placeholder="5YJ3E1EA1NF000000"
              />
              <button onClick={pairBLE} className="px-3 py-2 bg-blue-600 hover:bg-blue-700 rounded text-sm">
                Pair
              </button>
            </div>
            {bleStatus && (
              <div className="mt-1 text-xs">
                {bleStatus.keys_exist ? (
                  <span className={bleStatus.paired ? 'text-green-400' : 'text-yellow-400'}>
                    {bleStatus.paired ? 'Paired' : 'Keys generated — not yet paired'}
                  </span>
                ) : (
                  <span className="text-gray-500">No BLE keys — click Pair to set up</span>
                )}
              </div>
            )}
          </div>
        ) : (
          <div>
            <label className="text-xs text-gray-500">Webhook URL</label>
            <input
              value={config.keep_awake.webhook_url}
              onChange={e => update('keep_awake', 'webhook_url', e.target.value)}
              className="w-full mt-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
              placeholder="http://homeassistant.local:8123/api/webhook/..."
            />
          </div>
        )}
      </section>

      <section className="bg-gray-900 rounded-lg p-4 border border-gray-800 space-y-3">
        <h2 className="text-sm font-medium text-gray-300">Notifications</h2>
        <div>
          <label className="text-xs text-gray-500">Webhook URL</label>
          <input
            value={config.notifications.webhook_url}
            onChange={e => update('notifications', 'webhook_url', e.target.value)}
            className="w-full mt-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
            placeholder="http://homeassistant.local:8123/api/webhook/teslausb-notify"
          />
        </div>
      </section>

      <section className="bg-gray-900 rounded-lg p-4 border border-gray-800 space-y-3">
        <h2 className="text-sm font-medium text-gray-300">Temperature Thresholds</h2>
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="text-xs text-gray-500">Warning (°C)</label>
            <input
              type="number"
              value={config.temperature.warning_celsius}
              onChange={e => update('temperature', 'warning_celsius', Number(e.target.value))}
              className="w-full mt-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
            />
          </div>
          <div>
            <label className="text-xs text-gray-500">Caution (°C)</label>
            <input
              type="number"
              value={config.temperature.caution_celsius}
              onChange={e => update('temperature', 'caution_celsius', Number(e.target.value))}
              className="w-full mt-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
            />
          </div>
        </div>
      </section>

      <div className="flex items-center gap-3">
        <button
          onClick={save}
          disabled={saving}
          className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 rounded-md text-sm"
        >
          {saving ? 'Saving...' : 'Save'}
        </button>
        {message && <span className={`text-sm ${message.startsWith('Error') ? 'text-red-400' : 'text-green-400'}`}>{message}</span>}
      </div>
    </div>
  );
}
