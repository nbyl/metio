import { cn } from '../../lib/utils';

export interface SeparatorProps {
  /** Additional CSS classes */
  className?: string;
}

/**
 * Horizontal separator/divider component.
 *
 * @example
 * ```tsx
 * <CardContent>
 *   <p>Some content</p>
 *   <Separator />
 *   <p>More content</p>
 * </CardContent>
 * ```
 */
export function Separator({ className }: SeparatorProps) {
  return <div className={cn('separator', className)} role="separator" />;
}
