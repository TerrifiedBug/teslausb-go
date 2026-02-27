import { useState } from 'react';
import { Layout } from './components/Layout';
import { Dashboard } from './pages/Dashboard';
import { Files } from './pages/Files';
import { Config } from './pages/Config';
import { Logs } from './pages/Logs';

function App() {
  const [tab, setTab] = useState('dashboard');

  return (
    <Layout activeTab={tab} onTabChange={setTab}>
      {tab === 'dashboard' && <Dashboard />}
      {tab === 'files' && <Files />}
      {tab === 'config' && <Config />}
      {tab === 'logs' && <Logs />}
    </Layout>
  );
}

export default App;
