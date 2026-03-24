/**
 * Authenticated user information
 */
export interface AuthUser {
  email: string;
}

/**
 * Response from /api/auth/me endpoint
 */
export interface AuthMeResponse {
  authenticated: boolean;
  email?: string;
}
