import '@testing-library/jest-dom';

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
