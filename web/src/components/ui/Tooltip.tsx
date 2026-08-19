import type { ReactNode } from 'react';
import { Tooltip as TooltipPrimitive } from 'radix-ui';
import { cn } from '@/lib/utils';

export interface TooltipProps {
  /** The content to show in the tooltip */
  content: ReactNode;
  /** The element that triggers the tooltip */
  children: ReactNode;
  /** Additional CSS classes for the tooltip */
  className?: string;
  /** Delay before showing tooltip in ms */
  delay?: number;
}

/**
 * Tooltip component that shows content on hover or focus.
 *
 * Built on Radix UI's Tooltip primitive. The content is portaled to the body
 * and positioned around the trigger; multiline content (newlines) is rendered
 * via `whitespace-pre-line`.
 */
export function Tooltip({
  content,
  children,
  className,
  delay = 200,
}: TooltipProps) {
  return (
    <TooltipPrimitive.Provider delayDuration={delay}>
      <TooltipPrimitive.Root delayDuration={delay} disableHoverableContent>
        <TooltipPrimitive.Trigger asChild>{children}</TooltipPrimitive.Trigger>
        <TooltipPrimitive.Portal>
          <TooltipPrimitive.Content
            data-slot="tooltip"
            sideOffset={4}
            className={cn(
              'z-50 max-w-xs whitespace-pre-line px-2 py-1 text-xs text-white shadow-lg rounded-md bg-slate-900',
              className
            )}
          >
            {content}
          </TooltipPrimitive.Content>
        </TooltipPrimitive.Portal>
      </TooltipPrimitive.Root>
    </TooltipPrimitive.Provider>
  );
}
