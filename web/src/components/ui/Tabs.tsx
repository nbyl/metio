import type { ReactNode } from 'react';
import { Tabs as TabsPrimitive } from 'radix-ui';
import { cn } from '@/lib/utils';

export interface TabsProps {
  /** Currently active tab value */
  value: string;
  /** Callback when a tab is selected */
  onValueChange: (value: string) => void;
  /** Additional CSS classes */
  className?: string;
  children: ReactNode;
}

/**
 * Accessible tabs container that ties together {@link TabList}, {@link Tab}
 * and {@link TabPanel}. Built on Radix UI's Tabs primitive (roving tabindex,
 * arrow-key navigation, ARIA wiring).
 *
 * @example
 * ```tsx
 * <Tabs value={active} onValueChange={setActive}>
 *   <TabList>
 *     <Tab value="settings">Settings</Tab>
 *     <Tab value="backup">Backup</Tab>
 *   </TabList>
 *   <TabPanel value="settings">...</TabPanel>
 * </Tabs>
 * ```
 */
export function Tabs({ value, onValueChange, className, children }: TabsProps) {
  return (
    <TabsPrimitive.Root
      data-slot="tabs"
      value={value}
      onValueChange={onValueChange}
      className={cn('tabs', className)}
    >
      {children}
    </TabsPrimitive.Root>
  );
}

export interface TabListProps {
  /** Additional CSS classes */
  className?: string;
  children: ReactNode;
}

/**
 * Horizontal list of {@link Tab} triggers. Supports arrow-key and Home/End
 * keyboard navigation between tabs.
 */
export function TabList({ className, children }: TabListProps) {
  return (
    <TabsPrimitive.List
      data-slot="tablist"
      className={cn('tablist', className)}
    >
      {children}
    </TabsPrimitive.List>
  );
}

export interface TabProps {
  /** Value that activates this tab when selected */
  value: string;
  /** Whether the tab is disabled */
  disabled?: boolean;
  /** Additional CSS classes */
  className?: string;
  children: ReactNode;
}

/**
 * A selectable tab trigger. The active tab is styled with an underline accent.
 */
export function Tab({
  value,
  disabled = false,
  className,
  children,
}: TabProps) {
  return (
    <TabsPrimitive.Trigger
      data-slot="tab"
      value={value}
      disabled={disabled}
      className={cn(
        'inline-flex items-center px-4 py-2 text-sm font-medium border-b-2 transition-colors',
        'focus:outline-none focus-visible:ring-2 focus-visible:ring-green-500 focus-visible:ring-offset-0',
        'data-[state=active]:border-green-500 data-[state=active]:text-white',
        'data-[state=inactive]:border-transparent data-[state=inactive]:text-slate-400 hover:text-slate-200',
        'disabled:opacity-50',
        className
      )}
    >
      {children}
    </TabsPrimitive.Trigger>
  );
}

export interface TabPanelProps {
  /** Value of the tab this panel belongs to */
  value: string;
  /** Additional CSS classes */
  className?: string;
  children: ReactNode;
}

/**
 * Content shown for the tab with the matching {@link TabPanelProps.value}.
 */
export function TabPanel({ value, className, children }: TabPanelProps) {
  return (
    <TabsPrimitive.Content
      data-slot="tabpanel"
      value={value}
      className={cn('tabpanel', className)}
    >
      {children}
    </TabsPrimitive.Content>
  );
}
