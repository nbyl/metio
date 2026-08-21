import { Routes, Route, useParams, Navigate } from 'react-router-dom';
import { useAuth } from './hooks/useAuth';
import { ProtectedRoute } from './components/ProtectedRoute';
import { Layout } from './components/layout/Layout';
import { Header } from './components/layout/Header';
import { SetupWizard } from './components/setup/SetupWizard';
import { useSetupStatus } from './hooks/useSetupStatus';
import {
  ServerDashboard,
  ServerSetupWizard,
  ProvisioningProgress,
} from './components/server';

/**
 * Dashboard component - main server control panel
 */
function Dashboard() {
  const { user } = useAuth();
  const { data: status, isLoading } = useSetupStatus();

  if (isLoading) {
    return (
      <Layout>
        <Header email={user?.email} showUser />
        <div className="flex items-center justify-center py-12 text-muted-foreground">
          Loading...
        </div>
      </Layout>
    );
  }

  if (status && !status.initialized && status.serverCount === 0) {
    return <Navigate to="/setup" replace />;
  }

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
            <Layout>
              <Header />
              <ServerSetupWizard />
            </Layout>
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
      <Route
        path="/setup"
        element={
          <ProtectedRoute>
            <Layout>
              <Header />
              <SetupWizard />
            </Layout>
          </ProtectedRoute>
        }
      />
    </Routes>
  );
}

export default App;
