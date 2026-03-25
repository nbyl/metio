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
}

/**
 * Application configuration from backend /api/config endpoint
 */
export interface AppConfig {
  gcpProject: string;
  instanceName: string;
}
