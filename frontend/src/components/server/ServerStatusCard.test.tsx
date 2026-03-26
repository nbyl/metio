import { describe, it, expect, vi, beforeEach, type Mock } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router-dom';
import { ServerStatusCard } from './ServerStatusCard';
import { useServerStatus } from '../../hooks/useServerStatus';
import {
  useStartServer,
  useStopServer,
} from '../../hooks/useServerMutations';
import type { ServerStatus, ServerState } from '../../types/server';

// Mock the hooks
vi.mock('../../hooks/useServerStatus');
vi.mock('../../hooks/useServerMutations');

// Mock clipboard API
const mockClipboard = {
  writeText: vi.fn(),
};
Object.assign(navigator, { clipboard: mockClipboard });

// Mock sonner toast
vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const mockUseServerStatus = useServerStatus as Mock;
const mockUseStartServer = useStartServer as Mock;
const mockUseStopServer = useStopServer as Mock;

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>{children}</BrowserRouter>
      </QueryClientProvider>
    );
  };
}

const mockStatus: ServerStatus = {
  status: 'RUNNING',
  players: 5,
  maxPlayers: 20,
  uptime: '3h 45m',
  version: '1.20.4',
  ip: '192.168.1.100',
};

describe('ServerStatusCard', () => {
  const mockRefetch = vi.fn();
  const mockStartMutate = vi.fn();
  const mockStopMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();

    // Default mock implementations
    mockUseStartServer.mockReturnValue({
      mutate: mockStartMutate,
      isPending: false,
    });
    mockUseStopServer.mockReturnValue({
      mutate: mockStopMutate,
      isPending: false,
    });
  });

  describe('Loading State', () => {
    beforeEach(() => {
      mockUseServerStatus.mockReturnValue({
        data: undefined,
        isLoading: true,
        error: null,
        refetch: mockRefetch,
      });
    });

    it('renders loading skeleton', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      // Should show skeleton elements (they have animate-pulse class)
      const skeletons = document.querySelectorAll('.skeleton');
      expect(skeletons.length).toBeGreaterThan(0);
    });

    it('loading skeleton has no buttons', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });

    it('matches loading snapshot', () => {
      const { container } = render(<ServerStatusCard />, {
        wrapper: createWrapper(),
      });
      expect(container.firstChild).toMatchSnapshot();
    });
  });

  describe('Error State', () => {
    beforeEach(() => {
      mockUseServerStatus.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error('Connection failed'),
        refetch: mockRefetch,
      });
    });

    it('renders error state with message', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      expect(screen.getByText('Error: Connection failed')).toBeInTheDocument();
    });

    it('renders retry button', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
    });

    it('retry button calls refetch', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
      expect(mockRefetch).toHaveBeenCalled();
    });

    it('matches error snapshot', () => {
      const { container } = render(<ServerStatusCard />, {
        wrapper: createWrapper(),
      });
      expect(container.firstChild).toMatchSnapshot();
    });
  });

  describe('No Status State', () => {
    beforeEach(() => {
      mockUseServerStatus.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });
    });

    it('renders no status message', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      expect(
        screen.getByText('No server status available')
      ).toBeInTheDocument();
    });

    it('renders start button', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      expect(
        screen.getByRole('button', { name: 'Start Server' })
      ).toBeInTheDocument();
    });

    it('start button triggers mutation', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      fireEvent.click(screen.getByRole('button', { name: 'Start Server' }));
      expect(mockStartMutate).toHaveBeenCalled();
    });
  });

  describe('Stopped State', () => {
    beforeEach(() => {
      mockUseServerStatus.mockReturnValue({
        data: { ...mockStatus, status: 'STOPPED' },
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });
    });

    it('renders without stats grid', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      // Stats should not be visible
      expect(screen.queryByText('Version')).not.toBeInTheDocument();
      expect(screen.queryByText('Players')).not.toBeInTheDocument();
      expect(screen.queryByText('Uptime')).not.toBeInTheDocument();
    });

    it('renders offline badge', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      expect(screen.getByText('Offline')).toBeInTheDocument();
    });

    it('renders start button', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      expect(
        screen.getByRole('button', { name: 'Start Server' })
      ).toBeInTheDocument();
    });

    it('does not render copy IP button', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      expect(screen.queryByRole('button', { name: /Copy IP/i })).not.toBeInTheDocument();
    });

    it('matches stopped snapshot', () => {
      const { container } = render(<ServerStatusCard />, {
        wrapper: createWrapper(),
      });
      expect(container.firstChild).toMatchSnapshot();
    });
  });

  describe('Running State', () => {
    beforeEach(() => {
      mockUseServerStatus.mockReturnValue({
        data: mockStatus,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });
    });

    it('renders online badge', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      expect(screen.getByText('Online')).toBeInTheDocument();
    });

    it('renders stats grid with version', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      expect(screen.getByText('Version')).toBeInTheDocument();
      expect(screen.getByText('1.20.4')).toBeInTheDocument();
    });

    it('renders stats grid with players', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      expect(screen.getByText('Players')).toBeInTheDocument();
      expect(screen.getByText('5/20')).toBeInTheDocument();
    });

    it('renders stats grid with uptime', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      expect(screen.getByText('Uptime')).toBeInTheDocument();
      expect(screen.getByText('3h 45m')).toBeInTheDocument();
    });

    it('renders stats grid with IP', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      expect(screen.getByText('IP')).toBeInTheDocument();
      expect(screen.getByText('192.168.1.100')).toBeInTheDocument();
    });

    it('renders stop button', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      expect(
        screen.getByRole('button', { name: 'Stop Server' })
      ).toBeInTheDocument();
    });

    it('stop button triggers mutation', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      fireEvent.click(screen.getByRole('button', { name: 'Stop Server' }));
      expect(mockStopMutate).toHaveBeenCalled();
    });

    it('renders copy IP button', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      expect(
        screen.getByRole('button', { name: /Copy IP/i })
      ).toBeInTheDocument();
    });

    it('copy IP button copies to clipboard', async () => {
      mockClipboard.writeText.mockResolvedValue(undefined);

      render(<ServerStatusCard />, { wrapper: createWrapper() });

      fireEvent.click(screen.getByRole('button', { name: /Copy IP/i }));

      await waitFor(() => {
        expect(mockClipboard.writeText).toHaveBeenCalledWith('192.168.1.100');
      });
    });

    it('copy IP button shows visual feedback after copying', async () => {
      mockClipboard.writeText.mockResolvedValue(undefined);

      render(<ServerStatusCard />, { wrapper: createWrapper() });

      // Initially shows "Copy IP"
      expect(screen.getByRole('button', { name: /Copy IP/i })).toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', { name: /Copy IP/i }));

      // After click, should show "Copied!"
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /Copied!/i })).toBeInTheDocument();
      });
    });

    it('shows success toast when copy succeeds', async () => {
      const { toast } = await import('sonner');
      mockClipboard.writeText.mockResolvedValue(undefined);

      render(<ServerStatusCard />, { wrapper: createWrapper() });

      fireEvent.click(screen.getByRole('button', { name: /Copy IP/i }));

      await waitFor(() => {
        expect(toast.success).toHaveBeenCalledWith('IP copied to clipboard!');
      });
    });

    it('matches running snapshot', () => {
      const { container } = render(<ServerStatusCard />, {
        wrapper: createWrapper(),
      });
      expect(container.firstChild).toMatchSnapshot();
    });
  });

  describe('Starting State', () => {
    beforeEach(() => {
      mockUseServerStatus.mockReturnValue({
        data: { ...mockStatus, status: 'STARTING' },
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });
    });

    it('renders transitioning badge', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      const badge = document.querySelector('.badge-transitioning');
      expect(badge).toBeInTheDocument();
      expect(badge).toHaveTextContent('Starting...');
    });

    it('renders stats grid', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      expect(screen.getByText('Version')).toBeInTheDocument();
    });

    it('renders disabled starting button', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      const button = screen.getByRole('button', { name: 'Starting...' });
      expect(button).toBeDisabled();
    });

    it('does not render copy IP button', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      expect(screen.queryByRole('button', { name: /Copy IP/i })).not.toBeInTheDocument();
    });
  });

  describe('Stopping State', () => {
    beforeEach(() => {
      mockUseServerStatus.mockReturnValue({
        data: { ...mockStatus, status: 'STOPPING' },
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });
    });

    it('renders transitioning badge', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      const badge = document.querySelector('.badge-transitioning');
      expect(badge).toBeInTheDocument();
      expect(badge).toHaveTextContent('Stopping...');
    });

    it('renders disabled stopping button', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      const button = screen.getByRole('button', { name: 'Stopping...' });
      expect(button).toBeDisabled();
    });

    it('does not render copy IP button', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      expect(screen.queryByRole('button', { name: /Copy IP/i })).not.toBeInTheDocument();
    });
  });

  describe('Custom className', () => {
    beforeEach(() => {
      mockUseServerStatus.mockReturnValue({
        data: mockStatus,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });
    });

    it('accepts custom className', () => {
      const { container } = render(<ServerStatusCard className="custom-class" />, {
        wrapper: createWrapper(),
      });

      expect(container.firstChild).toHaveClass('custom-class');
    });
  });

  describe('Copy IP Error Handling', () => {
    beforeEach(() => {
      mockUseServerStatus.mockReturnValue({
        data: mockStatus,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });
    });

    it('shows error toast when clipboard fails', async () => {
      const { toast } = await import('sonner');
      mockClipboard.writeText.mockRejectedValue(new Error('Clipboard error'));

      render(<ServerStatusCard />, { wrapper: createWrapper() });

      fireEvent.click(screen.getByRole('button', { name: /Copy IP/i }));

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith('Failed to copy IP');
      });
    });
  });

  describe('Unknown Server State', () => {
    beforeEach(() => {
      mockUseServerStatus.mockReturnValue({
        data: { ...mockStatus, status: 'UNKNOWN' as ServerState },
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });
    });

    it('renders offline badge for unknown state', () => {
      render(<ServerStatusCard />, { wrapper: createWrapper() });

      const badge = document.querySelector('.badge-offline');
      expect(badge).toBeInTheDocument();
      expect(badge).toHaveTextContent('Unknown');
    });
  });
});
