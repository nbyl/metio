import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ProvisioningProgress } from './ProvisioningProgress';
import { useServerProvisioning } from '../../hooks/useServerProvisioning';

const navigateMock = vi.hoisted(() => vi.fn());

vi.mock('react-router-dom', () => ({
  useNavigate: () => navigateMock,
  useLocation: () => ({ state: { serverName: 'Survival' } }),
}));

vi.mock('../../hooks/useServerProvisioning', () => ({
  useServerProvisioning: vi.fn(),
}));

const steps = [
  {
    name: 'prepare',
    status: 'COMPLETED',
    message: 'Preparing infrastructure',
    timestamp: '2026-01-01T00:00:00Z',
  },
  {
    name: 'deploy',
    status: 'IN_PROGRESS',
    message: 'Deploying server',
    timestamp: '2026-01-01T00:00:00Z',
  },
];

const data = {
  id: 'srv1',
  operation: 'CREATE',
  state: 'IN_PROGRESS',
  currentStep: 'deploy',
  progress: 42,
  steps,
  startedAt: '2026-01-01T00:00:00Z',
};

function renderProgress() {
  return render(<ProvisioningProgress serverId="srv1" />);
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(useServerProvisioning).mockReturnValue({
    data,
    isLoading: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
  } as never);
});

describe('ProvisioningProgress', () => {
  it('renders the loading skeleton', () => {
    vi.mocked(useServerProvisioning).mockReturnValue({
      data: undefined,
      isLoading: true,
    } as never);

    renderProgress();
    expect(document.querySelector('.animate-pulse')).toBeInTheDocument();
  });

  it('renders steps, operation details, elapsed time, and Progress', () => {
    renderProgress();

    expect(screen.getByText('Creating Server:')).toBeInTheDocument();
    expect(screen.getByText('Survival')).toBeInTheDocument();
    expect(screen.getByText('Preparing infrastructure')).toBeInTheDocument();
    expect(screen.getByText('Deploying server')).toBeInTheDocument();
    expect(screen.getByText('Done')).toBeInTheDocument();
    expect(screen.getByText('In Progress')).toBeInTheDocument();
    expect(screen.getByRole('progressbar')).toHaveAttribute(
      'aria-valuenow',
      '42'
    );
  });

  it('renders completed and failed states', () => {
    vi.mocked(useServerProvisioning).mockReturnValue({
      data: { ...data, state: 'COMPLETED', progress: 100 },
      isLoading: false,
      isError: false,
    } as never);
    const { rerender } = renderProgress();
    expect(screen.getByText('Completed')).toBeInTheDocument();

    vi.mocked(useServerProvisioning).mockReturnValue({
      data: { ...data, state: 'FAILED', error: 'Deployment failed' },
      isLoading: false,
      isError: false,
    } as never);
    rerender(<ProvisioningProgress serverId="srv1" />);
    expect(screen.getByText('Failed')).toBeInTheDocument();
    expect(screen.getByText('Deployment failed')).toBeInTheDocument();
  });

  it('renders the no-provisioning state and navigates home', async () => {
    const user = userEvent.setup();
    vi.mocked(useServerProvisioning).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error('No provisioning in progress'),
      refetch: vi.fn(),
    } as never);

    renderProgress();
    await user.click(screen.getByRole('button', { name: /Back to Dashboard/ }));
    expect(navigateMock).toHaveBeenCalledWith('/');
  });

  it('renders a retry action for general errors', async () => {
    const user = userEvent.setup();
    const refetch = vi.fn();
    vi.mocked(useServerProvisioning).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error('Network failure'),
      refetch,
    } as never);

    renderProgress();
    expect(screen.getByText('Network failure')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(refetch).toHaveBeenCalledTimes(1);
  });
});
