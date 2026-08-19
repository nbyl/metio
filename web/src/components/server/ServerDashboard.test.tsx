import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ServerDashboard } from './ServerDashboard';
import type { ServerResponse, StatusResponse } from '../../types/server';

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

import { toast } from 'sonner';

const mockNavigate = vi.hoisted(() => vi.fn());

vi.mock('react-router-dom', () => ({
  useNavigate: () => mockNavigate,
}));

vi.mock('../../hooks/useServers', () => ({
  useServers: vi.fn(),
}));

vi.mock('../../hooks/useServerStatus', () => ({
  useServerStatus: vi.fn(),
}));

vi.mock('../../hooks/useServerProvisioning', () => ({
  useServerProvisioning: vi.fn(),
}));

vi.mock('../../hooks/useServerMutations', () => ({
  useStartServer: vi.fn(),
  useStopServer: vi.fn(),
  useUpdateServer: vi.fn(),
  useUpdateAgent: vi.fn(),
  useDeleteServer: vi.fn(),
}));

vi.mock('../../hooks/useWhitelist', () => ({
  useWhitelist: vi.fn(),
  useAddPlayer: vi.fn(),
  useRemovePlayer: vi.fn(),
  useToggleWhitelist: vi.fn(),
}));

vi.mock('../../hooks/useScheduledShutdown', () => ({
  useScheduleShutdown: vi.fn(),
  useCancelScheduledShutdown: vi.fn(),
}));

vi.mock('../../hooks/useBackupSettings', () => ({
  useBackupSettings: vi.fn(),
  useUpdateBackupSettings: vi.fn(),
}));

vi.mock('../../hooks/useServerOptions', () => ({
  useServerOptions: vi.fn(),
}));

const copyState = vi.hoisted(() => ({ copied: false, fn: vi.fn() }));

vi.mock('../../hooks/useCopyToClipboard', () => ({
  useCopyToClipboard: () => ({
    copy: copyState.fn,
    copied: copyState.copied,
  }),
}));

import { useServers } from '../../hooks/useServers';
import { useServerStatus } from '../../hooks/useServerStatus';
import { useServerProvisioning } from '../../hooks/useServerProvisioning';
import {
  useStartServer,
  useStopServer,
  useUpdateServer,
  useUpdateAgent,
  useDeleteServer,
} from '../../hooks/useServerMutations';
import {
  useWhitelist,
  useAddPlayer,
  useRemovePlayer,
  useToggleWhitelist,
} from '../../hooks/useWhitelist';
import {
  useScheduleShutdown,
  useCancelScheduledShutdown,
} from '../../hooks/useScheduledShutdown';
import {
  useBackupSettings,
  useUpdateBackupSettings,
} from '../../hooks/useBackupSettings';
import { useServerOptions } from '../../hooks/useServerOptions';

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

const runningStatus: StatusResponse = {
  serverState: 'RUNNING',
  players: { current: 5, max: 20 },
  uptime: '3h 45m',
  version: '1.20.4',
  instanceIP: '192.168.1.100',
};

const stoppedStatus: StatusResponse = {
  serverState: 'STOPPED',
  players: { current: 0, max: 20 },
  uptime: '',
  version: '1.20.4',
  instanceIP: '',
};

const startingStatus: StatusResponse = {
  serverState: 'STARTING',
  players: { current: 0, max: 20 },
  uptime: '',
  version: '1.20.4',
  instanceIP: '192.168.1.100',
};

const stoppingStatus: StatusResponse = {
  serverState: 'STOPPING',
  players: { current: 3, max: 20 },
  uptime: '4h 12m',
  version: '1.20.4',
  instanceIP: '192.168.1.100',
};

function mockServerResponse(status: StatusResponse): ServerResponse {
  return {
    id: 'srv1',
    config: {
      name: 'Survival',
      region: 'europe-west1',
      zone: 'europe-west1-b',
      machineType: 'e2-small',
      minecraftVersion: '1.20.4',
      diskSizeGB: 20,
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    },
    status,
    currentInfraVersion: 1,
    outdated: false,
  };
}

