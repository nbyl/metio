import { Routes, Route } from 'react-router-dom';
import { useAuth } from './hooks/useAuth';
import { ProtectedRoute } from './components/ProtectedRoute';
import { Layout } from './components/layout/Layout';
import { Header } from './components/layout/Header';
import { ServerDashboard } from './components/server';

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
    </Routes>
  );
}

export default App;
