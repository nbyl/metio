import {
  createContext,
  useCallback,
  useEffect,
  useState,
  type ReactNode,
} from 'react';
import type { AuthUser, AuthMeResponse } from '../types/auth';

export interface AuthContextValue {
  isAuthenticated: boolean;
  user: AuthUser | null;
  isLoading: boolean;
  checkAuth: () => Promise<void>;
}

// eslint-disable-next-line react-refresh/only-export-components
export const AuthContext = createContext<AuthContextValue | null>(null);

interface AuthProviderProps {
  children: ReactNode;
}

/**
 * Authentication context provider that manages auth state for the entire app.
 * Fetches /api/auth/me on mount to check authentication status.
 * Provides checkAuth() for re-checking auth after API errors.
 */
export function AuthProvider({ children }: AuthProviderProps) {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [user, setUser] = useState<AuthUser | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const checkAuth = useCallback(() => {
    return fetch('/api/auth/me')
      .then((response) => {
        if (response.ok) {
          return response.json() as Promise<AuthMeResponse>;
        }
        setIsAuthenticated(false);
        setUser(null);
        return Promise.resolve(null);
      })
      .then((data) => {
        if (data) {
          setIsAuthenticated(data.authenticated);
          setUser(
            data.authenticated && data.email ? { email: data.email } : null
          );
        }
      })
      .catch(() => {
        setIsAuthenticated(false);
        setUser(null);
      })
      .finally(() => {
        setIsLoading(false);
      });
  }, []);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  return (
    <AuthContext.Provider
      value={{ isAuthenticated, user, isLoading, checkAuth }}
    >
      {children}
    </AuthContext.Provider>
  );
}
