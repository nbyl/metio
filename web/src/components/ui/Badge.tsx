import type { ReactNode } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

const badgeVariants = cva('badge', {
  variants: {
    variant: {
      online: 'badge-online',
      offline: 'badge-offline',
      transitioning: 'badge-transitioning',
    },
  },
  defaultVariants: {
    variant: 'online',
  },
});

export interface BadgeProps
  extends
    React.HTMLAttributes<HTMLSpanElement>,
    VariantProps<typeof badgeVariants> {
  /** Badge style variant based on status */
  variant: 'online' | 'offline' | 'transitioning';
  /** Badge content */
  children: ReactNode;
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
export function Badge({ variant, children, className, ...props }: BadgeProps) {
  return (
    <span
      data-slot="badge"
      data-variant={variant}
      className={cn(badgeVariants({ variant }), className)}
      {...props}
    >
      {children}
    </span>
  );
}
// eslint-disable-next-line react-refresh/only-export-components
export { badgeVariants };
