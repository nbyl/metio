import { useContext } from 'react';
import { AuthContext, type AuthContextValue } from '../contexts/AuthContext';

/**
 * Hook to access authentication state.
 * Must be used within an AuthProvider.
 *
 * @returns Object containing:
 *   - isAuthenticated: boolean indicating if user is logged in
 *   - user: AuthUser object with email, or null if not authenticated
 *   - isLoading: boolean indicating if auth check is in progress
 *   - checkAuth: function to re-check authentication (call after API 401 errors)
 *
 * @example
 * ```tsx
 * const { isAuthenticated, user, isLoading } = useAuth();
 *
 * if (isLoading) return <div>Loading...</div>;
 * if (!isAuthenticated) return <div>Please log in</div>;
 *
 * return <div>Welcome, {user?.email}</div>;
 * ```
 */
export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
