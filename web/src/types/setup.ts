export interface APICheckResult {
  enabled: boolean;
}

export interface PermissionCheckResult {
  granted: boolean;
}

export interface Fix {
  type: string;
  api?: string;
  role?: string;
  permission?: string;
  consoleUrl: string;
}

export interface ValidationResult {
  valid: boolean;
  apis: Record<string, APICheckResult>;
  permissions: Record<string, PermissionCheckResult>;
  fixes: Fix[];
  checkedAt: string;
}

export interface SetupStatus {
  initialized: boolean;
  serverCount: number;
  checks: ValidationResult;
}
