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
