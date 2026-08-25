import '@testing-library/jest-dom';

// Node's experimental localStorage is unavailable unless --localstorage-file
// is passed, so provide an in-memory implementation for theme persistence.
if (
  !('localStorage' in globalThis) ||
  typeof globalThis.localStorage?.getItem !== 'function'
) {
  class MemoryStorage {
    private store = new Map<string, string>();
    get length() {
      return this.store.size;
    }
    key(index: number) {
      return [...this.store.keys()][index] ?? null;
    }
    getItem(key: string) {
      return this.store.has(key) ? this.store.get(key)! : null;
    }
    setItem(key: string, value: string) {
      this.store.set(key, String(value));
    }
    removeItem(key: string) {
      this.store.delete(key);
    }
    clear() {
      this.store.clear();
    }
  }
  Object.defineProperty(window, 'localStorage', {
    configurable: true,
    value: new MemoryStorage(),
  });
}

// Radix UI primitives (Tooltip/Popper, Tabs) rely on ResizeObserver for
// positioning/sizing. jsdom does not implement it, so provide a no-op stub.
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}

if (!('ResizeObserver' in globalThis)) {
  globalThis.ResizeObserver =
    ResizeObserverStub as unknown as typeof ResizeObserver;
}

// Radix Select uses pointer capture while opening its trigger. jsdom does not
// implement these HTMLElement methods.
if (!HTMLElement.prototype.hasPointerCapture) {
  HTMLElement.prototype.hasPointerCapture = () => false;
  HTMLElement.prototype.setPointerCapture = () => {};
  HTMLElement.prototype.releasePointerCapture = () => {};
}

if (!HTMLElement.prototype.scrollIntoView) {
  HTMLElement.prototype.scrollIntoView = () => {};
}

// Theme resolution relies on matchMedia, which jsdom does not implement.
// Tests can override the returned value per case via the helper below.
type MediaQueryListener = (event: MediaQueryListEvent) => void;

const mediaQueryListeners = new Map<string, Set<MediaQueryListener>>();

function createMatchMediaStub(prefersDark: boolean) {
  return (query: string): MediaQueryList =>
    ({
      matches: query === '(prefers-color-scheme: dark)' ? prefersDark : false,
      media: query,
      onchange: null,
      addEventListener: (_: string, listener: MediaQueryListener) => {
        if (!mediaQueryListeners.has(query)) {
          mediaQueryListeners.set(query, new Set());
        }
        mediaQueryListeners.get(query)!.add(listener);
      },
      removeEventListener: (_: string, listener: MediaQueryListener) => {
        mediaQueryListeners.get(query)?.delete(listener);
      },
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    }) as MediaQueryList;
}

Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: createMatchMediaStub(false),
});

/**
 * Simulate an OS dark-preference change; notifies live listeners like a real
 * `prefers-color-scheme` transition would.
 */
export function setPrefersDark(prefersDark: boolean) {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    configurable: true,
    value: createMatchMediaStub(prefersDark),
  });
  const event = { matches: prefersDark } as MediaQueryListEvent;
  mediaQueryListeners
    .get('(prefers-color-scheme: dark)')
    ?.forEach((listener) => listener(event));
}
