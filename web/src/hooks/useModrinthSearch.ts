import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import type { APIError, ModrinthPack } from '../types/server';

const SEARCH_DEBOUNCE_MS = 350;
const MIN_QUERY_LENGTH = 3;

async function searchModpacks(query: string): Promise<ModrinthPack[]> {
  const response = await fetch(
    `/api/modpacks/search?query=${encodeURIComponent(query)}`
  );

  if (response.status === 401) {
    window.location.href = '/auth/login';
    throw new Error('Session expired');
  }

  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to search modpacks');
  }

  return response.json();
}

/**
 * Debounces a raw value so upstream calls only happen after the user pauses.
 */
function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value);

  useEffect(() => {
    const timeout = setTimeout(() => setDebounced(value), delayMs);
    return () => clearTimeout(timeout);
  }, [value, delayMs]);

  return debounced;
}

/**
 * Searches the Modrinth modpack catalogue via the controller's
 * /api/modpacks/search endpoint. The query is debounced and results are fetched
 * only once the trimmed query is at least MIN_QUERY_LENGTH characters.
 */
export function useModrinthSearch(query: string) {
  const debouncedQuery = useDebouncedValue(query.trim(), SEARCH_DEBOUNCE_MS);

  const result = useQuery({
    queryKey: ['modpackSearch', debouncedQuery],
    queryFn: () => searchModpacks(debouncedQuery),
    enabled: debouncedQuery.length >= MIN_QUERY_LENGTH,
    staleTime: 15 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
  });

  // debouncedQuery lets callers distinguish "still settling" from "no results".
  return { ...result, debouncedQuery };
}
