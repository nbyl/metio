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

vi.mock('../../hooks/useServers', () => ({
  useServers: vi.fn(() => ({
    data: [
      { id: 'srv1', config: { name: 'Survival' } },
      { id: 'srv2', config: { name: 'Creative' } },
    ],
    isLoading: false,
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
      data: { backups: [], total: 0 },
      isLoading: false,
    } as never);

    renderPage();

    expect(screen.getByText(/No backups found/)).toBeInTheDocument();
  });
});

describe('BackupCatalogPage with backups', () => {
  beforeEach(() => {
    vi.mocked(useAllBackups).mockReturnValue({
      data: { backups, total: 2 },
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

    const activeBadge = screen.getAllByText('Active').find(
      (el) => el.getAttribute('data-slot') === 'badge'
    )!;
    await user.click(activeBadge);

    expect(screen.getByText('Survival')).toBeInTheDocument();
    expect(screen.queryByText('Creative')).not.toBeInTheDocument();
  });

  it('filters to deleted servers only', async () => {
    const user = userEvent.setup();
    renderPage();

    const deletedBadge = screen.getAllByText('Deleted').find(
      (el) => el.getAttribute('data-slot') === 'badge' && el.getAttribute('data-variant') === 'outline'
    )!;
    await user.click(deletedBadge);

    expect(screen.queryByText('Survival')).not.toBeInTheDocument();
    expect(screen.getByText('Creative')).toBeInTheDocument();
  });

  it('shows all backups when All status filter is selected', async () => {
    const user = userEvent.setup();
    renderPage();

    const deletedBadge = screen.getAllByText('Deleted').find(
      (el) => el.getAttribute('data-slot') === 'badge' && el.getAttribute('data-variant') === 'outline'
    )!;
    await user.click(deletedBadge);
    expect(screen.queryByText('Survival')).not.toBeInTheDocument();

    const allBadge = screen.getAllByText('All').find(
      (el) => el.getAttribute('data-slot') === 'badge'
    )!;
    await user.click(allBadge);
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

describe('BackupCatalogPage sort', () => {
  beforeEach(() => {
    vi.mocked(useAllBackups).mockReturnValue({
      data: { backups, total: 2 },
      isLoading: false,
    } as never);
  });

  it('calls useAllBackups with default sort params', () => {
    renderPage();

    expect(vi.mocked(useAllBackups)).toHaveBeenCalledWith({
      sort: 'created_at',
      dir: 'desc',
      limit: 25,
      offset: 0,
      server: undefined,
    });
  });

  it('calls useAllBackups with sort params from URL', () => {
    mockSearchParams.set('sort', 'duration_seconds');
    mockSearchParams.set('dir', 'asc');

    renderPage();

    expect(vi.mocked(useAllBackups)).toHaveBeenCalledWith({
      sort: 'duration_seconds',
      dir: 'asc',
      limit: 25,
      offset: 0,
      server: undefined,
    });
  });

  it('toggles sort direction when clicking same column', async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByText('Created'));

    expect(mockSetSearchParams).toHaveBeenCalled();
    const call = mockSetSearchParams.mock.calls[0][0];
    const next = typeof call === 'function' ? call(mockSearchParams) : call;
    expect(next.get('dir')).toBe('asc');
  });

  it('sets new sort field when clicking different column', async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByText('Size'));

    expect(mockSetSearchParams).toHaveBeenCalledTimes(2);
    const firstCall = mockSetSearchParams.mock.calls[0][0];
    const firstNext = typeof firstCall === 'function' ? firstCall(mockSearchParams) : firstCall;
    expect(firstNext.get('sort')).toBe('repository_size');

    const secondCall = mockSetSearchParams.mock.calls[1][0];
    const secondNext = typeof secondCall === 'function' ? secondCall(firstNext) : secondCall;
    expect(secondNext.get('dir')).toBe('desc');
  });
});

describe('BackupCatalogPage pagination', () => {
  beforeEach(() => {
    vi.mocked(useAllBackups).mockReturnValue({
      data: { backups, total: 2 },
      isLoading: false,
    } as never);
  });

  it('shows page range', () => {
    renderPage();

    expect(screen.getByText('1–2 of 2')).toBeInTheDocument();
  });

  it('disables previous button on first page', () => {
    renderPage();

    expect(screen.getByLabelText('Previous page')).toBeDisabled();
  });

  it('calls useAllBackups with offset=0 for page 1', () => {
    renderPage();

    expect(vi.mocked(useAllBackups)).toHaveBeenCalledWith(
      expect.objectContaining({ offset: 0 })
    );
  });

  it('calls useAllBackups with correct offset for page', () => {
    mockSearchParams.set('page', '3');
    mockSearchParams.set('pageSize', '25');

    renderPage();

    expect(vi.mocked(useAllBackups)).toHaveBeenCalledWith(
      expect.objectContaining({ offset: 50 })
    );
  });
});

describe('BackupCatalogPage page size', () => {
  beforeEach(() => {
    vi.mocked(useAllBackups).mockReturnValue({
      data: { backups, total: 2 },
      isLoading: false,
    } as never);
  });

  it('defaults to page size 25', () => {
    renderPage();

    expect(vi.mocked(useAllBackups)).toHaveBeenCalledWith(
      expect.objectContaining({ limit: 25 })
    );
  });

  it('uses page size from URL', () => {
    mockSearchParams.set('pageSize', '50');

    renderPage();

    expect(vi.mocked(useAllBackups)).toHaveBeenCalledWith(
      expect.objectContaining({ limit: 50 })
    );
  });

  it('sets page to 1 when changing page size', async () => {
    mockSearchParams.set('page', '3');
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByText('50'));

    const call = mockSetSearchParams.mock.calls[0][0];
    const next = typeof call === 'function' ? call(new URLSearchParams()) : call;
    expect(next.get('pageSize')).toBe('50');
    expect(next.get('page')).toBe('1');
  });
});

describe('BackupCatalogPage server filter', () => {
  beforeEach(() => {
    vi.mocked(useAllBackups).mockImplementation((params: { server?: string } = {}) => {
      const filtered = params.server
        ? backups.filter((b) => b.serverId === params.server)
        : backups;
      return {
        data: { backups: filtered, total: filtered.length },
        isLoading: false,
      } as never;
    });
  });

  it('filters backups by server query param', () => {
    mockSearchParams.set('server', 'srv1');

    renderPage();

    expect(screen.getAllByText('Survival').length).toBeGreaterThanOrEqual(1);
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

  it('passes server filter to useAllBackups', () => {
    mockSearchParams.set('server', 'srv1');

    renderPage();

    expect(vi.mocked(useAllBackups)).toHaveBeenCalledWith(
      expect.objectContaining({ server: 'srv1' })
    );
  });

  it('shows All Servers button in header', () => {
    renderPage();

    expect(screen.getByText('All Servers')).toBeInTheDocument();
  });

  it('clears server filter when back button is clicked', async () => {
    const user = userEvent.setup();
    mockSearchParams.set('server', 'srv1');

    renderPage();

    await user.click(screen.getByRole('button', { name: 'Show all backups' }));

    expect(mockSetSearchParams).toHaveBeenCalled();
  });
});
