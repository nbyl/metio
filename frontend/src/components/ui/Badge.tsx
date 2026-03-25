import type { ReactNode } from 'react';
import { cn } from '../../lib/utils';

const variantStyles = {
  online: 'badge-online',
  offline: 'badge-offline',
  transitioning: 'badge-transitioning',
} as const;

export interface BadgeProps {
  /** Badge style variant based on status */
  variant: keyof typeof variantStyles;
  /** Badge content */
  children: ReactNode;
  /** Additional CSS classes */
  className?: string;
}

/**
 * Badge component for displaying status indicators.
 *
 * @example
 * ```tsx
 * <Badge variant="online">Online</Badge>
 * <Badge variant="offline">Offline</Badge>
 * <Badge variant="transitioning">Starting...</Badge>
 * ```
 */
export function Badge({ variant, children, className }: BadgeProps) {
  return (
    <span className={cn('badge', variantStyles[variant], className)}>
      {children}
    </span>
  );
}
