import type { Timestamp } from 'firebase/firestore';

/**
 * Server state values matching Go db.ServerState
 */
export type ServerState = 'STOPPED' | 'STARTING' | 'RUNNING' | 'STOPPING';

/**
 * Player count information matching Go db.Players
 */
export interface Players {
  current: number;
  max: number;
}

/**
 * Server status document from Firestore
 * Path: instances/{instanceName}/data/status
 * Matches Go db.Status struct
 */
export interface ServerStatus {
  server_state: ServerState;
  players: Players;
  uptime: string;
  instance_ip: string;
  timestamp: Timestamp;
}

/**
 * Application configuration from backend /api/config endpoint
 */
export interface AppConfig {
  gcpProject: string;
  firestoreDatabase: string;
  instanceName: string;
}

/**
 * Firebase token response from /api/auth/firebase-token
 */
export interface FirebaseTokenResponse {
  token: string;
}
