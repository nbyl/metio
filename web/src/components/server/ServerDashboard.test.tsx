import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ServerDashboard } from './ServerDashboard'
import type { ServerResponse, StatusResponse } from '../../types/server'

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

const mockNavigate = vi.hoisted(() => vi.fn())

vi.mock('react-router-dom', () => ({
  useNavigate: () => mockNavigate,
}))

vi.mock('../../hooks/useServers', () => ({
  useServers: vi.fn(),
}))

vi.mock('../../hooks/useServerStatus', () => ({
  useServerStatus: vi.fn(),
}))

vi.mock('../../hooks/useServerProvisioning', () => ({
  useServerProvisioning: vi.fn(),
}))

vi.mock('../../hooks/useServerMutations', () => ({
  useStartServer: vi.fn(),
  useStopServer: vi.fn(),
  useUpdateServer: vi.fn(),
  useUpdateAgent: vi.fn(),
  useDeleteServer: vi.fn(),
}))

vi.mock('../../hooks/useWhitelist', () => ({
  useWhitelist: vi.fn(),
  useAddPlayer: vi.fn(),
  useRemovePlayer: vi.fn(),
  useToggleWhitelist: vi.fn(),
}))

vi.mock('../../hooks/useScheduledShutdown', () => ({
  useScheduleShutdown: vi.fn(),
  useCancelScheduledShutdown: vi.fn(),
}))

vi.mock('../../hooks/useBackupSettings', () => ({
  useBackupSettings: vi.fn(),
  useUpdateBackupSettings: vi.fn(),
}))

vi.mock('../../hooks/useCopyToClipboard', () => ({
  useCopyToClipboard: () => ({ copy: vi.fn(), copied: false }),
}))

import { useServers } from '../../hooks/useServers'
import { useServerStatus } from '../../hooks/useServerStatus'
import { useServerProvisioning } from '../../hooks/useServerProvisioning'
import {
  useStartServer,
  useStopServer,
  useUpdateServer,
  useUpdateAgent,
  useDeleteServer,
} from '../../hooks/useServerMutations'
import {
  useWhitelist,
  useAddPlayer,
  useRemovePlayer,
  useToggleWhitelist,
} from '../../hooks/useWhitelist'
import {
  useScheduleShutdown,
  useCancelScheduledShutdown,
} from '../../hooks/useScheduledShutdown'
import {
  useBackupSettings,
  useUpdateBackupSettings,
} from '../../hooks/useBackupSettings'

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
}

const runningStatus: StatusResponse = {
  serverState: 'RUNNING',
  players: { current: 5, max: 20 },
  uptime: '3h 45m',
  version: '1.20.4',
  instanceIP: '192.168.1.100',
}

const stoppedStatus: StatusResponse = {
  serverState: 'STOPPED',
  players: { current: 0, max: 20 },
  uptime: '',
  version: '1.20.4',
  instanceIP: '',
}

const startingStatus: StatusResponse = {
  serverState: 'STARTING',
  players: { current: 0, max: 20 },
  uptime: '',
  version: '1.20.4',
  instanceIP: '192.168.1.100',
}

const stoppingStatus: StatusResponse = {
  serverState: 'STOPPING',
  players: { current: 3, max: 20 },
  uptime: '4h 12m',
  version: '1.20.4',
  instanceIP: '192.168.1.100',
}

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
  }
}

function mockMutationHook() {
  return { mutate: vi.fn(), isPending: false }
}