function mockMutationHook() {
  return { mutate: vi.fn(), isPending: false };
}

function mockServerList(data: ServerResponse[]) {
  vi.mocked(useServers).mockReturnValue({
    data,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  } as never);
}

function applyBaseMocks() {
  vi.mocked(useServerStatus).mockReturnValue({ data: undefined } as never);
  vi.mocked(useServerProvisioning).mockReturnValue({
    data: undefined,
  } as never);
  vi.mocked(useStartServer).mockReturnValue(mockMutationHook() as never);
  vi.mocked(useStopServer).mockReturnValue(mockMutationHook() as never);
  vi.mocked(useUpdateServer).mockReturnValue(mockMutationHook() as never);
  vi.mocked(useUpdateAgent).mockReturnValue(mockMutationHook() as never);
  vi.mocked(useDeleteServer).mockReturnValue(mockMutationHook() as never);
  vi.mocked(useWhitelist).mockReturnValue({ data: undefined } as never);
  vi.mocked(useAddPlayer).mockReturnValue(mockMutationHook() as never);
  vi.mocked(useRemovePlayer).mockReturnValue(mockMutationHook() as never);
  vi.mocked(useToggleWhitelist).mockReturnValue(mockMutationHook() as never);
  vi.mocked(useScheduleShutdown).mockReturnValue(mockMutationHook() as never);
  vi.mocked(useCancelScheduledShutdown).mockReturnValue(
    mockMutationHook() as never
  );
  vi.mocked(useBackupSettings).mockReturnValue({
    data: { enabled: true },
    isLoading: false,
  } as never);
  vi.mocked(useUpdateBackupSettings).mockReturnValue(
    mockMutationHook() as never
  );
  vi.mocked(useServerOptions).mockReturnValue({
    data: { machineTypes: [], minecraftVersions: ['1.20.4'] },
    isLoading: false,
  } as never);
  copyState.copied = false;
  copyState.fn.mockReset();
}

let queryClient: QueryClient;

function renderDashboard() {
  return render(
    <QueryClientProvider client={queryClient}>
      <ServerDashboard />
    </QueryClientProvider>
  );
}

describe('ServerDashboard stats', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = createQueryClient();
    applyBaseMocks();
    mockServerList([mockServerResponse(stoppedStatus)]);
  });

  it('hides Players and Uptime when server is stopped', () => {
    renderDashboard();

    expect(screen.getByText('State')).toBeInTheDocument();
    expect(screen.getByText('IP')).toBeInTheDocument();
    expect(screen.queryByText('Players')).not.toBeInTheDocument();
    expect(screen.queryByText('Uptime')).not.toBeInTheDocument();
  });

  it('hides Players and Uptime while server is starting', () => {
    vi.mocked(useServers).mockReturnValue({
      data: [mockServerResponse(startingStatus)],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as never);

    renderDashboard();

    expect(screen.getByText('State')).toBeInTheDocument();
    expect(screen.getByText('IP')).toBeInTheDocument();
    expect(screen.queryByText('Players')).not.toBeInTheDocument();
    expect(screen.queryByText('Uptime')).not.toBeInTheDocument();
  });

  it('hides Players and Uptime while server is stopping', () => {
    vi.mocked(useServers).mockReturnValue({
      data: [mockServerResponse(stoppingStatus)],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as never);

    renderDashboard();

    expect(screen.getByText('State')).toBeInTheDocument();
    expect(screen.getByText('IP')).toBeInTheDocument();
    expect(screen.queryByText('Players')).not.toBeInTheDocument();
    expect(screen.queryByText('Uptime')).not.toBeInTheDocument();
  });

  it('shows Players and Uptime when server is running', () => {
    vi.mocked(useServers).mockReturnValue({
      data: [mockServerResponse(runningStatus)],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as never);

    renderDashboard();

    expect(screen.getByText('State')).toBeInTheDocument();
    expect(screen.getByText('IP')).toBeInTheDocument();
    expect(screen.getByText('Players')).toBeInTheDocument();
    expect(screen.getByText('5/20')).toBeInTheDocument();
    expect(screen.getByText('Uptime')).toBeInTheDocument();
    expect(screen.getByText('3h 45m')).toBeInTheDocument();
  });
});

describe('ServerDashboard create server button', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = createQueryClient();
    applyBaseMocks();
  });

  it('shows the Create Server button when servers already exist', () => {
    vi.mocked(useServers).mockReturnValue({
      data: [mockServerResponse(stoppedStatus)],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as never);

    renderDashboard();

    expect(
      screen.getByRole('button', { name: /create server/i })
    ).toBeInTheDocument();
  });

  it('navigates to the setup wizard when the Create Server button is clicked', async () => {
    vi.mocked(useServers).mockReturnValue({
      data: [mockServerResponse(stoppedStatus)],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as never);

    renderDashboard();

    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: /create server/i }));

    expect(mockNavigate).toHaveBeenCalledWith('/servers/new');
  });

  it('shows exactly one Create Server button in the empty state', () => {
    vi.mocked(useServers).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as never);

    renderDashboard();

    expect(
      screen.getAllByRole('button', { name: /create server/i })
    ).toHaveLength(1);
  });
});

