import { Gamepad2 } from 'lucide-react';

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
  return (
    <div className="page-header">
      <div>
        <h1 className="title">
          <Gamepad2 className="h-10 w-10" aria-hidden="true" />
          Metio
        </h1>
        <p className="subtitle">Minecraft Server Controller</p>
      </div>
      {showUser && email && (
        <div className="header-user">
          <span className="user-email">{email}</span>
          <a href="/auth/logout" className="btn btn-outline btn-sm">
            Logout
          </a>
        </div>
      )}
    </div>
  );
}
