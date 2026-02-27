import { useEffect, useState } from 'react';
import { api } from '../lib/api';
import type { Status, UpdateInfo } from '../lib/api';
import { formatBytes } from '../lib/format';
import { WSClient } from '../lib/ws';

const stateColors: Record<string, string> = {
  away: 'bg-blue-500',
  arriving: 'bg-yellow-500',
  archiving: 'bg-orange-500',
  idle: 'bg-green-500',
  booting: 'bg-gray-500',
  error: 'bg-red-500',
};

function signalPercent(dbm: number): number {
  if (dbm >= -50) return 100;
  if (dbm <= -100) return 0;
  return Math.round(2 * (dbm + 100));
}

export function Dashboard() {
  const [status, setStatus] = useState<Status | null>(null);
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null);
  const [updating, setUpdating] = useState(false);
  const [checkingUpdate, setCheckingUpdate] = useState(false);

  useEffect(() => {
    api.getStatus().then(setStatus).catch(console.error);
    const interval = setInterval(() => {
      api.getStatus().then(setStatus).catch(console.error);
    }, 10000);

    const ws = new WSClient();
    ws.connect();
    ws.onMessage((data) => {
      if (data.type === 'state') {
        setStatus(prev => prev ? { ...prev, state: data.state } : prev);
      }
    });

    return () => { clearInterval(interval); ws.disconnect(); };
  }, []);

  const checkForUpdate = async () => {
    setCheckingUpdate(true);
    try {
      const info = await api.checkUpdate();
      setUpdateInfo(info);
    } catch (e: any) {
      setUpdateInfo({ available: false, error: e.message });
    }
    setCheckingUpdate(false);
  };

  const applyUpdate = async () => {
    setUpdating(true);
    try {
      await api.applyUpdate();
      // Service will restart — auto-reload after delay
      setTimeout(() => window.location.reload(), 15000);
    } catch {
      setUpdating(false);
    }
  };

  if (!status) return <div className="text-gray-500">Loading...</div>;

  const diskPercent = status.disk_total ? Math.round(((status.disk_used || 0) / status.disk_total) * 100) : 0;

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4">
        <div className="bg-gray-900 rounded-lg p-4 border border-gray-800">
          <div className="text-sm text-gray-400 mb-1">State</div>
          <div className="flex items-center gap-2">
            <span className={`w-3 h-3 rounded-full ${stateColors[status.state] || 'bg-gray-500'}`} />
            <span className="text-lg font-medium capitalize">{status.state}</span>
          </div>
        </div>
        <div className="bg-gray-900 rounded-lg p-4 border border-gray-800">
          <div className="text-sm text-gray-400 mb-1">Temperature</div>
          <div className="text-lg font-medium">{status.temperature.toFixed(1)}&deg;C</div>
        </div>
        <div className="bg-gray-900 rounded-lg p-4 border border-gray-800">
          <div className="text-sm text-gray-400 mb-1">Disk Usage</div>
          <div className="text-lg font-medium">{diskPercent}%</div>
          <div className="w-full bg-gray-800 rounded-full h-2 mt-2">
            <div className="bg-blue-500 h-2 rounded-full" style={{ width: `${diskPercent}%` }} />
          </div>
          <div className="text-xs text-gray-500 mt-1">
            {formatBytes(status.disk_used || 0)} / {formatBytes(status.disk_total || 0)}
          </div>
        </div>
        <div className="bg-gray-900 rounded-lg p-4 border border-gray-800">
          <div className="text-sm text-gray-400 mb-1">Last Archive</div>
          <div className="text-lg font-medium">
            {status.last_archive && status.last_archive !== '0001-01-01T00:00:00Z'
              ? new Date(status.last_archive).toLocaleString()
              : 'Never'}
          </div>
          {status.archive_clips > 0 && (
            <div className="text-xs text-gray-500 mt-1">
              {status.archive_clips} clips ({formatBytes(status.archive_bytes)})
            </div>
          )}
          {status.archive_count > 0 && (
            <div className="text-xs text-gray-500 mt-0.5">
              Total: {status.total_archive_clips} clips ({formatBytes(status.total_archive_bytes)}) over {status.archive_count} archives
            </div>
          )}
        </div>

        {status.wifi_ssid && (
          <div className="bg-gray-900 rounded-lg p-4 border border-gray-800 col-span-2">
            <div className="text-sm text-gray-400 mb-1">WiFi</div>
            <div className="flex items-center justify-between">
              <div>
                <div className="text-lg font-medium">{status.wifi_ssid}</div>
                <div className="text-xs text-gray-500">{status.wifi_ip}</div>
              </div>
              <div className="text-right">
                <div className="text-sm text-gray-300">{status.wifi_signal_dbm} dBm</div>
                <div className="w-20 bg-gray-800 rounded-full h-2 mt-1">
                  <div
                    className="bg-green-500 h-2 rounded-full"
                    style={{ width: `${signalPercent(status.wifi_signal_dbm)}%` }}
                  />
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
      <div className="flex gap-2">
        <button
          onClick={() => api.triggerArchive()}
          className="px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded-md text-sm transition-colors"
        >
          Trigger Archive
        </button>
      </div>
      {status.last_error && (
        <div className="bg-red-900/30 border border-red-800 rounded-lg p-3 text-sm text-red-300">
          {status.last_error}
        </div>
      )}

      {/* Update section */}
      <div className="bg-gray-900 rounded-lg p-4 border border-gray-800">
        <div className="flex items-center justify-between">
          <div className="text-xs text-gray-600">{status.version}</div>
          <div className="flex items-center gap-2">
            {updating ? (
              <span className="text-sm text-yellow-400">Updating... page will reload</span>
            ) : updateInfo?.available ? (
              <>
                <span className="text-sm text-green-400">{updateInfo.version} available</span>
                <button
                  onClick={applyUpdate}
                  className="px-3 py-1 bg-green-600 hover:bg-green-700 rounded text-sm transition-colors"
                >
                  Update Now
                </button>
              </>
            ) : updateInfo && !updateInfo.error ? (
              <span className="text-sm text-gray-500">Up to date</span>
            ) : updateInfo?.error ? (
              <span className="text-sm text-red-400">{updateInfo.error}</span>
            ) : null}
            <button
              onClick={checkForUpdate}
              disabled={checkingUpdate || updating}
              className="px-3 py-1 bg-gray-800 hover:bg-gray-700 disabled:opacity-50 border border-gray-700 rounded text-sm text-gray-300"
            >
              {checkingUpdate ? 'Checking...' : 'Check for Updates'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
