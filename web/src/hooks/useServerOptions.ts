import { useQuery } from '@tanstack/react-query';
import type { SetupOptions, APIError } from '../types/server';

async function fetchOptions(): Promise<SetupOptions> {
  const response = await fetch('/api/options');

  if (response.status === 401) {
    window.location.href = '/auth/login';
    throw new Error('Session expired');
  }

  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to load server options');
  }

  return response.json();
}

export function useServerOptions() {
  return useQuery({
    queryKey: ['serverOptions'],
    queryFn: fetchOptions,
    staleTime: Infinity,
    gcTime: 30 * 60 * 1000,
  });
}
