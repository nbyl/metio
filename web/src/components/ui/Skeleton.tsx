import type { HTMLAttributes } from 'react';
import { cn } from '@/lib/utils';

export type SkeletonProps = HTMLAttributes<HTMLDivElement>;

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
      data-slot="skeleton"
      className={cn('skeleton animate-pulse rounded', className)}
      {...props}
    />
  );
}
