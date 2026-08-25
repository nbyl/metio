import { Routes, Route, Navigate, useParams } from 'react-router-dom';
import { ProtectedRoute } from './components/ProtectedRoute';
import { AuthenticatedLayout } from './components/layout/AuthenticatedLayout';
import { SetupWizard } from './components/setup/SetupWizard';
import { useSetupStatus } from './hooks/useSetupStatus';
import {
  ServerDashboard,
  ServerSetupWizard,
  ProvisioningProgress,
} from './components/server';
import { BackupCatalogPage } from './components/backup/BackupCatalogPage';

function Dashboard() {
  const { data: status, isLoading } = useSetupStatus();

  if (isLoading) {
    return (
      <AuthenticatedLayout>
        <div className="flex items-center justify-center py-12 text-muted-foreground">
          Loading...
        </div>
      </AuthenticatedLayout>
    );
  }

  if (status && !status.initialized && status.serverCount === 0) {
    return <Navigate to="/setup" replace />;
  }

  return (
    <AuthenticatedLayout>
      <ServerDashboard />
    </AuthenticatedLayout>
  );
}

function ProvisioningPage() {
  const { id } = useParams<{ id: string }>();
  return (
    <AuthenticatedLayout>
      {id && <ProvisioningProgress serverId={id} />}
    </AuthenticatedLayout>
  );
}

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
            <AuthenticatedLayout>
              <ServerSetupWizard />
            </AuthenticatedLayout>
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
            <AuthenticatedLayout>
              <SetupWizard />
            </AuthenticatedLayout>
          </ProtectedRoute>
        }
      />
      <Route
        path="/backups"
        element={
          <ProtectedRoute>
            <AuthenticatedLayout>
              <BackupCatalogPage />
            </AuthenticatedLayout>
          </ProtectedRoute>
        }
      />
    </Routes>
  );
}

export default App;
