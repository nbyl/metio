import { Toaster } from 'sonner';
import { useTheme } from '../theme-provider';

/** Toaster following the resolved theme instead of a hardcoded value. */
export function ThemedToaster() {
  const { resolvedTheme } = useTheme();
  return <Toaster position="top-right" theme={resolvedTheme} richColors />;
}