describe('ServerDashboard stats', () => {
  let queryClient: QueryClient

  beforeEach(() => {
    vi.clearAllMocks()
    queryClient = createQueryClient()

    vi.mocked(useServers).mockReturnValue({
      data: [mockServerResponse(stoppedStatus)],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as never)
    vi.mocked(useServerStatus).mockReturnValue({ data: undefined } as never)
    vi.mocked(useServerProvisioning).mockReturnValue({
      data: undefined,
    } as never)
    vi.mocked(useStartServer).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useStopServer).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useUpdateServer).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useUpdateAgent).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useDeleteServer).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useWhitelist).mockReturnValue({ data: undefined } as never)
    vi.mocked(useAddPlayer).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useRemovePlayer).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useToggleWhitelist).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useScheduleShutdown).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useCancelScheduledShutdown).mockReturnValue(
      mockMutationHook() as never
    )
    vi.mocked(useBackupSettings).mockReturnValue({
      data: { enabled: true },
      isLoading: false,
    } as never)
    vi.mocked(useUpdateBackupSettings).mockReturnValue(
      mockMutationHook() as never
    )
  })

  function renderDashboard() {
    return render(
      <QueryClientProvider client={queryClient}>
        <ServerDashboard />
      </QueryClientProvider>
    )
  }

  it('hides Players and Uptime when server is stopped', () => {
    renderDashboard()

    expect(screen.getByText('State')).toBeInTheDocument()
    expect(screen.getByText('IP')).toBeInTheDocument()
    expect(screen.queryByText('Players')).not.toBeInTheDocument()
    expect(screen.queryByText('Uptime')).not.toBeInTheDocument()
  })

  it('hides Players and Uptime while server is starting', () => {
    vi.mocked(useServers).mockReturnValue({
      data: [mockServerResponse(startingStatus)],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as never)

    renderDashboard()

    expect(screen.getByText('State')).toBeInTheDocument()
    expect(screen.getByText('IP')).toBeInTheDocument()
    expect(screen.queryByText('Players')).not.toBeInTheDocument()
    expect(screen.queryByText('Uptime')).not.toBeInTheDocument()
  })

  it('hides Players and Uptime while server is stopping', () => {
    vi.mocked(useServers).mockReturnValue({
      data: [mockServerResponse(stoppingStatus)],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as never)

    renderDashboard()

    expect(screen.getByText('State')).toBeInTheDocument()
    expect(screen.getByText('IP')).toBeInTheDocument()
    expect(screen.queryByText('Players')).not.toBeInTheDocument()
    expect(screen.queryByText('Uptime')).not.toBeInTheDocument()
  })

  it('shows Players and Uptime when server is running', () => {
    vi.mocked(useServers).mockReturnValue({
      data: [mockServerResponse(runningStatus)],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as never)

    renderDashboard()

    expect(screen.getByText('State')).toBeInTheDocument()
    expect(screen.getByText('IP')).toBeInTheDocument()
    expect(screen.getByText('Players')).toBeInTheDocument()
    expect(screen.getByText('5/20')).toBeInTheDocument()
    expect(screen.getByText('Uptime')).toBeInTheDocument()
    expect(screen.getByText('3h 45m')).toBeInTheDocument()
  })
})

describe('ServerDashboard create server button', () => {
  let queryClient: QueryClient

  beforeEach(() => {
    vi.clearAllMocks()
    queryClient = createQueryClient()

    vi.mocked(useServerStatus).mockReturnValue({ data: undefined } as never)
    vi.mocked(useServerProvisioning).mockReturnValue({
      data: undefined,
    } as never)
    vi.mocked(useStartServer).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useStopServer).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useUpdateServer).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useUpdateAgent).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useDeleteServer).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useWhitelist).mockReturnValue({ data: undefined } as never)
    vi.mocked(useAddPlayer).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useRemovePlayer).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useToggleWhitelist).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useScheduleShutdown).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useCancelScheduledShutdown).mockReturnValue(
      mockMutationHook() as never
    )
    vi.mocked(useBackupSettings).mockReturnValue({
      data: { enabled: true },
      isLoading: false,
    } as never)
    vi.mocked(useUpdateBackupSettings).mockReturnValue(
      mockMutationHook() as never
    )
  })

  function renderDashboard() {
    return render(
      <QueryClientProvider client={queryClient}>
        <ServerDashboard />
      </QueryClientProvider>
    )
  }

  it('shows the Create Server button when servers already exist', () => {
    vi.mocked(useServers).mockReturnValue({
      data: [mockServerResponse(stoppedStatus)],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as never)

    renderDashboard()

    expect(
      screen.getByRole('button', { name: /create server/i })
    ).toBeInTheDocument()
  })

  it('navigates to the setup wizard when the Create Server button is clicked', async () => {
    vi.mocked(useServers).mockReturnValue({
      data: [mockServerResponse(stoppedStatus)],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as never)

    renderDashboard()

    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: /create server/i }))

    expect(mockNavigate).toHaveBeenCalledWith('/servers/new')
  })

  it('shows exactly one Create Server button in the empty state', () => {
    vi.mocked(useServers).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as never)

    renderDashboard()

    expect(
      screen.getAllByRole('button', { name: /create server/i })
    ).toHaveLength(1)
  })
})

