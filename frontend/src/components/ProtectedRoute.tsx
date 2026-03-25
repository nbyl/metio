import type { ReactNode } from 'react';
import { useAuth } from '../hooks/useAuth';

interface ProtectedRouteProps {
  children: ReactNode;
}

/**
 * Wrapper component that redirects to login if user is not authenticated.
 * Shows a loading state while checking authentication.
 * Redirects to /auth/login (Go backend OAuth endpoint) if not authenticated.
 */
export function ProtectedRoute({ children }: ProtectedRouteProps) {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return (
      <div className="dark min-h-screen bg-background p-8">
        <div className="container">
          <div className="page-header">
            <h1 className="title">Metio</h1>
            <p className="subtitle">Minecraft Server Controller</p>
          </div>
          <div className="card">
            <div className="card-content">
              <p className="text-muted">Checking authentication...</p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (!isAuthenticated) {
    // Redirect to Go backend login endpoint
    window.location.href = '/auth/login';
    return null;
  }

  return <>{children}</>;
}
