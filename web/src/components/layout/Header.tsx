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
    <header className="relative flex items-center justify-center">
      <div className="text-center">
        <h1 className="flex items-center justify-center gap-3 text-4xl font-bold text-foreground">
          <Gamepad2 className="h-10 w-10" aria-hidden="true" />
          Metio
        </h1>
        <p className="text-muted-foreground">Minecraft Server Controller</p>
        {options?.controllerVersion && (
          <p className="text-xs text-muted-foreground/70">
            Version {options.controllerVersion}
          </p>
        )}
      </div>
      {showUser && email && (
        <div className="absolute right-0 flex items-center gap-3">
          <span className="text-sm text-muted-foreground">{email}</span>
          <Button asChild variant="outline" size="sm">
            <a href="/auth/logout">Logout</a>
          </Button>
        </div>
      )}
    </header>
  );
}
