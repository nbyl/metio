import type { ComponentProps } from 'react';
import { Separator as SeparatorPrimitive } from 'radix-ui';
import { cn } from '@/lib/utils';

export type SeparatorProps = ComponentProps<typeof SeparatorPrimitive.Root>;

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
export function Separator({
  className,
  orientation = 'horizontal',
  ...props
}: SeparatorProps) {
  return (
    <SeparatorPrimitive.Root
      data-slot="separator"
      orientation={orientation}
      className={cn('separator', className)}
      {...props}
    />
  );
}
