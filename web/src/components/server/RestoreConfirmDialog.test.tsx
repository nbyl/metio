import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { RestoreConfirmDialog } from './RestoreConfirmDialog';
import { useRestoreBackup } from '../../hooks/useBackups';

const mockNavigate = vi.hoisted(() => vi.fn());

vi.mock('react-router-dom', () => ({
  useNavigate: () => mockNavigate,
}));

vi.mock('../../hooks/useBackups', () => ({
  useRestoreBackup: vi.fn(),
}));

const backup = {
  id: 'srv1:snap1',
  serverId: 'srv1',
  serverName: 'Survival',
  snapshotId: 'abc123def456',
  repositoryPrefix: 'servers/srv1/restic/',
  createdAt: '2026-08-20T10:00:00Z',
  durationSeconds: 42,
  fileCount: 1234,
  repositorySize: 567890,
  minecraftVersion: '1.21.4',
  status: 'COMPLETED' as const,
};

function renderDialog(
  overrides: Partial<React.ComponentProps<typeof RestoreConfirmDialog>> = {}
) {
  return render(
    <RestoreConfirmDialog
      open={true}
      backup={backup}
      serverId="srv1"
      serverName="Survival"
      currentMinecraftVersion="1.21.4"
      onClose={vi.fn()}
      {...overrides}
    />
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(useRestoreBackup).mockReturnValue({
    mutate: vi.fn(),
    isPending: false,
  } as never);
});

describe('RestoreConfirmDialog info step', () => {
  it('shows snapshot details', () => {
    renderDialog();

    expect(
      screen.getByRole('alertdialog', { name: /Restore Backup/ })
    ).toBeInTheDocument();
    expect(screen.getByText('1,234')).toBeInTheDocument();
    expect(screen.getByText('1.21.4')).toBeInTheDocument();
  });

  it('shows version mismatch warning when versions differ', () => {
    renderDialog({ currentMinecraftVersion: '1.21.2' });

    expect(screen.getByText('Minecraft Version Mismatch')).toBeInTheDocument();
    expect(screen.getByText(/Backup was created with/)).toBeInTheDocument();
  });

  it('hides version mismatch warning when versions match', () => {
    renderDialog({ currentMinecraftVersion: '1.21.4' });

    expect(
      screen.queryByText('Minecraft Version Mismatch')
    ).not.toBeInTheDocument();
  });

  it('shows the confirm step after clicking Continue', async () => {
    const user = userEvent.setup();
    renderDialog();

    await user.click(screen.getByText('Continue →'));

    expect(
      screen.getByText(/Are you sure you want to restore/)
    ).toBeInTheDocument();
  });
});

describe('RestoreConfirmDialog confirm step', () => {
  it('calls restore mutation and navigates on confirm', async () => {
    const user = userEvent.setup();
    const mockMutate = vi.fn((_id: string, opts?: { onSuccess?: () => void }) =>
      opts?.onSuccess?.()
    );
    vi.mocked(useRestoreBackup).mockReturnValue({
      mutate: mockMutate,
      isPending: false,
    } as never);

    renderDialog();

    await user.click(screen.getByText('Continue →'));
    await user.click(screen.getByRole('button', { name: 'Restore Backup' }));

    expect(mockMutate).toHaveBeenCalledWith('srv1:snap1', expect.anything());
    expect(mockNavigate).toHaveBeenCalledWith('/servers/srv1/provisioning', {
      state: { serverName: 'Survival' },
    });
  });

  it('goes back to info step when Back is clicked', async () => {
    const user = userEvent.setup();
    renderDialog();

    await user.click(screen.getByText('Continue →'));
    expect(
      screen.getByText(/Are you sure you want to restore/)
    ).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Back' }));
    expect(screen.getByText('1,234')).toBeInTheDocument();
  });
});
