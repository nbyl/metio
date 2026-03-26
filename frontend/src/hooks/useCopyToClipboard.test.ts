import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useCopyToClipboard } from './useCopyToClipboard';

describe('useCopyToClipboard', () => {
  const originalClipboard = navigator.clipboard;

  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    // Restore original clipboard
    Object.defineProperty(navigator, 'clipboard', {
      value: originalClipboard,
      writable: true,
    });
  });

  describe('modern Clipboard API', () => {
    beforeEach(() => {
      Object.defineProperty(navigator, 'clipboard', {
        value: {
          writeText: vi.fn().mockResolvedValue(undefined),
        },
        writable: true,
      });
    });

    it('copies text using navigator.clipboard.writeText', async () => {
      const { result } = renderHook(() => useCopyToClipboard());

      await act(async () => {
        await result.current.copy('test text');
      });

      expect(navigator.clipboard.writeText).toHaveBeenCalledWith('test text');
    });

    it('sets copied to true on success', async () => {
      const { result } = renderHook(() => useCopyToClipboard());

      expect(result.current.copied).toBe(false);

      await act(async () => {
        await result.current.copy('test text');
      });

      expect(result.current.copied).toBe(true);
    });

    it('returns true on success', async () => {
      const { result } = renderHook(() => useCopyToClipboard());

      let success = false;
      await act(async () => {
        success = await result.current.copy('test text');
      });

      expect(success).toBe(true);
    });

    it('resets copied to false after delay', async () => {
      const { result } = renderHook(() => useCopyToClipboard(1000));

      await act(async () => {
        await result.current.copy('test text');
      });

      expect(result.current.copied).toBe(true);

      act(() => {
        vi.advanceTimersByTime(1000);
      });

      expect(result.current.copied).toBe(false);
    });

    it('uses default reset delay of 2000ms', async () => {
      const { result } = renderHook(() => useCopyToClipboard());

      await act(async () => {
        await result.current.copy('test text');
      });

      expect(result.current.copied).toBe(true);

      act(() => {
        vi.advanceTimersByTime(1999);
      });

      expect(result.current.copied).toBe(true);

      act(() => {
        vi.advanceTimersByTime(1);
      });

      expect(result.current.copied).toBe(false);
    });

    it('sets error on clipboard failure', async () => {
      const clipboardError = new Error('Clipboard access denied');
      Object.defineProperty(navigator, 'clipboard', {
        value: {
          writeText: vi.fn().mockRejectedValue(clipboardError),
        },
        writable: true,
      });

      const { result } = renderHook(() => useCopyToClipboard());

      await act(async () => {
        await result.current.copy('test text');
      });

      expect(result.current.error).toEqual(clipboardError);
      expect(result.current.copied).toBe(false);
    });

    it('returns false on failure', async () => {
      Object.defineProperty(navigator, 'clipboard', {
        value: {
          writeText: vi.fn().mockRejectedValue(new Error('Failed')),
        },
        writable: true,
      });

      const { result } = renderHook(() => useCopyToClipboard());

      let success = true;
      await act(async () => {
        success = await result.current.copy('test text');
      });

      expect(success).toBe(false);
    });

    it('clears error on subsequent successful copy', async () => {
      const writeTextMock = vi
        .fn()
        .mockRejectedValueOnce(new Error('First call fails'))
        .mockResolvedValueOnce(undefined);

      Object.defineProperty(navigator, 'clipboard', {
        value: { writeText: writeTextMock },
        writable: true,
      });

      const { result } = renderHook(() => useCopyToClipboard());

      // First call fails
      await act(async () => {
        await result.current.copy('test text');
      });
      expect(result.current.error).not.toBeNull();

      // Second call succeeds
      await act(async () => {
        await result.current.copy('test text');
      });
      expect(result.current.error).toBeNull();
      expect(result.current.copied).toBe(true);
    });
  });

  describe('fallback for older browsers', () => {
    let execCommandMock: ReturnType<typeof vi.fn>;
    let originalExecCommand: typeof document.execCommand;

    beforeEach(() => {
      // Remove clipboard API to trigger fallback
      Object.defineProperty(navigator, 'clipboard', {
        value: undefined,
        writable: true,
      });

      originalExecCommand = document.execCommand;
      execCommandMock = vi.fn().mockReturnValue(true);
      document.execCommand = execCommandMock;
    });

    afterEach(() => {
      document.execCommand = originalExecCommand;
    });

    it('uses execCommand when clipboard API is unavailable', async () => {
      const { result } = renderHook(() => useCopyToClipboard());

      await act(async () => {
        await result.current.copy('fallback text');
      });

      expect(execCommandMock).toHaveBeenCalledWith('copy');
      expect(result.current.copied).toBe(true);
    });

    it('sets error when execCommand fails', async () => {
      execCommandMock.mockReturnValue(false);

      const { result } = renderHook(() => useCopyToClipboard());

      await act(async () => {
        await result.current.copy('fallback text');
      });

      expect(result.current.error).not.toBeNull();
      expect(result.current.error?.message).toBe('execCommand copy failed');
      expect(result.current.copied).toBe(false);
    });
  });

  describe('edge cases', () => {
    beforeEach(() => {
      Object.defineProperty(navigator, 'clipboard', {
        value: {
          writeText: vi.fn().mockResolvedValue(undefined),
        },
        writable: true,
      });
    });

    it('handles non-Error exceptions', async () => {
      Object.defineProperty(navigator, 'clipboard', {
        value: {
          writeText: vi.fn().mockRejectedValue('string error'),
        },
        writable: true,
      });

      const { result } = renderHook(() => useCopyToClipboard());

      await act(async () => {
        await result.current.copy('test');
      });

      expect(result.current.error).toBeInstanceOf(Error);
      expect(result.current.error?.message).toBe('Failed to copy to clipboard');
    });

    it('copy function is stable across renders', async () => {
      const { result, rerender } = renderHook(() => useCopyToClipboard());

      const firstCopy = result.current.copy;
      rerender();
      const secondCopy = result.current.copy;

      expect(firstCopy).toBe(secondCopy);
    });
  });
});
