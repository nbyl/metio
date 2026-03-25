import { useQuery } from '@tanstack/react-query';
import type { ServerStatus } from '../types/server';

const POLL_INTERVAL_MS = 5000;

/**
 * Fetches server status from the API
 */
async function fetchServerStatus(): Promise<ServerStatus> {
  const response = await fetch('/api/server/status');
  if (!response.ok) {
    throw new Error(
      `Failed to fetch status: ${response.status} ${response.statusText}`
    );
  }
  return response.json();
}

/**
 * Custom hook that polls server status from /api/server/status
 * Polls every 5 seconds when the page is visible, pauses when hidden.
 *
 * @returns React Query result with server status data
 *
 * @example
 * ```tsx
 * const { data: status, isLoading, error } = useServerStatus();
 *
 * if (isLoading) return <div>Loading...</div>;
 * if (error) return <div>Error: {error.message}</div>;
 * if (!status) return <div>No status available</div>;
 *
 * return <div>Server is {status.status}</div>;
 * ```
 */
export function useServerStatus() {
  return useQuery({
    queryKey: ['serverStatus'],
    queryFn: fetchServerStatus,
    refetchInterval: POLL_INTERVAL_MS,
    refetchIntervalInBackground: false, // Pause polling when tab is hidden
    staleTime: 0, // Always consider data stale for polling
  });
}
