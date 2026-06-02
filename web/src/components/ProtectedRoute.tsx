import { useEffect, type ReactNode } from 'react';
import { useAuth } from '../hooks/useAuth';
import { Layout } from './layout/Layout';
import { Header } from './layout/Header';
import { Card, CardContent } from './ui/Card';

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

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      window.location.href = '/auth/login';
    }
  }, [isLoading, isAuthenticated]);

  if (isLoading) {
    return (
      <Layout>
        <Header />
        <Card>
          <CardContent>
            <p className="text-muted">Checking authentication...</p>
          </CardContent>
        </Card>
      </Layout>
    );
  }

  if (!isAuthenticated) {
    // Show loading while redirect is happening
    return (
      <Layout>
        <Header />
        <Card>
          <CardContent>
            <p className="text-muted">Redirecting to login...</p>
          </CardContent>
        </Card>
      </Layout>
    );
  }

  return <>{children}</>;
}
