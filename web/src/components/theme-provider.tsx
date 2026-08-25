import { createContext, useContext, useEffect, useState } from 'react';

type Theme = 'light' | 'dark' | 'system';

interface ThemeProviderState {
  /** Selected mode; `system` follows the OS preference */
  theme: Theme;
  setTheme: (theme: Theme) => void;
  /** Effectively applied theme after resolving `system` */
  resolvedTheme: 'light' | 'dark';
}

const STORAGE_KEY = 'metio-theme';

const ThemeProviderContext = createContext<ThemeProviderState>({
  theme: 'system',
  setTheme: () => {},
  resolvedTheme: 'light',
});

function readStoredTheme(): Theme {
  if (typeof window === 'undefined') return 'system';
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (stored === 'light' || stored === 'dark' || stored === 'system') {
      return stored;
    }
  } catch {
    // localStorage unavailable (e.g. storage disabled) — fall back to system
  }
  return 'system';
}

function systemPrefersDark(): boolean {
  return (
    typeof window !== 'undefined' &&
    window.matchMedia('(prefers-color-scheme: dark)').matches
  );
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setTheme] = useState<Theme>(readStoredTheme);
  const [systemDark, setSystemDark] = useState(systemPrefersDark);

  // Keep following live OS preference changes while in system mode.
  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const onChange = (event: MediaQueryListEvent) =>
      setSystemDark(event.matches);
    media.addEventListener('change', onChange);
    return () => media.removeEventListener('change', onChange);
  }, []);

  useEffect(() => {
    const dark = theme === 'dark' || (theme === 'system' && systemDark);
    const root = document.documentElement;
    root.classList.toggle('dark', dark);
    root.style.colorScheme = dark ? 'dark' : 'light';
  }, [theme, systemDark]);

  const value: ThemeProviderState = {
    theme,
    setTheme: (next: Theme) => {
      try {
        window.localStorage.setItem(STORAGE_KEY, next);
      } catch {
        // ignore persistence failures; the in-memory choice still applies
      }
      setTheme(next);
    },
    resolvedTheme: theme === 'system' ? (systemDark ? 'dark' : 'light') : theme,
  };

  return (
    <ThemeProviderContext.Provider value={value}>
      {children}
    </ThemeProviderContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useTheme() {
  return useContext(ThemeProviderContext);
}
