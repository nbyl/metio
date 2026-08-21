import { Gamepad2, Moon, Sun } from 'lucide-react';
import { useServerOptions } from '../../hooks/useServerOptions';
import { Button } from '../ui/Button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from '../ui';
import { useTheme } from '../theme-provider';

export interface HeaderProps {
  /** User email to display */
  email?: string;
  /** Whether to show user section with logout */
  showUser?: boolean;
}

/**
 * Theme mode switcher (Light / Dark / System) using the resolved theme to
 * pick the visible icon.
 */
function ThemeSwitcher() {
  const { theme, setTheme } = useTheme();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          className="relative"
          aria-label="Change theme"
        >
          <Sun className="h-4 w-4 scale-100 rotate-0 transition-all dark:scale-0 dark:-rotate-90" />
          <Moon className="absolute h-4 w-4 scale-0 rotate-90 transition-all dark:scale-100 dark:rotate-0" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuRadioGroup
          value={theme}
          onValueChange={(value) => setTheme(value as 'light' | 'dark' | 'system')}
        >
          <DropdownMenuRadioItem value="light">Light</DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="dark">Dark</DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="system">System</DropdownMenuRadioItem>
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
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
          <ThemeSwitcher />
        </div>
      )}
    </header>
  );
}
