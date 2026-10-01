import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useModrinthPackVersions } from './useModrinthPackVersions';
import type { ModrinthVersion } from '../types/server';

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

const mockVersions: ModrinthVersion[] = [
  {
    id: 'v1',
    name: '1.0.0',
    versionNumber: '1.0.0',
    gameVersions: ['1.21.11'],
    loaders: ['fabric'],
    latest: true,
  },
];

describe('useModrinthPackVersions', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = createQueryClient();
  });

  afterEach(() => {
    queryClient.clear();
  });

  it('fetches versions when a projectId is provided', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockVersions),
    });

    const { result } = renderHook(() => useModrinthPackVersions('proj-1'), {
      wrapper: createWrapper(queryClient),
    });

    await waitFor(() => {
      expect(result.current.data).toEqual(mockVersions);
    });
    expect(mockFetch).toHaveBeenCalledWith('/api/modpacks/proj-1/versions');
  });

  it('stays disabled without a projectId', async () => {
    const { result } = renderHook(() => useModrinthPackVersions(null), {
      wrapper: createWrapper(queryClient),
    });

    await waitFor(() => {
      expect(result.current.isFetching).toBe(false);
    });
    expect(mockFetch).not.toHaveBeenCalled();
    expect(result.current.data).toBeUndefined();
  });

  it('exposes an error when the versions request fails', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ error: 'boom' }),
    });

    const { result } = renderHook(() => useModrinthPackVersions('proj-1'), {
      wrapper: createWrapper(queryClient),
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
    expect(result.current.error?.message).toBe('boom');
  });
});
