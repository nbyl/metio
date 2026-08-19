import { Gamepad2 } from 'lucide-react';
import { useServerOptions } from '../../hooks/useServerOptions';
import { Button } from '../ui/Button';

export interface HeaderProps {
  /** User email to display */
  email?: string;
  /** Whether to show user section with logout */
  showUser?: boolean;
}

/**
 * Application header with Metio branding and optional user info.
 *
 * @example
 * ```tsx
 * // Header with user info
 * <Header email="user@example.com" showUser />
 *
 * // Header without user info (e.g., loading state)
 * <Header />
 * ```
 */
export function Header({ email, showUser = false }: HeaderProps) {
  const { data: options } = useServerOptions();

  return (
    <div className="page-header">
      <div>
        <h1 className="title">
          <Gamepad2 className="h-10 w-10" aria-hidden="true" />
          Metio
        </h1>
        <p className="subtitle">Minecraft Server Controller</p>
        {options?.controllerVersion && (
          <p className="text-xs text-slate-500">
            Version {options.controllerVersion}
          </p>
        )}
      </div>
      {showUser && email && (
        <div className="header-user">
          <span className="user-email">{email}</span>
          <Button asChild variant="outline" size="sm">
            <a href="/auth/logout">Logout</a>
          </Button>
        </div>
      )}
    </div>
  );
}
