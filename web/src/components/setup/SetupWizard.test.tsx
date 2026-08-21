import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SetupWizard } from './SetupWizard';
import { useSetupStatus } from '../../hooks/useSetupStatus';
import { useInitialize } from '../../hooks/useInitialize';

const navigateMock = vi.hoisted(() => vi.fn());

vi.mock('react-router-dom', () => ({
  useNavigate: () => navigateMock,
}));

vi.mock('../../hooks/useSetupStatus', () => ({
  useSetupStatus: vi.fn(),
}));

vi.mock('../../hooks/useInitialize', () => ({
  useInitialize: vi.fn(),
}));

const validStatus = {
  initialized: false,
  serverCount: 0,
  checks: {
    valid: true,
    apis: { compute: { enabled: true } },
    permissions: { admin: { granted: true } },
    fixes: [],
    checkedAt: '2026-01-01T00:00:00Z',
  },
};

function renderWizard() {
  return render(<SetupWizard />);
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(useSetupStatus).mockReturnValue({
    data: validStatus,
    isLoading: false,
  } as never);
  vi.mocked(useInitialize).mockReturnValue({
    mutate: vi.fn(),
    isPending: false,
    isError: false,
    isSuccess: false,
  } as never);
});

describe('SetupWizard', () => {
  it('renders the welcome step and advances to validation', async () => {
    const user = userEvent.setup();
    renderWizard();

    expect(screen.getByText('Welcome to Metio')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Next' }));
    expect(screen.getByText('GCP APIs')).toBeInTheDocument();
  });

  it('blocks validation progression when checks are invalid', async () => {
    const user = userEvent.setup();
    vi.mocked(useSetupStatus).mockReturnValue({
      data: { ...validStatus, checks: { ...validStatus.checks, valid: false } },
      isLoading: false,
    } as never);
    renderWizard();

    await user.click(screen.getByRole('button', { name: 'Next' }));
    await user.click(screen.getByRole('button', { name: 'Next' }));
    expect(screen.getByText('GCP APIs')).toBeInTheDocument();
    expect(
      screen.queryByText("We'll now create the Pulumi state bucket")
    ).not.toBeInTheDocument();
  });

  it('shows validation details and required fixes', async () => {
    const user = userEvent.setup();
    vi.mocked(useSetupStatus).mockReturnValue({
      data: {
        ...validStatus,
        checks: {
          ...validStatus.checks,
          valid: false,
          fixes: [
            {
              type: 'enable_api',
              api: 'compute.googleapis.com',
              consoleUrl: 'https://console.example.com',
            },
          ],
        },
      },
      isLoading: false,
    } as never);
    renderWizard();

    await user.click(screen.getByRole('button', { name: 'Next' }));
    expect(screen.getByText('Required fixes:')).toBeInTheDocument();
    expect(
      screen.getByText('Enable API: compute.googleapis.com')
    ).toBeInTheDocument();
  });

  it('gates initialization on a successful mutation and reaches completion', async () => {
    const user = userEvent.setup();
    const mutate = vi.fn();
    vi.mocked(useInitialize).mockReturnValue({
      mutate,
      isPending: false,
      isError: false,
      isSuccess: true,
    } as never);
    renderWizard();

    await user.click(screen.getByRole('button', { name: 'Next' }));
    await user.click(screen.getByRole('button', { name: 'Next' }));
    expect(
      screen.getByRole('button', { name: 'Create State Bucket' })
    ).toBeInTheDocument();
    await user.click(
      screen.getByRole('button', { name: 'Create State Bucket' })
    );
    expect(mutate).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole('button', { name: 'Finish' }));
    expect(screen.getByText('Setup Complete')).toBeInTheDocument();
  });

  it('shows initialization errors and retries', async () => {
    const user = userEvent.setup();
    const mutate = vi.fn();
    vi.mocked(useInitialize).mockReturnValue({
      mutate,
      isPending: false,
      isError: true,
      error: new Error('Initialization failed'),
      isSuccess: false,
    } as never);
    renderWizard();

    await user.click(screen.getByRole('button', { name: 'Next' }));
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await user.click(
      screen.getByRole('button', { name: 'Create State Bucket' })
    );
    expect(screen.getByText('Initialization failed')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(mutate).toHaveBeenCalledTimes(2);
  });
});
