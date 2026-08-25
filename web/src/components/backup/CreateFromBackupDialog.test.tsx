import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CreateFromBackupDialog } from './CreateFromBackupDialog';
import { useCreateServerFromBackup } from '../../hooks/useBackups';
import { useServerOptions } from '../../hooks/useServerOptions';

const mockNavigate = vi.hoisted(() => vi.fn());

vi.mock('react-router-dom', () => ({
  useNavigate: () => mockNavigate,
}));

vi.mock('../../hooks/useBackups', () => ({
  useCreateServerFromBackup: vi.fn(),
}));

vi.mock('../../hooks/useServerOptions', () => ({
  useServerOptions: vi.fn(),
}));

const backup = {
  id: 'srv1:snap1',
  serverId: 'srv1',
  serverName: 'Survival',
  snapshotId: 'snap1',
  repositoryPrefix: 'servers/srv1/restic/',
  createdAt: '2026-08-20T10:00:00Z',
  durationSeconds: 42,
  fileCount: 1234,
  repositorySize: 567890,
  minecraftVersion: '1.21.4',
  status: 'COMPLETED' as const,
  sourceConfig: {
    region: 'europe-west1',
    zone: 'europe-west1-b',
    machineType: 'e2-medium',
    diskSizeGB: 10,
    minecraftVersion: '1.21.4',
  },
};

function renderDialog(
  overrides: Partial<React.ComponentProps<typeof CreateFromBackupDialog>> = {}
) {
  return render(
    <CreateFromBackupDialog
      open={true}
      backup={backup}
      onClose={vi.fn()}
      {...overrides}
    />
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(useCreateServerFromBackup).mockReturnValue({
    mutate: vi.fn(),
    isPending: false,
  } as never);
  vi.mocked(useServerOptions).mockReturnValue({
    data: {
      machineTypes: [
        { id: 'e2-medium', vcpus: 2, memoryGB: 4 },
        { id: 'e2-large', vcpus: 2, memoryGB: 8 },
      ],
      regions: [{ id: 'europe-west1', zones: ['europe-west1-b'] }],
      minecraftVersions: ['1.21.4', '1.21.3'],
    },
    isLoading: false,
  } as never);
});

describe('CreateFromBackupDialog', () => {
  it('renders the dialog title', () => {
    renderDialog();

    expect(screen.getByText('Create Server from Backup')).toBeInTheDocument();
  });

  it('shows the server name input', () => {
    renderDialog();

    expect(screen.getByLabelText('Server Name')).toBeInTheDocument();
  });

  it('shows infrastructure fields', () => {
    renderDialog();

    expect(screen.getByLabelText('Region')).toBeInTheDocument();
    expect(screen.getByLabelText('Zone')).toBeInTheDocument();
    expect(screen.getByLabelText('Machine Type')).toBeInTheDocument();
    expect(screen.getByLabelText('Minecraft Version')).toBeInTheDocument();
    expect(screen.getByLabelText('Disk Size (GB)')).toBeInTheDocument();
  });

  it('disables Create Server button when name is empty', () => {
    renderDialog();

    const createButton = screen.getByRole('button', { name: /Create Server/ });
    expect(createButton).toBeDisabled();
  });

  it('enables Create Server button when name is provided', async () => {
    const user = userEvent.setup();
    renderDialog();

    await user.type(screen.getByLabelText('Server Name'), 'My Server');

    const createButton = screen.getByRole('button', { name: /Create Server/ });
    expect(createButton).not.toBeDisabled();
  });

  it('calls the mutation and navigates on successful creation', async () => {
    const user = userEvent.setup();
    const mockMutate = vi.fn(
      (_req: unknown, opts?: { onSuccess?: (data: { id: string }) => void }) =>
        opts?.onSuccess?.({ id: 'new-server-id' })
    );
    vi.mocked(useCreateServerFromBackup).mockReturnValue({
      mutate: mockMutate,
      isPending: false,
    } as never);

    renderDialog();

    await user.type(screen.getByLabelText('Server Name'), 'My Server');
    fireEvent.submit(screen.getByRole('button', { name: /Create Server/ }));

    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'My Server' }),
      expect.anything()
    );
    expect(mockNavigate).toHaveBeenCalledWith(
      '/servers/new-server-id/provisioning',
      { state: { serverName: 'My Server' } }
    );
  });

  it('shows loading state while creating', () => {
    vi.mocked(useCreateServerFromBackup).mockReturnValue({
      mutate: vi.fn(),
      isPending: true,
    } as never);

    renderDialog();

    const createButton = screen.getByRole('button', { name: /Create Server/ });
    expect(createButton).toBeDisabled();
    expect(createButton.querySelector('.animate-spin')).not.toBeNull();
  });
});
