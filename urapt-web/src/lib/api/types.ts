// DTOs mirroring urapt's shared/models and shared/api packages.
// Field names match the server's JSON tags exactly.

export type Visibility = "public" | "private";

export type Access = "read" | "write" | "read-write" | "admin";

export interface User {
  id: string;
  username: string;
  is_admin: boolean;
  created_at: string;
  updated_at: string;
}

export interface APIToken {
  id: string;
  user_id: string;
  name: string;
  prefix: string;
  /** Plaintext token. Only present on creation/login. */
  token?: string;
  created_at: string;
  last_used_at?: string | null;
  revoked_at?: string | null;
}

export interface Repository {
  id: string;
  name: string;
  owner_user_id: string;
  owner?: User;
  visibility: Visibility;
  description: string;
  created_at: string;
  updated_at: string;
}

export interface RepositoryMember {
  repository_id: string;
  user_id: string;
  user?: User;
  access: Access;
  created_at: string;
}

export interface Distribution {
  id: string;
  repository_id: string;
  name: string;
  created_at: string;
}

export interface Component {
  id: string;
  distribution_id: string;
  name: string;
  created_at: string;
}

export interface Architecture {
  id: string;
  distribution_id: string;
  name: string;
  created_at: string;
}

export interface Package {
  id: string;
  repository_id: string;
  distribution_id: string;
  component_id: string;
  name: string;
  version: string;
  architecture: string;
  source?: string;
  maintainer?: string;
  priority?: string;
  section?: string;
  origin?: string;
  homepage?: string;
  description?: string;
  description_md5?: string;
  depends?: string;
  pre_depends?: string;
  recommends?: string;
  suggests?: string;
  conflicts?: string;
  breaks?: string;
  provides?: string;
  replaces?: string;
  enhances?: string;
  installed_size?: number;
  essential?: string;
  built_using?: string;
  tag?: string;
  raw_control: string;
  filename: string;
  pool_path: string;
  size: number;
  md5sum: string;
  sha1: string;
  sha256: string;
  uploaded_by_user_id: string;
  created_at: string;
}

// --- server / setup ---

export interface ServerInfo {
  version: string;
  needs_setup: boolean;
  default_key_fingerprint: string;
  open_registration: boolean;
}

// --- auth DTOs ---

export interface RegisterRequest {
  username: string;
  password: string;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface AuthResponse {
  user: User;
  token: string;
}

export interface CreateTokenRequest {
  name: string;
}

// --- repository DTOs ---

export interface CreateRepoRequest {
  name: string;
  visibility: Visibility;
  description?: string;
}

export interface UpdateRepoRequest {
  name?: string;
  visibility?: Visibility;
  description?: string;
}

export interface AddMemberRequest {
  username: string;
  access: Access;
}

export interface UpdateMemberRequest {
  access: Access;
}

// --- structure DTOs ---

export interface CreateNamedRequest {
  name: string;
}

// --- users (admin) ---

export interface UpdateUserRequest {
  is_admin?: boolean;
}

// --- list envelope ---

export interface ListResponse<T> {
  items: T[];
  page: number;
  per_page: number;
  total: number;
}

export interface ListParams extends Record<string, unknown> {
  page?: number;
  per_page?: number;
}

// --- error envelope ---

export type ErrorCode =
  | "bad_request"
  | "unauthorized"
  | "forbidden"
  | "not_found"
  | "conflict"
  | "payload_too_large"
  | "internal";

export interface ApiErrorBody {
  error: {
    code: ErrorCode | string;
    message: string;
    details?: unknown;
  };
}
