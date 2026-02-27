import { useEffect, useState } from 'react';
import { api } from '../lib/api';
import type { FileEntry } from '../lib/api';
import { formatBytes } from '../lib/format';

export function Files() {
  const [path, setPath] = useState('TeslaCam');
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [error, setError] = useState('');

  const loadFiles = (p: string) => {
    setPath(p);
    setError('');
    api.getFiles(p).then(setFiles).catch(e => setError(e.message));
  };

  useEffect(() => { loadFiles('TeslaCam'); }, []);

  const breadcrumbs = path.split('/').filter(Boolean);

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-1 text-sm text-gray-400">
        <button onClick={() => loadFiles('TeslaCam')} className="hover:text-white">/TeslaCam</button>
        {breadcrumbs.slice(1).map((part, i) => {
          const fullPath = breadcrumbs.slice(0, i + 2).join('/');
          return (
            <span key={fullPath}>
              <span className="mx-1">/</span>
              <button onClick={() => loadFiles(fullPath)} className="hover:text-white">{part}</button>
            </span>
          );
        })}
      </div>

      {error && <div className="text-red-400 text-sm">{error}</div>}

      <div className="bg-gray-900 rounded-lg border border-gray-800 divide-y divide-gray-800">
        {files.length === 0 && <div className="p-4 text-gray-500 text-sm">No files</div>}
        {files.map(file => (
          <div key={file.path} className="flex items-center justify-between p-3 hover:bg-gray-800/50">
            <div className="flex items-center gap-2">
              {file.is_dir ? (
                <button onClick={() => loadFiles(file.path)} className="text-blue-400 hover:text-blue-300">
                  {file.name}/
                </button>
              ) : (
                <span>{file.name}</span>
              )}
            </div>
            <div className="flex items-center gap-3 text-sm text-gray-500">
              {!file.is_dir && <span>{formatBytes(file.size)}</span>}
              {!file.is_dir && (
                <a href={api.downloadURL(file.path)} className="text-blue-400 hover:text-blue-300">
                  Download
                </a>
              )}
              <button
                onClick={async () => {
                  if (confirm(`Delete ${file.name}?`)) {
                    await api.deleteFile(file.path);
                    loadFiles(path);
                  }
                }}
                className="text-red-400 hover:text-red-300"
              >
                Delete
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
