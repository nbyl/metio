import type { ReactNode } from 'react';
import { cn } from '../../lib/utils';
import { TooltipProvider } from '../ui/Tooltip';

export interface LayoutProps {
  /** Page content */
  children: ReactNode;
  /** Additional CSS classes for the container */
  className?: string;
}

/**
 * Full page layout wrapper with dark theme and centered container.
 *
 * @example
 * ```tsx
 * <Layout>
 *   <Header />
 *   <Card>Content</Card>
 * </Layout>
 * ```
 */
export function Layout({ children, className }: LayoutProps) {
  return (
    <div className="dark min-h-screen bg-background p-8">
      <TooltipProvider>
        <div className={cn('container', className)}>{children}</div>
      </TooltipProvider>
    </div>
  );
}
