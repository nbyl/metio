import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BackupCatalogPage } from './BackupCatalogPage';
import { useAllBackups } from '../../hooks/useBackups';

const mockNavigate = vi.hoisted(() => vi.fn());
const mockSearchParams = vi.hoisted(() => new URLSearchParams());
const mockSetSearchParams = vi.hoisted(() => vi.fn());

vi.mock('react-router-dom', () => ({
  useNavigate: () => mockNavigate,
  useSearchParams: () => [mockSearchParams, mockSetSearchParams],
}));

vi.mock('../../hooks/useBackups', () => ({
  useAllBackups: vi.fn(),
  useCreateServerFromBackup: vi.fn(() => ({
    mutate: vi.fn(),
    isPending: false,
  })),
  useRestoreBackup: vi.fn(() => ({
    mutate: vi.fn(),
    isPending: false,
  })),
}));

vi.mock('../../hooks/useServerOptions', () => ({
  useServerOptions: vi.fn(() => ({ data: undefined, isLoading: false })),
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
    sourceConfig: {
      region: 'europe-west1',
      zone: 'europe-west1-b',
      machineType: 'e2-medium',
      diskSizeGB: 10,
      minecraftVersion: '1.21.4',
    },
  },
  {
    id: 'srv2:snap2',
    serverId: 'srv2',
    serverName: 'Creative',
    snapshotId: 'snap2',
    repositoryPrefix: 'servers/srv2/restic/',
    createdAt: '2026-08-19T10:00:00Z',
    durationSeconds: 30,
    fileCount: 500,
    repositorySize: 200000,
    minecraftVersion: '1.21.3',
    status: 'COMPLETED' as const,
    serverDeletedAt: '2026-08-22T12:00:00Z',
    retentionUntil: '2026-09-21T12:00:00Z',
  },
];

function renderPage() {
  return render(<BackupCatalogPage />);
}

beforeEach(() => {
  vi.clearAllMocks();
  for (const key of Array.from(mockSearchParams.keys())) {
    mockSearchParams.delete(key);
  }
  mockSetSearchParams.mockReset();
});

describe('BackupCatalogPage loading', () => {
  it('shows loading skeletons', () => {
    vi.mocked(useAllBackups).mockReturnValue({
      data: undefined,
      isLoading: true,
    } as never);

    renderPage();

    expect(screen.getByText('Backups')).toBeInTheDocument();
    expect(document.querySelectorAll('[data-slot="skeleton"]').length).toBe(18);
  });
});

describe('BackupCatalogPage error', () => {
  it('shows an error message with retry button', () => {
    vi.mocked(useAllBackups).mockReturnValue({
      data: undefined,
      error: new Error('Network failure'),
      isLoading: false,
      refetch: vi.fn(),
    } as never);

    renderPage();

    expect(screen.getByText(/Network failure/)).toBeInTheDocument();
    expect(screen.getByText('Retry')).toBeInTheDocument();
  });
});

describe('BackupCatalogPage empty', () => {
  it('shows an empty state when there are no backups', () => {
    vi.mocked(useAllBackups).mockReturnValue({
      data: [],
      isLoading: false,
    } as never);

    renderPage();

    expect(screen.getByText(/No backups found/)).toBeInTheDocument();
  });
});

describe('BackupCatalogPage with backups', () => {
  beforeEach(() => {
    vi.mocked(useAllBackups).mockReturnValue({
      data: backups,
      isLoading: false,
    } as never);
  });

  it('renders the table headers', () => {
    renderPage();

    expect(screen.getByText('Server')).toBeInTheDocument();
    expect(screen.getByText('Status')).toBeInTheDocument();
    expect(screen.getByText('Created')).toBeInTheDocument();
    expect(screen.getByText('Duration')).toBeInTheDocument();
    expect(screen.getByText('Files')).toBeInTheDocument();
    expect(screen.getByText('Size')).toBeInTheDocument();
    expect(screen.getByText('Version')).toBeInTheDocument();
    expect(screen.getByText('Retention')).toBeInTheDocument();
    expect(screen.getByText('Actions')).toBeInTheDocument();
  });

  it('renders backup data', () => {
    renderPage();

    expect(screen.getByText('Survival')).toBeInTheDocument();
    expect(screen.getByText('Creative')).toBeInTheDocument();
    expect(screen.getByText('1,234')).toBeInTheDocument();
    expect(screen.getByText('500')).toBeInTheDocument();
  });

  it('shows the deleted badge for deleted server backups', () => {
    renderPage();

    expect(
      screen
        .getAllByText('Deleted')
        .filter((el) => el.getAttribute('data-slot') === 'badge').length
    ).toBeGreaterThanOrEqual(1);
  });

  it('shows retention date for deleted server backups', () => {
    renderPage();

    expect(screen.getByText(/Retention until/)).toBeInTheDocument();
  });

  it('shows Create Server in dropdown for completed backups with sourceConfig', async () => {
    const user = userEvent.setup();
    renderPage();

    const dropdownTriggers = screen.getAllByRole('button', { name: '' });
    await user.click(dropdownTriggers[0]);

    expect(screen.getByRole('menuitem', { name: /Create Server/ })).toBeInTheDocument();
  });

  it('filters to active servers only', async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole('tab', { name: 'Active' }));

    expect(screen.getByText('Survival')).toBeInTheDocument();
    expect(screen.queryByText('Creative')).not.toBeInTheDocument();
  });

  it('filters to deleted servers only', async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole('tab', { name: 'Deleted' }));

    expect(screen.queryByText('Survival')).not.toBeInTheDocument();
    expect(screen.getByText('Creative')).toBeInTheDocument();
  });

  it('shows all backups when All tab is selected', async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole('tab', { name: 'Deleted' }));
    expect(screen.queryByText('Survival')).not.toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: 'All' }));
    expect(screen.getByText('Survival')).toBeInTheDocument();
    expect(screen.getByText('Creative')).toBeInTheDocument();
  });

  it('displays backup count', () => {
    renderPage();

    expect(screen.getByText('2 backups')).toBeInTheDocument();
  });

  it('shows Restore in dropdown for completed backups of active servers', async () => {
    const user = userEvent.setup();
    renderPage();

    const dropdownTriggers = screen.getAllByRole('button', { name: '' });
    await user.click(dropdownTriggers[0]);

    expect(screen.getByRole('menuitem', { name: /Restore/ })).toBeInTheDocument();
  });

  it('does not show dropdown for deleted server backups without sourceConfig', () => {
    renderPage();

    const rows = screen.getAllByText('Creative').map((el) => el.closest('tr'));
    expect(rows.length).toBeGreaterThanOrEqual(1);
    for (const row of rows) {
      expect(row?.querySelectorAll('[data-slot="dropdown-menu-trigger"]')).toHaveLength(0);
    }
  });
});