describe('ServerDashboard backup settings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = createQueryClient();
    applyBaseMocks();
    mockServerList([mockServerResponse(stoppedStatus)]);
  });

  it('shows the backup settings inside the settings modal', async () => {
    renderDashboard();

    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: /update/i }));
    await user.click(screen.getByRole('tab', { name: 'Backup' }));

    expect(screen.getByRole('tab', { name: 'Backup' })).toHaveAttribute(
      'aria-selected',
      'true'
    );
    expect(screen.getByText('Scheduled backups enabled')).toBeInTheDocument();
  });

  it('preloads current settings into the backup form', async () => {
    vi.mocked(useBackupSettings).mockReturnValue({
      data: {
        enabled: true,
        backupIntervalHours: 6,
        keep: 3,
        keepUnit: 'daily',
      },
      isLoading: false,
    } as never);

    renderDashboard();
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: /update/i }));
    await user.click(screen.getByRole('tab', { name: 'Backup' }));

    expect(screen.getByLabelText('Backup interval (hours)')).toHaveValue(6);
    expect(screen.getByLabelText('Retention policy')).toHaveValue(3);
  });

  it('saves settings and navigates to provisioning on success', async () => {
    vi.mocked(useBackupSettings).mockReturnValue({
      data: { enabled: true, backupIntervalHours: 1 },
      isLoading: false,
    } as never);
    const mockMutate = vi.fn(
      (_settings: unknown, opts?: { onSuccess?: () => void }) =>
        opts?.onSuccess?.()
    );
    vi.mocked(useUpdateBackupSettings).mockReturnValue({
      mutate: mockMutate,
      isPending: false,
    } as never);

    renderDashboard();
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: /update/i }));
    await user.click(screen.getByRole('tab', { name: 'Backup' }));
    await user.click(screen.getByRole('button', { name: /save/i }));

    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({ enabled: true, backupIntervalHours: 1 }),
      expect.anything()
    );
    expect(mockNavigate).toHaveBeenCalledWith('/servers/srv1/provisioning', {
      state: { serverName: 'Survival' },
    });
  });
});

describe('ServerDashboard loading and error states', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = createQueryClient();
    applyBaseMocks();
  });

  it('renders skeletons while servers are loading', () => {
    vi.mocked(useServers).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
      refetch: vi.fn(),
    } as never);

    const { container } = renderDashboard();

    expect(
      container.querySelectorAll('[data-slot="skeleton"]').length
    ).toBeGreaterThan(0);
  });

  it('renders the error message and retries', async () => {
    const user = userEvent.setup();
    const refetch = vi.fn();
    vi.mocked(useServers).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('boom'),
      refetch,
    } as never);

    renderDashboard();

    expect(screen.getByText('Error: boom')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(refetch).toHaveBeenCalled();
  });

  it('shows Unknown when the server has no status', () => {
    mockServerList([
      { ...mockServerResponse(runningStatus), status: undefined },
    ]);

    renderDashboard();

    expect(screen.getAllByText('Unknown').length).toBeGreaterThanOrEqual(1);
  });
});

