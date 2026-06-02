import { useState, useCallback } from 'react';

export interface UseCopyToClipboardResult {
  /** Copy text to clipboard */
  copy: (text: string) => Promise<boolean>;
  /** Whether copy was successful (resets after delay) */
  copied: boolean;
  /** Error if copy failed */
  error: Error | null;
}

/**
 * Hook for copying text to clipboard with fallback for older browsers.
 *
 * @param resetDelay - Time in ms before `copied` resets to false (default: 2000)
 * @returns Object with copy function, copied state, and error
 *
 * @example
 * ```tsx
 * const { copy, copied, error } = useCopyToClipboard();
 *
 * <button onClick={() => copy('Hello!')}>
 *   {copied ? 'Copied!' : 'Copy'}
 * </button>
 * ```
 */
export function useCopyToClipboard(resetDelay = 2000): UseCopyToClipboardResult {
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const copy = useCallback(
    async (text: string): Promise<boolean> => {
      try {
        // Modern Clipboard API
        if (navigator.clipboard?.writeText) {
          await navigator.clipboard.writeText(text);
        } else {
          // Fallback for older browsers using execCommand
          const textArea = document.createElement('textarea');
          textArea.value = text;
          // Prevent scrolling to bottom
          textArea.style.position = 'fixed';
          textArea.style.left = '-9999px';
          textArea.style.top = '0';
          document.body.appendChild(textArea);
          textArea.focus();
          textArea.select();

          const successful = document.execCommand('copy');
          document.body.removeChild(textArea);

          if (!successful) {
            throw new Error('execCommand copy failed');
          }
        }

        setCopied(true);
        setError(null);
        setTimeout(() => setCopied(false), resetDelay);
        return true;
      } catch (err) {
        const copyError =
          err instanceof Error ? err : new Error('Failed to copy to clipboard');
        setError(copyError);
        setCopied(false);
        return false;
      }
    },
    [resetDelay]
  );

  return { copy, copied, error };
}