describe('BackupCatalogPage server filter', () => {
  beforeEach(() => {
    vi.mocked(useAllBackups).mockReturnValue({
      data: backups,
      isLoading: false,
    } as never);
  });

  it('filters backups by server query param', () => {
    mockSearchParams.set('server', 'srv1');

    renderPage();

    expect(screen.getByText('Survival')).toBeInTheDocument();
    expect(screen.queryByText('Creative')).not.toBeInTheDocument();
  });

  it('shows server name in heading when filtered', () => {
    mockSearchParams.set('server', 'srv1');

    renderPage();

    expect(screen.getByText('Backups for Survival')).toBeInTheDocument();
  });

  it('shows back button when filtered by server', () => {
    mockSearchParams.set('server', 'srv1');

    renderPage();

    expect(
      screen.getByRole('button', { name: 'Show all backups' })
    ).toBeInTheDocument();
  });

  it('hides filter tabs when filtered by server', () => {
    mockSearchParams.set('server', 'srv1');

    renderPage();

    expect(screen.queryByRole('tab', { name: 'All' })).not.toBeInTheDocument();
  });

  it('clears server filter when back button is clicked', async () => {
    const user = userEvent.setup();
    mockSearchParams.set('server', 'srv1');

    renderPage();

    await user.click(screen.getByRole('button', { name: 'Show all backups' }));

    expect(mockSetSearchParams).toHaveBeenCalledWith({});
  });

  it('shows empty state for unknown server id', () => {
    mockSearchParams.set('server', 'unknown');

    renderPage();

    expect(screen.getByText(/No backups found/)).toBeInTheDocument();
  });

  it('shows fallback link in empty state for server filter', () => {
    mockSearchParams.set('server', 'unknown');

    renderPage();

    const buttons = screen.getAllByRole('button', { name: 'Show all backups' });
    expect(buttons.length).toBeGreaterThanOrEqual(2);
  });
});