describe('ServerDashboard server controls', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = createQueryClient();
    applyBaseMocks();
  });

  it('starts a stopped server', async () => {
    const user = userEvent.setup();
    const startMutation = vi.fn();
    vi.mocked(useStartServer).mockReturnValue({
      mutate: startMutation,
      isPending: false,
    } as never);
    mockServerList([mockServerResponse(stoppedStatus)]);

    renderDashboard();
    await user.click(screen.getByRole('button', { name: 'Start Server' }));

    expect(startMutation).toHaveBeenCalled();
  });

  it('stops a running server', async () => {
    const user = userEvent.setup();
    const stopMutation = vi.fn();
    vi.mocked(useStopServer).mockReturnValue({
      mutate: stopMutation,
      isPending: false,
    } as never);
    mockServerList([mockServerResponse(runningStatus)]);

    renderDashboard();
    await user.click(screen.getByRole('button', { name: 'Stop Server' }));

    expect(stopMutation).toHaveBeenCalled();
  });

  it('shows a disabled transitioning button while starting', () => {
    mockServerList([mockServerResponse(startingStatus)]);

    renderDashboard();

    const button = screen.getByRole('button', { name: 'Starting...' });
    expect(button).toBeDisabled();
  });

  it('shows a disabled transitioning button while stopping', () => {
    mockServerList([mockServerResponse(stoppingStatus)]);

    renderDashboard();

    const button = screen.getByRole('button', { name: 'Stopping...' });
    expect(button).toBeDisabled();
  });
});

describe('ServerDashboard copy IP', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = createQueryClient();
    applyBaseMocks();
    mockServerList([mockServerResponse(runningStatus)]);
  });

  it('shows a success toast when the IP is copied', async () => {
    const user = userEvent.setup();
    copyState.fn.mockResolvedValue(true);
    renderDashboard();

    await user.click(screen.getByRole('button', { name: 'Copy IP' }));

    expect(copyState.fn).toHaveBeenCalledWith('192.168.1.100');
    expect(toast.success).toHaveBeenCalledWith('IP copied to clipboard!');
  });

  it('shows an error toast when copying fails', async () => {
    const user = userEvent.setup();
    copyState.fn.mockResolvedValue(false);
    renderDashboard();

    await user.click(screen.getByRole('button', { name: 'Copy IP' }));

    expect(toast.error).toHaveBeenCalledWith('Failed to copy IP');
  });

  it('shows Copied when the copied flag is set', () => {
    copyState.copied = true;
    renderDashboard();

    expect(screen.getByText('Copied!')).toBeInTheDocument();
    expect(screen.queryByText('Copy IP')).toBeNull();
  });
});

