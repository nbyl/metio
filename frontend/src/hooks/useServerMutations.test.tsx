import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useStartServer, useStopServer } from './useServerMutations';
import type { ServerStatus } from '../types/server';

// Mock sonner toast
vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

// Mock fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
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

const mockStatus: ServerStatus = {
  status: 'STOPPED',
  players: 0,
  maxPlayers: 20,
  uptime: '',
  version: '1.20.4',
  ip: '',
};

const mockRunningStatus: ServerStatus = {
  status: 'RUNNING',
  players: 5,
  maxPlayers: 20,
  uptime: '3h 45m',
  version: '1.20.4',
  ip: '192.168.1.100',
};

describe('useStartServer', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = createQueryClient();
  });

  afterEach(() => {
    queryClient.clear();
  });

  describe('optimistic updates', () => {
    it('immediately updates status to STARTING on mutate', async () => {
      // Set initial status
      queryClient.setQueryData(['serverStatus'], mockStatus);

      // Mock a slow API response
      mockFetch.mockImplementation(
        () =>
          new Promise((resolve) =>
            setTimeout(
              () =>
                resolve({
                  ok: true,
                  json: () => Promise.resolve({ success: true, state: 'STARTING' }),
                }),
              100
            )
          )
      );

      const { result } = renderHook(() => useStartServer(), {
        wrapper: createWrapper(queryClient),
      });

      // Trigger the mutation
      await act(async () => {
        result.current.mutate();
      });

      // Check that status was immediately updated to STARTING (before API response)
      await waitFor(() => {
        const updatedStatus = queryClient.getQueryData<ServerStatus>(['serverStatus']);
        expect(updatedStatus?.status).toBe('STARTING');
      });
    });

    it('preserves other status fields during optimistic update', async () => {
      queryClient.setQueryData(['serverStatus'], mockStatus);

      mockFetch.mockImplementation(
        () =>
          new Promise((resolve) =>
            setTimeout(
              () =>
                resolve({
                  ok: true,
                  json: () => Promise.resolve({ success: true, state: 'STARTING' }),
                }),
              100
            )
          )
      );

      const { result } = renderHook(() => useStartServer(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.mutate();
      });

      const updatedStatus = queryClient.getQueryData<ServerStatus>(['serverStatus']);
      expect(updatedStatus?.version).toBe('1.20.4');
      expect(updatedStatus?.maxPlayers).toBe(20);
    });

    it('rolls back to previous status on error', async () => {
      queryClient.setQueryData(['serverStatus'], mockStatus);

      mockFetch.mockRejectedValue(new Error('Network error'));

      const { result } = renderHook(() => useStartServer(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.mutate();
      });

      // Wait for error handling
      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      // Status should be rolled back to STOPPED
      const rolledBackStatus = queryClient.getQueryData<ServerStatus>(['serverStatus']);
      expect(rolledBackStatus?.status).toBe('STOPPED');
    });

    it('shows success toast on successful start', async () => {
      const { toast } = await import('sonner');
      queryClient.setQueryData(['serverStatus'], mockStatus);

      mockFetch.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ success: true, state: 'STARTING' }),
      });

      const { result } = renderHook(() => useStartServer(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.mutate();
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(toast.success).toHaveBeenCalledWith('Server starting...');
    });

    it('shows error toast on failed start', async () => {
      const { toast } = await import('sonner');
      queryClient.setQueryData(['serverStatus'], mockStatus);

      mockFetch.mockResolvedValue({
        ok: false,
        status: 500,
        json: () => Promise.resolve({ error: 'Server error' }),
      });

      const { result } = renderHook(() => useStartServer(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.mutate();
      });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(toast.error).toHaveBeenCalledWith('Server error');
    });
  });
});

describe('useStopServer', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = createQueryClient();
  });

  afterEach(() => {
    queryClient.clear();
  });

  describe('optimistic updates', () => {
    it('immediately updates status to STOPPING on mutate', async () => {
      queryClient.setQueryData(['serverStatus'], mockRunningStatus);

      mockFetch.mockImplementation(
        () =>
          new Promise((resolve) =>
            setTimeout(
              () =>
                resolve({
                  ok: true,
                  json: () => Promise.resolve({ success: true, state: 'STOPPING' }),
                }),
              100
            )
          )
      );

      const { result } = renderHook(() => useStopServer(), {
        wrapper: createWrapper(queryClient),
      });

      await act(async () => {
        result.current.mutate();
      });

      await waitFor(() => {
        const updatedStatus = queryClient.getQueryData<ServerStatus>(['serverStatus']);
        expect(updatedStatus?.status).toBe('STOPPING');
      });
    });

    it('preserves other status fields during optimistic update', async () => {
      queryClient.setQueryData(['serverStatus'], mockRunningStatus);

      mockFetch.mockImplementation(
        () =>
          new Promise((resolve) =>
            setTimeout(
              () =>
                resolve({
                  ok: true,
                  json: () => Promise.resolve({ success: true, state: 'STOPPING' }),
                }),
              100
            )
          )
      );

      const { result } = renderHook(() => useStopServer(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.mutate();
      });

      const updatedStatus = queryClient.getQueryData<ServerStatus>(['serverStatus']);
      expect(updatedStatus?.players).toBe(5);
      expect(updatedStatus?.ip).toBe('192.168.1.100');
    });

    it('rolls back to previous status on error', async () => {
      queryClient.setQueryData(['serverStatus'], mockRunningStatus);

      mockFetch.mockRejectedValue(new Error('Network error'));

      const { result } = renderHook(() => useStopServer(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.mutate();
      });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      const rolledBackStatus = queryClient.getQueryData<ServerStatus>(['serverStatus']);
      expect(rolledBackStatus?.status).toBe('RUNNING');
    });

    it('shows success toast on successful stop', async () => {
      const { toast } = await import('sonner');
      queryClient.setQueryData(['serverStatus'], mockRunningStatus);

      mockFetch.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ success: true, state: 'STOPPING' }),
      });

      const { result } = renderHook(() => useStopServer(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.mutate();
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(toast.success).toHaveBeenCalledWith('Server stopping...');
    });

    it('shows error toast on failed stop', async () => {
      const { toast } = await import('sonner');
      queryClient.setQueryData(['serverStatus'], mockRunningStatus);

      mockFetch.mockResolvedValue({
        ok: false,
        status: 500,
        json: () => Promise.resolve({ error: 'Server error' }),
      });

      const { result } = renderHook(() => useStopServer(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.mutate();
      });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(toast.error).toHaveBeenCalledWith('Server error');
    });
  });
});
