import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useModrinthSearch } from './useModrinthSearch';
import type { ModrinthPack } from '../types/server';

const mockFetch = vi.fn();
global.fetch = mockFetch;

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
    },
  });
}

function createWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

const mockPacks: ModrinthPack[] = [
  {
    id: 'sodium',
    slug: 'sodium',
    name: 'Sodium',
    author: 'jellysquid',
    iconUrl: 'https://cdn.modrinth.com/sodium.png',
    downloads: 1_000_000,
    gameVersions: ['1.21.11'],
    loader: 'fabric',
  },
];

describe('useModrinthSearch', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = createQueryClient();
  });

  afterEach(() => {
    queryClient.clear();
  });

  it('fetches modpacks for the debounced query', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockPacks),
    });

    const { result } = renderHook(() => useModrinthSearch('sodium'), {
      wrapper: createWrapper(queryClient),
    });

    await waitFor(() => {
      expect(result.current.data).toEqual(mockPacks);
    });
    expect(mockFetch).toHaveBeenCalledWith('/api/modpacks/search?query=sodium');
  });

  it('does not fetch for queries shorter than three characters', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockPacks),
    });

    const { result } = renderHook(() => useModrinthSearch('ab'), {
      wrapper: createWrapper(queryClient),
    });

    // Give the debounce time to settle; the query stays disabled.
    await new Promise((resolve) => setTimeout(resolve, 450));

    expect(mockFetch).not.toHaveBeenCalled();
    expect(result.current.data).toBeUndefined();
  });

  it('exposes an error when the search request fails', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ error: 'boom' }),
    });

    const { result } = renderHook(() => useModrinthSearch('sodium'), {
      wrapper: createWrapper(queryClient),
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
    expect(result.current.error?.message).toBe('boom');
  });
});
