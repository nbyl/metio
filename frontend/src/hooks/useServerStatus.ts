import { useEffect, useState, useCallback } from 'react';
import type { ServerStatus } from '../types/server';

const POLL_INTERVAL_MS = 5000;

export interface UseServerStatusResult {
  status: ServerStatus | null;
  loading: boolean;
  error: Error | null;
  refetch: () => Promise<void>;
}

/**
 * Custom hook that polls server status from /api/server/status
 * Polls every 5 seconds when the page is visible, pauses when hidden.
 *
 * @returns Object containing status data, loading state, error, and refetch function
 *
 * @example
 * ```tsx
 * const { status, loading, error, refetch } = useServerStatus();
 *
 * if (loading) return <div>Loading...</div>;
 * if (error) return <div>Error: {error.message}</div>;
 * if (!status) return <div>No status available</div>;
 *
 * return <div>Server is {status.status}</div>;
 * ```
 */
export function useServerStatus(): UseServerStatusResult {
  const [status, setStatus] = useState<ServerStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const fetchStatus = useCallback(async (isInitial = false) => {
    try {
      const response = await fetch('/api/server/status');
      if (!response.ok) {
        throw new Error(`Failed to fetch status: ${response.status} ${response.statusText}`);
      }
      const data: ServerStatus = await response.json();
      setStatus(data);
      setError(null);
    } catch (err) {
      setError(err as Error);
      // Keep last known status on error (don't clear it)
    } finally {
      if (isInitial) {
        setLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    let intervalId: number | undefined;
    let mounted = true;

    // Initial fetch
    fetchStatus(true);

    // Set up polling
    const startPolling = () => {
      if (intervalId) return;
      intervalId = window.setInterval(() => {
        if (mounted) {
          fetchStatus(false);
        }
      }, POLL_INTERVAL_MS);
    };

    const stopPolling = () => {
      if (intervalId) {
        window.clearInterval(intervalId);
        intervalId = undefined;
      }
    };

    // Handle visibility changes - pause polling when page is hidden
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        // Fetch immediately when becoming visible, then resume polling
        fetchStatus(false);
        startPolling();
      } else {
        stopPolling();
      }
    };

    // Start polling if page is visible
    if (document.visibilityState === 'visible') {
      startPolling();
    }

    document.addEventListener('visibilitychange', handleVisibilityChange);

    // Cleanup
    return () => {
      mounted = false;
      stopPolling();
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, [fetchStatus]);

  return { status, loading, error, refetch: () => fetchStatus(false) };
}
