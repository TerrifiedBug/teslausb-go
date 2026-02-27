import { useEffect, useState, useRef } from 'react';
import { api } from '../lib/api';
import { WSClient } from '../lib/ws';

export function Logs() {
  const [logs, setLogs] = useState<string[]>([]);
  const [filter, setFilter] = useState('all');
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    api.getLogs().then(setLogs).catch(console.error);

    const ws = new WSClient();
    ws.connect();
    ws.onMessage((data) => {
      if (data.type === 'log') {
        setLogs(prev => [...prev.slice(-499), data.line]);
      }
    });

    return () => ws.disconnect();
  }, []);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [logs]);

  const filtered = filter === 'all' ? logs : logs.filter(l => l.toLowerCase().includes(filter));

  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        {['all', 'error', 'warn', 'info'].map(level => (
          <button
            key={level}
            onClick={() => setFilter(level === 'all' ? 'all' : level)}
            className={`px-3 py-1 rounded text-xs ${
              filter === level ? 'bg-gray-700 text-white' : 'bg-gray-900 text-gray-500'
            }`}
          >
            {level.toUpperCase()}
          </button>
        ))}
      </div>
      <div className="bg-gray-900 rounded-lg border border-gray-800 p-3 h-[70vh] overflow-y-auto font-mono text-xs">
        {filtered.map((line, i) => (
          <div key={i} className="py-0.5 text-gray-400 hover:text-gray-200">{line}</div>
        ))}
        <div ref={bottomRef} />
      </div>
    </div>
  );
}
