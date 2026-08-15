/**
 * Server state values matching Go db.ServerState
 */
export type ServerState = 'STOPPED' | 'STARTING' | 'RUNNING' | 'STOPPING'

/**
 * Error response from API endpoints
 */
export interface APIError {
  error: string
}

// --- Multi-server types ---

/**
 * Shutdown schedule input/output matching backend ShutdownScheduleInput
 */
export interface ShutdownScheduleInput {
  enabled: boolean
  time?: string
  timezone?: string
}

/**
 * Server configuration from backend ServerConfigJSON
 */
export interface ServerConfig {
  name: string
  region: string
  zone: string
  machineType: string
  minecraftVersion: string
  diskSizeGB: number
  infraVersion?: number
  deployedByControllerVersion?: string
  shutdownSchedule?: ShutdownScheduleInput
  createdAt: string
  updatedAt: string
}

/**
 * Server response from /api/servers and /api/servers/{id}
 * Matches Go handlers.ServerResponse
 */
export interface ServerResponse {
  id: string
  config: ServerConfig
  status?: StatusResponse
  currentInfraVersion: number
  outdated: boolean
  outdatedMachineAgent?: boolean
  controllerVersion?: string
}

/**
 * Players JSON matching Go handlers.PlayersJSON
 */
export interface PlayersJSON {
  current: number
  max: number
}

/**
 * Status details in server response
 */
export interface StatusResponse {
  serverState: ServerState
  players: PlayersJSON
  uptime: string
  version: string
  instanceIP: string
  timestamp?: string
  whitelistEnabled?: boolean
  scheduledShutdown?: string
  agentVersion?: string
}

/**
 * Per-server backup override matching backend BackupSettings.
 * Empty/zero values fall back to the deployment defaults.
 */
export interface BackupSettings {
  enabled: boolean
  backupIntervalHours?: number
  keep?: number
  keepUnit?: string
}

/**
 * Request body for updating a server
 * All fields are optional (partial update)
 */
export interface UpdateServerRequest {
  name?: string
  region?: string
  zone?: string
  machineType?: string
  minecraftVersion?: string
  diskSizeGB?: number
  shutdownSchedule?: ShutdownScheduleInput
}

/**
 * Provisioning step from backend StepResponse
 */
export interface CreateServerRequest {
  name: string
  region: string
  zone: string
  machineType: string
  minecraftVersion: string
  diskSizeGB?: number
  shutdownSchedule?: ShutdownScheduleInput
}

export interface MachineTypeOption {
  id: string
  vcpus: number
  memoryGB: number
}

export interface RegionOption {
  id: string
  zones: string[]
}

export interface SetupOptions {
  machineTypes: MachineTypeOption[]
  regions: RegionOption[]
  minecraftVersions: string[]
  controllerVersion?: string
}

export interface ProvisioningStep {
  name: string
  status: string
  message: string
  timestamp: string
}

/**
 * Provisioning status from /api/servers/{id}/provisioning
 */
export interface ProvisioningStatusResponse {
  id: string
  operation: string
  state: string
  currentStep: string
  progress: number
  steps: ProvisioningStep[]
  error?: string
  startedAt: string
  completedAt?: string
  outputs?: Record<string, string>
}
