/**
 * Whitelist player from backend /api/server/whitelist endpoint
 */
export interface WhitelistPlayer {
  username: string;
  uuid: string;
  addedAt: string;
  addedBy: string;
}

/**
 * Whitelist response from backend /api/server/whitelist endpoint
 */
export interface WhitelistResponse {
  enabled: boolean;
  players: WhitelistPlayer[];
}

/**
 * Request body for POST /api/server/whitelist
 */
export interface AddPlayerRequest {
  username: string;
}

/**
 * Request body for PUT /api/server/whitelist/enabled
 */
export interface SetEnabledRequest {
  enabled: boolean;
}