describe('ServerDashboard update and destroy flows', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = createQueryClient();
    applyBaseMocks();
    mockServerList([mockServerResponse(stoppedStatus)]);
  });

  it('submits an update and navigates to provisioning on success', async () => {
    const user = userEvent.setup();
    const updateMutation = vi.fn(
      (_vars: unknown, opts?: { onSuccess?: () => void }) => opts?.onSuccess?.()
    );
    vi.mocked(useUpdateServer).mockReturnValue({
      mutate: updateMutation,
      isPending: false,
    } as never);

    renderDashboard();
    await user.click(screen.getByRole('button', { name: 'Update' }));
    await user.clear(screen.getByDisplayValue('Survival'));
    await user.type(screen.getByDisplayValue(''), 'Renamed');
    await user.click(screen.getByRole('button', { name: 'Update Server' }));

    expect(updateMutation).toHaveBeenCalledWith(
      { id: 'srv1', data: { name: 'Renamed' } },
      expect.anything()
    );
    expect(mockNavigate).toHaveBeenCalledWith('/servers/srv1/provisioning', {
      state: { serverName: 'Survival' },
    });
  });

  it('destroys the server after confirming the name', async () => {
    const user = userEvent.setup();
    const deleteMutation = vi.fn(
      (_vars: unknown, opts?: { onSuccess?: () => void }) => opts?.onSuccess?.()
    );
    vi.mocked(useDeleteServer).mockReturnValue({
      mutate: deleteMutation,
      isPending: false,
    } as never);

    renderDashboard();
    await user.click(screen.getByRole('button', { name: 'Destroy' }));
    await user.click(screen.getByRole('button', { name: 'Continue →' }));
    await user.type(screen.getByPlaceholderText('Survival'), 'Survival');
    await user.click(screen.getByRole('button', { name: 'Destroy Server' }));

    expect(deleteMutation).toHaveBeenCalledWith(
      { id: 'srv1' },
      expect.anything()
    );
    expect(mockNavigate).toHaveBeenCalledWith('/servers/srv1/provisioning', {
      state: { serverName: 'Survival' },
    });
  });

  it('updates the agent and navigates to provisioning on success', async () => {
    const user = userEvent.setup();
    const agentMutation = vi.fn(
      (_vars: unknown, opts?: { onSuccess?: () => void }) => opts?.onSuccess?.()
    );
    vi.mocked(useUpdateAgent).mockReturnValue({
      mutate: agentMutation,
      isPending: false,
    } as never);
    mockServerList([
      { ...mockServerResponse(runningStatus), outdatedMachineAgent: true },
    ]);

    renderDashboard();
    await user.click(screen.getByRole('button', { name: 'Update Agent' }));

    expect(agentMutation).toHaveBeenCalledWith(undefined, expect.anything());
    expect(mockNavigate).toHaveBeenCalledWith('/servers/srv1/provisioning', {
      state: { serverName: 'Survival' },
    });
  });

  it('shows the outdated server label in the card title', () => {
    mockServerList([{ ...mockServerResponse(stoppedStatus), outdated: true }]);

    renderDashboard();

    expect(
      screen.getAllByText('Update Available').length
    ).toBeGreaterThanOrEqual(1);
  });
});

describe('ServerDashboard provisioning banner', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = createQueryClient();
    applyBaseMocks();
    mockServerList([mockServerResponse(stoppedStatus)]);
  });

  function mockProvisioning(operation: string) {
    vi.mocked(useServerProvisioning).mockReturnValue({
      data: {
        id: 'srv1',
        operation,
        state: 'IN_PROGRESS',
        currentStep: 'step',
        progress: 42,
        steps: [],
        startedAt: '2026-01-01T00:00:00Z',
      },
    } as never);
  }

  it('renders the creating banner with progress', async () => {
    const user = userEvent.setup();
    mockProvisioning('CREATE');
    const { container } = renderDashboard();

    expect(screen.getByText('Creating...')).toBeInTheDocument();
    expect(screen.getByText('42%')).toBeInTheDocument();
    const bar = container.querySelector('.bg-green-500');
    expect(bar).toHaveStyle({ width: '42%' });

    await user.click(screen.getByText('Creating...'));
    expect(mockNavigate).toHaveBeenCalledWith('/servers/srv1/provisioning', {
      state: { serverName: 'Survival' },
    });
  });

  it('renders the updating banner', () => {
    mockProvisioning('UPDATE');
    renderDashboard();

    expect(screen.getByText('Updating...')).toBeInTheDocument();
  });

  it('renders the destroying banner', () => {
    mockProvisioning('DESTROY');
    renderDashboard();

    expect(screen.getByText('Destroying...')).toBeInTheDocument();
  });
});

