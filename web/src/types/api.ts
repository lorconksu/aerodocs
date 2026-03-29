export type Role = 'admin' | 'viewer'

export interface User {
  id: string
  username: string
  email: string
  role: Role
  totp_enabled: boolean
  avatar: string | null
  created_at: string
  updated_at: string
}

export interface AuthStatusResponse {
  initialized: boolean
  version: string
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

export type ServerStatus = 'pending' | 'online' | 'offline'

export interface Server {
  id: string
  name: string
  hostname: string | null
  ip_address: string | null
  os: string | null
  status: ServerStatus
  agent_version: string | null
  labels: string
  last_seen_at: string | null
  created_at: string
  updated_at: string
}

export interface ServerListResponse {
  servers: Server[]
  total: number
  limit: number
  offset: number
}

export interface CreateServerRequest {
  name: string
  labels?: string
}

export interface CreateServerResponse {
  server: Server
  registration_token: string
  install_command: string
}

export interface BatchDeleteRequest {
  ids: string[]
}

// File tree types (sub-project 4)
export interface FileNode {
  name: string
  path: string
  is_dir: boolean
  size: number
  readable: boolean
}

export interface FileListResponse {
  files: FileNode[]
}

export interface FileReadResponse {
  data: string  // base64-encoded
  total_size: number
  mime_type: string
}

export interface PathPermission {
  id: string
  user_id: string
  username: string
  server_id: string
  path: string
  created_at: string
}

export interface PathListResponse {
  paths: PathPermission[]
}

export interface CreatePathRequest {
  user_id: string
  path: string
}
