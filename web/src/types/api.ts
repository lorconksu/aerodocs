export type Role = 'admin' | 'viewer'

export interface User {
  id: string
  username: string
  email: string
  role: Role
  totp_enabled: boolean
  created_at: string
  updated_at: string
}

export interface AuthStatusResponse {
  initialized: boolean
}

export interface RegisterRequest {
  username: string
  email: string
  password: string
}

export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  totp_token?: string
  setup_token?: string
  requires_totp_setup?: boolean
}

export interface LoginTOTPRequest {
  totp_token: string
  code: string
}

export interface AuthResponse {
  access_token: string
  refresh_token: string
  user: User
}

export interface TokenPair {
  access_token: string
  refresh_token: string
}

export interface TOTPSetupResponse {
  secret: string
  qr_url: string
}

export interface TOTPEnableRequest {
  code: string
}

export interface CreateUserRequest {
  username: string
  email: string
  role: Role
}

export interface CreateUserResponse {
  user: User
  temporary_password: string
}

export interface AuditEntry {
  id: string
  user_id: string | null
  action: string
  target: string | null
  detail: string | null
  ip_address: string | null
  created_at: string
}

export interface AuditLogResponse {
  entries: AuditEntry[]
  total: number
  limit: number
  offset: number
}

export interface ChangePasswordRequest {
  current_password: string
  new_password: string
}

export interface UpdateRoleRequest {
  role: Role
}

export interface TOTPDisableRequest {
  user_id: string
  admin_totp_code: string
}

export interface ApiError {
  error: string
}
