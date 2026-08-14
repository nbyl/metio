import {
  createContext,
  useContext,
  useId,
  useRef,
  type KeyboardEvent,
  type MutableRefObject,
  type ReactNode,
} from 'react'
import { cn } from '../../lib/utils'

interface TabsContextValue {
  idPrefix: string
  value: string
  onValueChange: (value: string) => void
  tabsRef: MutableRefObject<Map<string, HTMLButtonElement>>
}

const TabsContext = createContext<TabsContextValue | null>(null)

export interface TabsProps {
  /** Currently active tab value */
  value: string
  /** Callback when a tab is selected */
  onValueChange: (value: string) => void
  /** Additional CSS classes */
  className?: string
  children: ReactNode
}

/**
 * Accessible tabs container that ties together {@link TabList}, {@link Tab}
 * and {@link TabPanel}.
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
  const idPrefix = useId()
  const tabsRef = useRef<Map<string, HTMLButtonElement>>(new Map())

  return (
    <TabsContext.Provider value={{ idPrefix, value, onValueChange, tabsRef }}>
      <div className={cn('tabs', className)}>{children}</div>
    </TabsContext.Provider>
  )
}

function useTabs(): TabsContextValue {
  const context = useContext(TabsContext)
  if (!context) {
    throw new Error('Tab components must be used within a <Tabs>')
  }
  return context
}

export interface TabListProps {
  /** Additional CSS classes */
  className?: string
  children: ReactNode
}

/**
 * Horizontal list of {@link Tab} triggers. Supports arrow-key and Home/End
 * keyboard navigation between tabs.
 */
export function TabList({ className, children }: TabListProps) {
  const { tabsRef } = useTabs()

  const focusTab = (current: HTMLButtonElement, offset: number) => {
    const tabs = Array.from(tabsRef.current.values())
    if (tabs.length === 0) return
    const currentIndex = tabs.indexOf(current)
    const nextIndex = (currentIndex + offset + tabs.length) % tabs.length
    tabs[nextIndex].focus()
  }

  const handleKeyDown = (e: KeyboardEvent) => {
    const current = e.currentTarget as HTMLButtonElement
    switch (e.key) {
      case 'ArrowLeft':
        e.preventDefault()
        focusTab(current, -1)
        break
      case 'ArrowRight':
        e.preventDefault()
        focusTab(current, 1)
        break
      case 'Home':
        e.preventDefault()
        tabsRef.current.values().next().value?.focus()
        break
      case 'End': {
        e.preventDefault()
        const tabs = Array.from(tabsRef.current.values())
        tabs[tabs.length - 1]?.focus()
        break
      }
    }
  }

  return (
    <div
      role="tablist"
      onKeyDown={handleKeyDown}
      className={cn('tablist', className)}
    >
      {children}
    </div>
  )
}

export interface TabProps {
  /** Value that activates this tab when selected */
  value: string
  /** Whether the tab is disabled */
  disabled?: boolean
  /** Additional CSS classes */
  className?: string
  children: ReactNode
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
  const { idPrefix, value: activeValue, onValueChange, tabsRef } = useTabs()
  const selected = activeValue === value
  const id = `${idPrefix}-${value}-tab`
  const panelId = `${idPrefix}-${value}-panel`

  return (
    <button
      type="button"
      id={id}
      role="tab"
      aria-selected={selected}
      aria-controls={panelId}
      tabIndex={selected ? 0 : -1}
      disabled={disabled}
      onClick={() => onValueChange(value)}
      ref={(el) => {
        if (el) tabsRef.current.set(value, el)
        else tabsRef.current.delete(value)
      }}
      className={cn(
        'inline-flex items-center px-4 py-2 text-sm font-medium border-b-2 transition-colors',
        'focus:outline-none focus-visible:ring-2 focus-visible:ring-green-500 focus-visible:ring-offset-0',
        selected
          ? 'border-green-500 text-white'
          : 'border-transparent text-slate-400 hover:text-slate-200 disabled:opacity-50',
        className
      )}
    >
      {children}
    </button>
  )
}

export interface TabPanelProps {
  /** Value of the tab this panel belongs to */
  value: string
  /** Additional CSS classes */
  className?: string
  children: ReactNode
}

/**
 * Content shown for the tab with the matching {@link TabPanelProps.value}.
 */
export function TabPanel({ value, className, children }: TabPanelProps) {
  const { idPrefix, value: activeValue } = useTabs()
  const id = `${idPrefix}-${value}-panel`
  const tabId = `${idPrefix}-${value}-tab`
  const selected = activeValue === value

  return (
    <div
      id={id}
      role="tabpanel"
      aria-labelledby={tabId}
      hidden={!selected}
      className={cn('tabpanel', className)}
    >
      {children}
    </div>
  )
}
