import { useQuery } from '@tanstack/react-query';
import type { APIError, ModrinthVersion } from '../types/server';

async function fetchModpackVersions(
  projectId: string
): Promise<ModrinthVersion[]> {
  const response = await fetch(`/api/modpacks/${projectId}/versions`);

  if (response.status === 401) {
    window.location.href = '/auth/login';
    throw new Error('Session expired');
  }

  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to load modpack versions');
  }

  return response.json();
}

/**
 * Lists the published versions of a modpack via
 * /api/modpacks/{id}/versions. Disabled until a projectId is provided.
 */
export function useModrinthPackVersions(projectId: string | null) {
  return useQuery({
    queryKey: ['modpackVersions', projectId],
    queryFn: () => fetchModpackVersions(projectId as string),
    enabled: Boolean(projectId),
    staleTime: 15 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
  });
}
