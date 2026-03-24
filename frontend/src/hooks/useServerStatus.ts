import { useEffect, useState } from 'react';
import { doc, onSnapshot, type Unsubscribe } from 'firebase/firestore';
import { initializeFirebase } from '../lib/firebase';
import type { ServerStatus } from '../types/firestore';

export interface UseServerStatusResult {
  status: ServerStatus | null;
  loading: boolean;
  error: Error | null;
}

/**
 * Custom hook that subscribes to real-time server status updates from Firestore
 *
 * @returns Object containing status data, loading state, and any error
 *
 * @example
 * ```tsx
 * const { status, loading, error } = useServerStatus();
 *
 * if (loading) return <div>Loading...</div>;
 * if (error) return <div>Error: {error.message}</div>;
 * if (!status) return <div>No status available</div>;
 *
 * return <div>Server is {status.server_state}</div>;
 * ```
 */
export function useServerStatus(): UseServerStatusResult {
  const [status, setStatus] = useState<ServerStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    let unsubscribe: Unsubscribe | undefined;
    let mounted = true;

    async function subscribe() {
      try {
        const { db, config } = await initializeFirebase();

        // Subscribe to the status document
        // Path: instances/{instanceName}/data/status
        const docRef = doc(
          db,
          'instances',
          config.instanceName,
          'data',
          'status'
        );

        unsubscribe = onSnapshot(
          docRef,
          (snapshot) => {
            if (!mounted) return;

            if (snapshot.exists()) {
              setStatus(snapshot.data() as ServerStatus);
            } else {
              setStatus(null);
            }
            setLoading(false);
          },
          (err) => {
            if (!mounted) return;
            console.error('Firestore subscription error:', err);
            setError(err);
            setLoading(false);
          }
        );
      } catch (err) {
        if (!mounted) return;
        console.error('Firebase initialization error:', err);
        setError(err as Error);
        setLoading(false);
      }
    }

    subscribe();

    // Cleanup subscription on unmount
    return () => {
      mounted = false;
      if (unsubscribe) {
        unsubscribe();
      }
    };
  }, []);

  return { status, loading, error };
}
