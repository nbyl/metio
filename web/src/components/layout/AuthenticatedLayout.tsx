import type { ReactNode } from 'react';
import { useAuth } from '../../hooks/useAuth';
import { Layout } from './Layout';
import { Header } from './Header';

export interface AuthenticatedLayoutProps {
  children: ReactNode;
}

/**
 * Layout wrapper for authenticated pages. Automatically provides the
 * Header with user context (email, nav links, user menu) so individual
 * pages don't need to pass these props manually.
 *
 * @example
 * ```tsx
 * <AuthenticatedLayout>
 *   <ServerDashboard />
 * </AuthenticatedLayout>
 * ```
 */
export function AuthenticatedLayout({ children }: AuthenticatedLayoutProps) {
  const { user } = useAuth();
  return (
    <Layout>
      <Header email={user?.email} showUser />
      {children}
    </Layout>
  );
}
