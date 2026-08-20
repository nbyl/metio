import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ServerSetupWizard } from './ServerSetupWizard';
import { useServerOptions } from '../../hooks/useServerOptions';
import { useCreateServer } from '../../hooks/useServerMutations';

const navigateMock = vi.hoisted(() => vi.fn());

vi.mock('react-router-dom', () => ({
  useNavigate: () => navigateMock,
}));

vi.mock('../../hooks/useServerOptions', () => ({
  useServerOptions: vi.fn(),
}));

vi.mock('../../hooks/useServerMutations', () => ({
  useCreateServer: vi.fn(),
}));

const options = {
  regions: [
    { id: 'us-central1', zones: ['us-central1-a', 'us-central1-b'] },
    { id: 'europe-west3', zones: ['europe-west3-a'] },
  ],
  machineTypes: [
    { id: 'e2-small', vcpus: 2, memoryGB: 2 },
    { id: 'n2-standard-2', vcpus: 2, memoryGB: 8 },
    { id: 'c2-standard-4', vcpus: 4, memoryGB: 16 },
  ],
  minecraftVersions: ['1.21.11', '1.21.10'],
};

function renderWizard() {
  return render(<ServerSetupWizard />);
}

async function completeBasicInfo(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText('Server Name'), 'survival');
  await user.click(screen.getByRole('combobox', { name: 'Region' }));
  await user.click(screen.getByRole('option', { name: 'us-central1' }));
  await user.click(screen.getByRole('combobox', { name: 'Zone' }));
  await user.click(screen.getByRole('option', { name: 'us-central1-a' }));
}

async function completeSpecs(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: /e2-small/ }));
  await user.click(screen.getByRole('combobox', { name: 'Minecraft Version' }));
  await user.click(screen.getByRole('option', { name: '1.21.11' }));
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(useServerOptions).mockReturnValue({
    data: options,
    isLoading: false,
  } as never);
  vi.mocked(useCreateServer).mockReturnValue({
    mutate: vi.fn(),
    isPending: false,
  } as never);
});

describe('ServerSetupWizard', () => {
  it('renders the first step and blocks invalid progression', async () => {
    const user = userEvent.setup();
    renderWizard();

    expect(
      screen.getByRole('heading', { name: 'Create New Server' })
    ).toBeInTheDocument();
    expect(screen.getByText('Basic Info')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Next' }));

    expect(screen.getByText('Server name is required')).toBeInTheDocument();
    expect(screen.getByText('Region is required')).toBeInTheDocument();
    expect(screen.getByText('Zone is required')).toBeInTheDocument();
    expect(screen.getByText('Server Specs')).toBeInTheDocument();
  });

  it('validates the server name format', async () => {
    const user = userEvent.setup();
    renderWizard();

    await user.type(screen.getByLabelText('Server Name'), 'Bad Name');
    await user.click(screen.getByRole('button', { name: 'Next' }));

    expect(
      screen.getByText(
        'Name must start with a letter and contain only lowercase letters, digits, and hyphens'
      )
    ).toBeInTheDocument();
  });

  it('advances through Basic Info and Specs only after valid fields', async () => {
    const user = userEvent.setup();
    renderWizard();

    await completeBasicInfo(user);
    await user.click(screen.getByRole('button', { name: 'Next' }));
    expect(screen.getByText('Server Specs')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Next' }));
    expect(screen.getByText('Machine type is required')).toBeInTheDocument();
    expect(
      screen.getByText('Minecraft version is required')
    ).toBeInTheDocument();

    await completeSpecs(user);
    await user.click(screen.getByRole('button', { name: 'Next' }));
    expect(screen.getByText('Scheduled Shutdown')).toBeInTheDocument();
  });

  it('supports showing all machine types and selecting a machine', async () => {
    const user = userEvent.setup();
    renderWizard();
    await completeBasicInfo(user);
    await user.click(screen.getByRole('button', { name: 'Next' }));

    expect(
      screen.queryByRole('button', { name: /c2-standard-4/ })
    ).not.toBeInTheDocument();
    await user.click(
      screen.getByRole('button', { name: /Show all machine types/ })
    );
    expect(
      screen.getByRole('button', { name: /c2-standard-4/ })
    ).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /c2-standard-4/ }));
    expect(
      screen.getByRole('button', { name: /c2-standard-4/ })
    ).toHaveAttribute('aria-pressed', 'true');
  });

  it('preserves scheduled shutdown values and reaches the review step', async () => {
    const user = userEvent.setup();
    renderWizard();
    await completeBasicInfo(user);
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await completeSpecs(user);
    await user.click(screen.getByRole('button', { name: 'Next' }));

    await user.click(
      screen.getByRole('switch', { name: 'Toggle scheduled shutdown' })
    );
    await user.click(screen.getByRole('combobox', { name: 'Shutdown Time' }));
    await user.click(screen.getByRole('option', { name: '22:00' }));
    await user.click(screen.getByRole('combobox', { name: 'Timezone' }));
    await user.click(screen.getByRole('option', { name: 'UTC' }));
    await user.click(screen.getByRole('button', { name: 'Next' }));

    expect(screen.getByText('Review')).toBeInTheDocument();
    expect(screen.getByText('22:00 UTC')).toBeInTheDocument();
  });

  it('submits the validated form payload and navigates after creation', async () => {
    const user = userEvent.setup();
    const mutate = vi.fn((_payload, callbacks) => {
      callbacks?.onSuccess?.({ id: 'created-server' });
    });
    vi.mocked(useCreateServer).mockReturnValue({
      mutate,
      isPending: false,
    } as never);
    renderWizard();

    await completeBasicInfo(user);
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await completeSpecs(user);
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await user.click(screen.getByRole('button', { name: 'Create Server' }));

    expect(mutate).toHaveBeenCalledWith(
      {
        name: 'survival',
        region: 'us-central1',
        zone: 'us-central1-a',
        machineType: 'e2-small',
        minecraftVersion: '1.21.11',
        diskSizeGB: 20,
      },
      expect.any(Object)
    );
    expect(navigateMock).toHaveBeenCalledWith(
      '/servers/created-server/provisioning',
      {
        state: { serverName: 'survival' },
      }
    );
  });

  it('renders loading and missing-options states', () => {
    vi.mocked(useServerOptions).mockReturnValue({
      data: undefined,
      isLoading: true,
    } as never);
    const { rerender } = renderWizard();
    expect(document.querySelector('.animate-pulse')).toBeInTheDocument();

    vi.mocked(useServerOptions).mockReturnValue({
      data: undefined,
      isLoading: false,
    } as never);
    rerender(<ServerSetupWizard />);
    expect(
      screen.getByText('Failed to load server options')
    ).toBeInTheDocument();
  });
});
