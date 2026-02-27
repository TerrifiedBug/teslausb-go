import type { ReactNode } from 'react';

const tabs = [
  { id: 'dashboard', label: 'Dashboard' },
  { id: 'files', label: 'Files' },
  { id: 'config', label: 'Config' },
  { id: 'logs', label: 'Logs' },
];

export function Layout({ children, activeTab, onTabChange }: {
  children: ReactNode;
  activeTab: string;
  onTabChange: (tab: string) => void;
}) {
  return (
    <div className="min-h-screen bg-gray-950 text-gray-100">
      <header className="border-b border-gray-800 px-4 py-3">
        <div className="max-w-4xl mx-auto flex items-center justify-between">
          <h1 className="text-lg font-semibold">TeslaUSB</h1>
          <nav className="flex gap-1">
            {tabs.map(tab => (
              <button
                key={tab.id}
                onClick={() => onTabChange(tab.id)}
                className={`px-3 py-1.5 rounded-md text-sm transition-colors ${
                  activeTab === tab.id
                    ? 'bg-gray-800 text-white'
                    : 'text-gray-400 hover:text-gray-200 hover:bg-gray-900'
                }`}
              >
                {tab.label}
              </button>
            ))}
          </nav>
        </div>
      </header>
      <main className="max-w-4xl mx-auto p-4">
        {children}
      </main>
    </div>
  );
}
