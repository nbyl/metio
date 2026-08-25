import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { BackupListPanel } from './BackupListPanel';
import { useServerBackups, useRestoreBackup } from '../../hooks/useBackups';

vi.mock('react-router-dom', () => ({
  useNavigate: () => vi.fn(),
}));

vi.mock('../../hooks/useBackups', () => ({
  useServerBackups: vi.fn(),
  useRestoreBackup: vi.fn(),
}));

const backups = [
  {
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
  },
  {
    id: 'srv1:snap2',
    serverId: 'srv1',
    serverName: 'Survival',
    snapshotId: 'snap2',
    repositoryPrefix: 'servers/srv1/restic/',
    createdAt: '2026-08-19T10:00:00Z',
    durationSeconds: 30,
    fileCount: 1100,
    repositorySize: 400000,
    minecraftVersion: '1.21.4',
    status: 'FAILED' as const,
  },
];

function renderPanel(
  props: Partial<React.ComponentProps<typeof BackupListPanel>> = {}
) {
  return render(
    <BackupListPanel
      serverId="srv1"
      serverName="Survival"
      minecraftVersion="1.21.4"
      {...props}
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

describe('BackupListPanel loading', () => {
  it('shows a spinner while backups are loading', () => {
    vi.mocked(useServerBackups).mockReturnValue({
      data: undefined,
      isLoading: true,
    } as never);

    renderPanel();

    expect(document.querySelector('.animate-spin')).not.toBeNull();
  });
});

describe('BackupListPanel empty', () => {
  it('shows an empty message when there are no backups', () => {
    vi.mocked(useServerBackups).mockReturnValue({
      data: [],
      isLoading: false,
    } as never);

    renderPanel();

    expect(screen.getByText(/No backups yet/)).toBeInTheDocument();
  });
});

describe('BackupListPanel table', () => {
  beforeEach(() => {
    vi.mocked(useServerBackups).mockReturnValue({
      data: backups,
      isLoading: false,
    } as never);
  });

  it('renders the backup table headers', () => {
    renderPanel();

    expect(screen.getByText('Created')).toBeInTheDocument();
    expect(screen.getByText('Duration')).toBeInTheDocument();
    expect(screen.getByText('Files')).toBeInTheDocument();
    expect(screen.getByText('Size')).toBeInTheDocument();
    expect(screen.getByText('Version')).toBeInTheDocument();
    expect(screen.getByText('Action')).toBeInTheDocument();
  });

  it('renders backup data rows', () => {
    renderPanel();

    expect(screen.getByText('1,234')).toBeInTheDocument();
    expect(screen.getByText('1,100')).toBeInTheDocument();
    expect(screen.getAllByText('1.21.4').length).toBeGreaterThanOrEqual(1);
  });

  it('shows restore button for completed backups', () => {
    renderPanel();

    const restoreButtons = screen.getAllByText('Restore');
    expect(restoreButtons.length).toBeGreaterThanOrEqual(1);
  });

  it('disables restore button for failed backups', () => {
    renderPanel();

    const restoreButtons = screen.getAllByText('Restore');
    const failedRow = restoreButtons[1];
    expect(failedRow.closest('button')).toBeDisabled();
  });
});