describe('ServerDashboard backup settings', () => {
  let queryClient: QueryClient

  beforeEach(() => {
    vi.clearAllMocks()
    queryClient = createQueryClient()

    vi.mocked(useServers).mockReturnValue({
      data: [mockServerResponse(stoppedStatus)],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as never)
    vi.mocked(useServerStatus).mockReturnValue({ data: undefined } as never)
    vi.mocked(useServerProvisioning).mockReturnValue({
      data: undefined,
    } as never)
    vi.mocked(useStartServer).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useStopServer).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useUpdateServer).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useUpdateAgent).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useDeleteServer).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useWhitelist).mockReturnValue({ data: undefined } as never)
    vi.mocked(useAddPlayer).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useRemovePlayer).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useToggleWhitelist).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useScheduleShutdown).mockReturnValue(mockMutationHook() as never)
    vi.mocked(useCancelScheduledShutdown).mockReturnValue(
      mockMutationHook() as never
    )
    vi.mocked(useBackupSettings).mockReturnValue({
      data: { enabled: true },
      isLoading: false,
    } as never)
    vi.mocked(useUpdateBackupSettings).mockReturnValue(
      mockMutationHook() as never
    )
  })

  function renderDashboard() {
    return render(
      <QueryClientProvider client={queryClient}>
        <ServerDashboard />
      </QueryClientProvider>
    )
  }

  it('shows the backup settings inside the settings modal', async () => {
    renderDashboard()

    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: /update/i }))
    await user.click(screen.getByRole('tab', { name: 'Backup' }))

    expect(screen.getByRole('tab', { name: 'Backup' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    expect(screen.getByText('Scheduled backups enabled')).toBeInTheDocument()
  })

  it('preloads current settings into the backup form', async () => {
    vi.mocked(useBackupSettings).mockReturnValue({
      data: { enabled: true, backupIntervalHours: 6, keep: 3, keepUnit: 'daily' },
      isLoading: false,
    } as never)

    renderDashboard()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: /update/i }))
    await user.click(screen.getByRole('tab', { name: 'Backup' }))

    expect(screen.getByLabelText('Backup interval (hours)')).toHaveValue(6)
    expect(screen.getByLabelText('Retention policy')).toHaveValue(3)
  })

  it('saves settings and navigates to provisioning on success', async () => {
    vi.mocked(useBackupSettings).mockReturnValue({
      data: { enabled: true, backupIntervalHours: 1 },
      isLoading: false,
    } as never)
    const mockMutate = vi.fn(
      (_settings: unknown, opts?: { onSuccess?: () => void }) =>
        opts?.onSuccess?.()
    )
    vi.mocked(useUpdateBackupSettings).mockReturnValue({
      mutate: mockMutate,
      isPending: false,
    } as never)

    renderDashboard()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: /update/i }))
    await user.click(screen.getByRole('tab', { name: 'Backup' }))
    await user.click(screen.getByRole('button', { name: /save/i }))

    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({ enabled: true, backupIntervalHours: 1 }),
      expect.anything()
    )
    expect(mockNavigate).toHaveBeenCalledWith('/servers/srv1/provisioning', {
      state: { serverName: 'Survival' },
    })
  })
})
