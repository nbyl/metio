import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useStartServer, useStopServer } from './useServerMutations';
import type { StatusResponse } from '../types/server';

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const mockFetch = vi.fn();
global.fetch = mockFetch;

const TEST_SERVER_ID = 'srv1';

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

const mockStatus: StatusResponse = {
  serverState: 'STOPPED',
  players: { current: 0, max: 20 },
  uptime: '',
  version: '1.20.4',
  instanceIP: '',
};

const mockRunningStatus: StatusResponse = {
  serverState: 'RUNNING',
  players: { current: 5, max: 20 },
  uptime: '3h 45m',
  version: '1.20.4',
  instanceIP: '192.168.1.100',
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
      queryClient.setQueryData(['serverStatus', TEST_SERVER_ID], mockStatus);

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

      const { result } = renderHook(() => useStartServer(TEST_SERVER_ID), {
        wrapper: createWrapper(queryClient),
      });

      await act(async () => {
        result.current.mutate();
      });

      await waitFor(() => {
        const updatedStatus = queryClient.getQueryData<StatusResponse>(['serverStatus', TEST_SERVER_ID]);
        expect(updatedStatus?.serverState).toBe('STARTING');
      });
    });

    it('preserves other status fields during optimistic update', async () => {
      queryClient.setQueryData(['serverStatus', TEST_SERVER_ID], mockStatus);

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

      const { result } = renderHook(() => useStartServer(TEST_SERVER_ID), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.mutate();
      });

      const updatedStatus = queryClient.getQueryData<StatusResponse>(['serverStatus', TEST_SERVER_ID]);
      expect(updatedStatus?.version).toBe('1.20.4');
      expect(updatedStatus?.players.max).toBe(20);
    });

    it('rolls back to previous status on error', async () => {
      queryClient.setQueryData(['serverStatus', TEST_SERVER_ID], mockStatus);

      mockFetch.mockRejectedValue(new Error('Network error'));

      const { result } = renderHook(() => useStartServer(TEST_SERVER_ID), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.mutate();
      });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      const rolledBackStatus = queryClient.getQueryData<StatusResponse>(['serverStatus', TEST_SERVER_ID]);
      expect(rolledBackStatus?.serverState).toBe('STOPPED');
    });

    it('shows success toast on successful start', async () => {
      const { toast } = await import('sonner');
      queryClient.setQueryData(['serverStatus', TEST_SERVER_ID], mockStatus);

      mockFetch.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ success: true, state: 'STARTING' }),
      });

      const { result } = renderHook(() => useStartServer(TEST_SERVER_ID), {
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
      queryClient.setQueryData(['serverStatus', TEST_SERVER_ID], mockStatus);

      mockFetch.mockResolvedValue({
        ok: false,
        status: 500,
        json: () => Promise.resolve({ error: 'Server error' }),
      });

      const { result } = renderHook(() => useStartServer(TEST_SERVER_ID), {
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
      queryClient.setQueryData(['serverStatus', TEST_SERVER_ID], mockRunningStatus);

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

      const { result } = renderHook(() => useStopServer(TEST_SERVER_ID), {
        wrapper: createWrapper(queryClient),
      });

      await act(async () => {
        result.current.mutate();
      });

      await waitFor(() => {
        const updatedStatus = queryClient.getQueryData<StatusResponse>(['serverStatus', TEST_SERVER_ID]);
        expect(updatedStatus?.serverState).toBe('STOPPING');
      });
    });

    it('preserves other status fields during optimistic update', async () => {
      queryClient.setQueryData(['serverStatus', TEST_SERVER_ID], mockRunningStatus);

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

      const { result } = renderHook(() => useStopServer(TEST_SERVER_ID), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.mutate();
      });

      const updatedStatus = queryClient.getQueryData<StatusResponse>(['serverStatus', TEST_SERVER_ID]);
      expect(updatedStatus?.players.current).toBe(5);
      expect(updatedStatus?.instanceIP).toBe('192.168.1.100');
    });

    it('rolls back to previous status on error', async () => {
      queryClient.setQueryData(['serverStatus', TEST_SERVER_ID], mockRunningStatus);

      mockFetch.mockRejectedValue(new Error('Network error'));

      const { result } = renderHook(() => useStopServer(TEST_SERVER_ID), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        result.current.mutate();
      });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      const rolledBackStatus = queryClient.getQueryData<StatusResponse>(['serverStatus', TEST_SERVER_ID]);
      expect(rolledBackStatus?.serverState).toBe('RUNNING');
    });

    it('shows success toast on successful stop', async () => {
      const { toast } = await import('sonner');
      queryClient.setQueryData(['serverStatus', TEST_SERVER_ID], mockRunningStatus);

      mockFetch.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ success: true, state: 'STOPPING' }),
      });

      const { result } = renderHook(() => useStopServer(TEST_SERVER_ID), {
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
      queryClient.setQueryData(['serverStatus', TEST_SERVER_ID], mockRunningStatus);

      mockFetch.mockResolvedValue({
        ok: false,
        status: 500,
        json: () => Promise.resolve({ error: 'Server error' }),
      });

      const { result } = renderHook(() => useStopServer(TEST_SERVER_ID), {
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
