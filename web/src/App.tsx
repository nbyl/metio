import { Routes, Route, useParams } from 'react-router-dom';
import { useAuth } from './hooks/useAuth';
import { ProtectedRoute } from './components/ProtectedRoute';
import { Layout } from './components/layout/Layout';
import { Header } from './components/layout/Header';
import { ServerDashboard, ServerSetupWizard, ProvisioningProgress } from './components/server';

/**
 * Dashboard component - main server control panel
 */
function Dashboard() {
  const { user } = useAuth();

  return (
    <Layout>
      <Header email={user?.email} showUser />
      <ServerDashboard />
    </Layout>
  );
}

/**
 * Provisioning page - shows provisioning progress for a specific server
 */
function ProvisioningPage() {
  const { id } = useParams<{ id: string }>();

  return (
    <Layout>
      <Header />
      {id && <ProvisioningProgress serverId={id} />}
    </Layout>
  );
}

/**
 * Main App component with routing
 */
function App() {
  return (
    <Routes>
      <Route
        path="/"
        element={
          <ProtectedRoute>
            <Dashboard />
          </ProtectedRoute>
        }
      />
      <Route
        path="/servers/new"
        element={
          <ProtectedRoute>
            <ServerSetupWizard />
          </ProtectedRoute>
        }
      />
      <Route
        path="/servers/:id/provisioning"
        element={
          <ProtectedRoute>
            <ProvisioningPage />
          </ProtectedRoute>
        }
      />
    </Routes>
  );
}

export default App;
