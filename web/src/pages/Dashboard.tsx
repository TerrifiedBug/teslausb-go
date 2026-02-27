import { useEffect, useState } from 'react';
import { api } from '../lib/api';
import type { Status } from '../lib/api';
import { WSClient } from '../lib/ws';

const stateColors: Record<string, string> = {
  away: 'bg-blue-500',
  arriving: 'bg-yellow-500',
  archiving: 'bg-orange-500',
  idle: 'bg-green-500',
  booting: 'bg-gray-500',
  error: 'bg-red-500',
};

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

export function Dashboard() {
  const [status, setStatus] = useState<Status | null>(null);

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
        </div>
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
      <div className="text-xs text-gray-600">v{status.version}</div>
    </div>
  );
}
