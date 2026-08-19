import { useQuery } from '@tanstack/react-query';
import type { ProvisioningStatusResponse } from '../types/server';

const POLL_INTERVAL_MS = 2000;

async function fetchServerProvisioning(
  serverId: string
): Promise<ProvisioningStatusResponse> {
  const response = await fetch(`/api/servers/${serverId}/provisioning`);

  if (response.status === 404) {
    throw new Error('No provisioning in progress');
  }

  if (!response.ok) {
    throw new Error(
      `Failed to fetch provisioning status: ${response.status} ${response.statusText}`
    );
  }

  return response.json();
}

export function useServerProvisioning(serverId: string) {
  return useQuery({
    queryKey: ['serverProvisioning', serverId],
    queryFn: () => fetchServerProvisioning(serverId),
    refetchInterval: (query) => {
      if (query.state.error) return false;
      const data = query.state.data;
      if (data && (data.state === 'COMPLETED' || data.state === 'FAILED'))
        return false;
      return POLL_INTERVAL_MS;
    },
    refetchIntervalInBackground: false,
    staleTime: 0,
    retry: (failureCount, error) => {
      if (
        error instanceof Error &&
        error.message === 'No provisioning in progress'
      )
        return false;
      return failureCount < 2;
    },
    retryDelay: 2000,
    gcTime: 0,
  });
}