describe('ServerDashboard whitelist', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = createQueryClient();
    applyBaseMocks();
    mockServerList([mockServerResponse(runningStatus)]);
  });

  const whitelist = {
    enabled: true,
    players: [{ username: 'alice', uuid: 'u1', addedAt: '', addedBy: 'admin' }],
  };

  it('shows the players when expanded', async () => {
    const user = userEvent.setup();
    vi.mocked(useWhitelist).mockReturnValue({
      data: whitelist,
      isLoading: false,
    } as never);

    renderDashboard();
    await user.click(screen.getByText('Whitelist'));

    expect(screen.getByText('alice')).toBeInTheDocument();
    expect(screen.getByText('(1)')).toBeInTheDocument();
  });

  it('toggles the whitelist switch', async () => {
    const user = userEvent.setup();
    const toggleMutation = vi.fn();
    vi.mocked(useToggleWhitelist).mockReturnValue({
      mutate: toggleMutation,
      isPending: false,
    } as never);
    vi.mocked(useWhitelist).mockReturnValue({
      data: whitelist,
      isLoading: false,
    } as never);

    renderDashboard();
    await user.click(screen.getByRole('switch', { name: 'Toggle whitelist' }));

    expect(toggleMutation).toHaveBeenCalledWith(false);
  });

  it('adds a player and clears the input on success', async () => {
    const user = userEvent.setup();
    const addMutation = vi.fn(
      (_username: string, opts?: { onSuccess?: () => void }) =>
        opts?.onSuccess?.()
    );
    vi.mocked(useAddPlayer).mockReturnValue({
      mutate: addMutation,
      isPending: false,
    } as never);
    vi.mocked(useWhitelist).mockReturnValue({
      data: whitelist,
      isLoading: false,
    } as never);

    renderDashboard();
    await user.click(screen.getByText('Whitelist'));
    await user.type(screen.getByPlaceholderText('Minecraft username'), 'Bobby');
    await user.click(screen.getByRole('button', { name: 'Add' }));

    expect(addMutation).toHaveBeenCalledWith('Bobby', expect.anything());
    expect(screen.getByPlaceholderText('Minecraft username')).toHaveValue('');
  });

  it('does not add a player when the input is empty', async () => {
    const user = userEvent.setup();
    const addMutation = vi.fn();
    vi.mocked(useAddPlayer).mockReturnValue({
      mutate: addMutation,
      isPending: false,
    } as never);
    vi.mocked(useWhitelist).mockReturnValue({
      data: whitelist,
      isLoading: false,
    } as never);

    renderDashboard();
    await user.click(screen.getByText('Whitelist'));

    const addButton = screen.getByRole('button', { name: 'Add' });
    expect(addButton).toBeDisabled();
    await user.click(addButton);
    expect(addMutation).not.toHaveBeenCalled();
  });

  it('removes a player', async () => {
    const user = userEvent.setup();
    const removeMutation = vi.fn();
    vi.mocked(useRemovePlayer).mockReturnValue({
      mutate: removeMutation,
      isPending: false,
    } as never);
    vi.mocked(useWhitelist).mockReturnValue({
      data: whitelist,
      isLoading: false,
    } as never);

    renderDashboard();
    await user.click(screen.getByText('Whitelist'));
    await user.click(screen.getByRole('button', { name: 'Remove alice' }));

    expect(removeMutation).toHaveBeenCalledWith('u1');
  });

  it('shows the empty whitelist message', async () => {
    const user = userEvent.setup();
    vi.mocked(useWhitelist).mockReturnValue({
      data: { enabled: true, players: [] },
      isLoading: false,
    } as never);

    renderDashboard();
    await user.click(screen.getByText('Whitelist'));

    expect(screen.getByText('No players in whitelist')).toBeInTheDocument();
  });

  it('shows a spinner while the whitelist is loading', async () => {
    const user = userEvent.setup();
    vi.mocked(useWhitelist).mockReturnValue({
      data: undefined,
      isLoading: true,
    } as never);

    renderDashboard();
    await user.click(screen.getByText('Whitelist'));

    expect(document.querySelector('.animate-spin')).not.toBeNull();
  });

  it('omits the switch when no whitelist data is available', async () => {
    const user = userEvent.setup();
    vi.mocked(useWhitelist).mockReturnValue({
      data: undefined,
      isLoading: false,
    } as never);

    renderDashboard();
    await user.click(screen.getByText('Whitelist'));

    expect(
      screen.queryByRole('switch', { name: 'Toggle whitelist' })
    ).toBeNull();
  });
});

