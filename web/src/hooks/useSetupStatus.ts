import { useQuery } from '@tanstack/react-query';
import type { SetupStatus } from '../types/setup';

async function fetchSetupStatus(): Promise<SetupStatus> {
  const response = await fetch('/api/setup/status');

  if (response.status === 401) {
    window.location.href = '/auth/login';
    throw new Error('Session expired');
  }

  if (!response.ok) {
    throw new Error(`Failed to fetch setup status: ${response.status}`);
  }

  return response.json();
}

export function useSetupStatus() {
  return useQuery({
    queryKey: ['setup', 'status'],
    queryFn: fetchSetupStatus,
    staleTime: 0,
  });
}
