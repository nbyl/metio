import { CircleUser, Gamepad2 } from 'lucide-react';
import { NavLink } from 'react-router-dom';
import { cn } from '../../lib/utils';
import { useServerOptions } from '../../hooks/useServerOptions';
import { Button } from '../ui/Button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '../ui';
import { useTheme } from '../theme-provider';

export interface HeaderProps {
  /** User email to display */
  email?: string;
  /** Whether to show user section with logout */
  showUser?: boolean;
}

const navLinkClass = ({ isActive }: { isActive: boolean }) =>
  cn(
    'text-sm text-muted-foreground transition-colors hover:text-foreground',
    isActive && 'border-b-2 border-foreground font-medium text-foreground'
  );

/**
 * Application header with Metio branding, optional navigation, and user menu.
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
  const { theme, setTheme } = useTheme();

  return (
    <div>
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
          <div className="absolute right-0">
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon-sm" aria-label="User menu">
                  <CircleUser className="h-5 w-5" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuLabel>{email}</DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuSub>
                  <DropdownMenuSubTrigger>Theme</DropdownMenuSubTrigger>
                  <DropdownMenuSubContent>
                    <DropdownMenuRadioGroup
                      value={theme}
                      onValueChange={(value) =>
                        setTheme(value as 'light' | 'dark' | 'system')
                      }
                    >
                      <DropdownMenuRadioItem value="light">
                        Light
                      </DropdownMenuRadioItem>
                      <DropdownMenuRadioItem value="dark">
                        Dark
                      </DropdownMenuRadioItem>
                      <DropdownMenuRadioItem value="system">
                        System
                      </DropdownMenuRadioItem>
                    </DropdownMenuRadioGroup>
                  </DropdownMenuSubContent>
                </DropdownMenuSub>
                <DropdownMenuSeparator />
                <DropdownMenuItem asChild>
                  <a href="/auth/logout">Logout</a>
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        )}
      </header>
      {showUser && (
        <nav className="flex justify-center gap-6 py-2">
          <NavLink to="/" end className={navLinkClass}>
            Servers
          </NavLink>
          <NavLink to="/backups" className={navLinkClass}>
            Backups
          </NavLink>
        </nav>
      )}
    </div>
  );
}