describe('ServerDashboard scheduled shutdown', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = createQueryClient();
    applyBaseMocks();
  });

  it('schedules a shutdown from the time input', async () => {
    const user = userEvent.setup();
    const scheduleMutation = vi.fn(
      (_iso: string, opts?: { onSuccess?: () => void }) => opts?.onSuccess?.()
    );
    vi.mocked(useScheduleShutdown).mockReturnValue({
      mutate: scheduleMutation,
      isPending: false,
    } as never);
    mockServerList([mockServerResponse(runningStatus)]);

    const { container } = renderDashboard();
    await user.click(screen.getByText('Scheduled Shutdown'));
    const timeInput = container.querySelector(
      'input[type="time"]'
    ) as HTMLInputElement;
    await user.type(timeInput, '14:30');
    await user.click(screen.getByRole('button', { name: 'Schedule' }));

    expect(scheduleMutation).toHaveBeenCalledWith(
      expect.stringMatching(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/),
      expect.anything()
    );
    expect(screen.getByRole('button', { name: 'Schedule' })).toBeDisabled();
  });

  it('does not schedule without a time', async () => {
    const user = userEvent.setup();
    const scheduleMutation = vi.fn();
    vi.mocked(useScheduleShutdown).mockReturnValue({
      mutate: scheduleMutation,
      isPending: false,
    } as never);
    mockServerList([mockServerResponse(runningStatus)]);

    renderDashboard();
    await user.click(screen.getByText('Scheduled Shutdown'));

    expect(screen.getByRole('button', { name: 'Schedule' })).toBeDisabled();
    await user.click(screen.getByRole('button', { name: 'Schedule' }));
    expect(scheduleMutation).not.toHaveBeenCalled();
  });

  it('shows the shutdown time and cancels it', async () => {
    const user = userEvent.setup();
    const cancelMutation = vi.fn();
    vi.mocked(useCancelScheduledShutdown).mockReturnValue({
      mutate: cancelMutation,
      isPending: false,
    } as never);
    const status = {
      ...runningStatus,
      scheduledShutdown: new Date(Date.now() + 30 * 60000).toISOString(),
    };
    mockServerList([mockServerResponse(status)]);

    renderDashboard();
    await user.click(screen.getByText('Scheduled Shutdown'));

    expect(screen.getByText(/Server will shut down at/)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Cancel Shutdown' }));
    expect(cancelMutation).toHaveBeenCalled();
  });

  it('shows the time until shutdown as imminent when in the past', () => {
    mockServerList([
      mockServerResponse({
        ...runningStatus,
        scheduledShutdown: new Date(Date.now() - 60000).toISOString(),
      }),
    ]);

    renderDashboard();

    expect(screen.getByText('imminent')).toBeInTheDocument();
  });

  it('shows the time until shutdown in minutes', () => {
    mockServerList([
      mockServerResponse({
        ...runningStatus,
        scheduledShutdown: new Date(
          Date.now() + 5 * 60000 + 30000
        ).toISOString(),
      }),
    ]);

    renderDashboard();

    expect(screen.getByText('5 min')).toBeInTheDocument();
  });

  it('shows the time until shutdown in hours and minutes', () => {
    mockServerList([
      mockServerResponse({
        ...runningStatus,
        scheduledShutdown: new Date(
          Date.now() + 125 * 60000 + 30000
        ).toISOString(),
      }),
    ]);

    renderDashboard();

    expect(screen.getByText('2h 5m')).toBeInTheDocument();
  });
});
