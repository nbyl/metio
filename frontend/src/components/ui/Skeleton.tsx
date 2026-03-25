import type { HTMLAttributes } from 'react';
import { cn } from '../../lib/utils';

export interface SkeletonProps extends HTMLAttributes<HTMLDivElement> {
  /** Additional CSS classes */
  className?: string;
}

/**
 * Skeleton loading placeholder component.
 *
 * @example
 * ```tsx
 * // Basic usage
 * <Skeleton className="h-4 w-32" />
 *
 * // As a text placeholder
 * <Skeleton className="h-4 w-full" />
 *
 * // As a button placeholder
 * <Skeleton className="h-9 w-24" />
 * ```
 */
export function Skeleton({ className, ...props }: SkeletonProps) {
  return (
    <div
      className={cn('skeleton animate-pulse rounded', className)}
      {...props}
    />
  );
}
