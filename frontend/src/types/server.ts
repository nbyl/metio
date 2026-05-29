/**
 * Server state values matching Go db.ServerState
 */
export type ServerState = 'STOPPED' | 'STARTING' | 'RUNNING' | 'STOPPING';

/**
 * Server status from backend /api/server/status endpoint
 * Matches Go handlers.ServerStatus struct
 */
export interface ServerStatus {
  status: ServerState;
  players: number;
  maxPlayers: number;
  uptime: string;
  version: string;
  ip: string;
  whitelistEnabled: boolean;
  scheduledShutdown?: string; // RFC3339 datetime or undefined
}

/**
 * Response from /api/server/start and /api/server/stop endpoints
 * Matches Go handlers.ServerActionResponse struct
 */
export interface ServerActionResponse {
  success: boolean;
  state: ServerState;
}

/**
 * Request for scheduling a shutdown
 */
export interface ScheduleShutdownRequest {
  shutdownTime: string; // RFC3339 format
}

/**
 * Response from /api/server/shutdown/schedule endpoints
 */
export interface ScheduleShutdownResponse {
  success: boolean;
  scheduledShutdown?: string; // RFC3339 datetime or undefined
}

/**
 * Error response from API endpoints
 */
export interface APIError {
  error: string;
}

/**
 * Application configuration from backend /api/config endpoint
 */
export interface AppConfig {
  gcpProject: string;
  instanceName: string;
}

// --- Multi-server types ---

/**
 * Shutdown schedule input/output matching backend ShutdownScheduleInput
 */
export interface ShutdownScheduleInput {
  enabled: boolean;
  time?: string;
  timezone?: string;
}

/**
 * Server configuration from backend ServerConfigJSON
 */
export interface ServerConfig {
  name: string;
  region: string;
  zone: string;
  machineType: string;
  minecraftVersion: string;
  diskSizeGB: number;
  infraVersion?: number;
  deployedByControllerVersion?: string;
  shutdownSchedule?: ShutdownScheduleInput;
  createdAt: string;
  updatedAt: string;
}

/**
 * Server response from /api/servers and /api/servers/{id}
 * Matches Go handlers.ServerResponse
 */
export interface ServerResponse {
  id: string;
  config: ServerConfig;
  status?: StatusResponse;
  currentInfraVersion: number;
  outdated: boolean;
}

/**
 * Status details in server response
 */
export interface StatusResponse {
  serverState: ServerState;
  players: number;
  maxPlayers: number;
  uptime: string;
  version: string;
  instanceIP: string;
  scheduledShutdown?: string;
}

/**
 * Request body for updating a server
 * All fields are optional (partial update)
 */
export interface UpdateServerRequest {
  name?: string;
  region?: string;
  zone?: string;
  machineType?: string;
  minecraftVersion?: string;
  diskSizeGB?: number;
  shutdownSchedule?: ShutdownScheduleInput;
}

/**
 * Provisioning step from backend StepResponse
 */
export interface ProvisioningStep {
  name: string;
  status: string;
  message: string;
  timestamp: string;
}

/**
 * Provisioning status from /api/servers/{id}/provisioning
 */
export interface ProvisioningStatusResponse {
  id: string;
  operation: string;
  state: string;
  currentStep: string;
  progress: number;
  steps: ProvisioningStep[];
  error?: string;
  startedAt: string;
  completedAt?: string;
  outputs?: Record<string, string>;
}
