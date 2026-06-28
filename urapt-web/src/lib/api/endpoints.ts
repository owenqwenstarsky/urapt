import { http } from "./client";
import type {
  APIToken,
  AddMemberRequest,
  Architecture,
  AuthResponse,
  Component,
  CreateNamedRequest,
  CreateRepoRequest,
  CreateTokenRequest,
  Distribution,
  ListParams,
  ListResponse,
  LoginRequest,
  Package,
  RegisterRequest,
  Repository,
  RepositoryMember,
  ServerInfo,
  UpdateMemberRequest,
  UpdateRepoRequest,
  UpdateUserRequest,
  User,
} from "./types";

// --- server / setup ---

export const getServerInfo = () => http.get<ServerInfo>("/server/info");

export const getServerPubkey = () =>
  http
    .raw("/server/pubkey", { headers: { Accept: "application/pgp-keys" } })
    .then((r) => r.text());

// --- auth ---

export const register = (body: RegisterRequest) =>
  http.post<AuthResponse>("/auth/register", body);

export const login = (body: LoginRequest) => http.post<AuthResponse>("/auth/login", body);

export const logout = () => http.post<void>("/auth/logout");

export const getMe = () => http.get<User>("/me");

// --- tokens ---

export const listTokens = () => http.get<ListResponse<APIToken>>("/me/tokens");

export const createToken = (body: CreateTokenRequest) =>
  http.post<APIToken>("/me/tokens", body);

export const revokeToken = (id: string) => http.del<void>(`/me/tokens/${id}`);

// --- repositories ---

export const listRepositories = (params?: ListParams) =>
  http.get<ListResponse<Repository>>("/repositories", params);

export const createRepository = (body: CreateRepoRequest) =>
  http.post<Repository>("/repositories", body);

export const getRepository = (name: string) =>
  http.get<Repository>(`/repositories/${encodeURIComponent(name)}`);

export const updateRepository = (name: string, body: UpdateRepoRequest) =>
  http.patch<Repository>(`/repositories/${encodeURIComponent(name)}`, body);

export const deleteRepository = (name: string) =>
  http.del<void>(`/repositories/${encodeURIComponent(name)}`);

export const getRepoPubkey = (name: string) =>
  http
    .raw(`/repositories/${encodeURIComponent(name)}/pubkey`, {
      headers: { Accept: "application/pgp-keys" },
    })
    .then((r) => r.text());

// --- members ---

export const listMembers = (repo: string) =>
  http.get<RepositoryMember[]>(`/repositories/${encodeURIComponent(repo)}/members`);

export const addMember = (repo: string, body: AddMemberRequest) =>
  http.post<RepositoryMember>(`/repositories/${encodeURIComponent(repo)}/members`, body);

export const updateMember = (repo: string, username: string, body: UpdateMemberRequest) =>
  http.patch<RepositoryMember>(
    `/repositories/${encodeURIComponent(repo)}/members/${encodeURIComponent(username)}`,
    body,
  );

export const removeMember = (repo: string, username: string) =>
  http.del<void>(
    `/repositories/${encodeURIComponent(repo)}/members/${encodeURIComponent(username)}`,
  );

// --- distributions ---

export const listDistributions = (repo: string) =>
  http.get<Distribution[]>(`/repositories/${encodeURIComponent(repo)}/distributions`);

export const createDistribution = (repo: string, body: CreateNamedRequest) =>
  http.post<Distribution>(
    `/repositories/${encodeURIComponent(repo)}/distributions`,
    body,
  );

export const deleteDistribution = (repo: string, dist: string) =>
  http.del<void>(
    `/repositories/${encodeURIComponent(repo)}/distributions/${encodeURIComponent(dist)}`,
  );

// --- components ---

export const listComponents = (repo: string, dist: string) =>
  http.get<Component[]>(
    `/repositories/${encodeURIComponent(repo)}/distributions/${encodeURIComponent(dist)}/components`,
  );

export const createComponent = (repo: string, dist: string, body: CreateNamedRequest) =>
  http.post<Component>(
    `/repositories/${encodeURIComponent(repo)}/distributions/${encodeURIComponent(dist)}/components`,
    body,
  );

export const deleteComponent = (repo: string, dist: string, comp: string) =>
  http.del<void>(
    `/repositories/${encodeURIComponent(repo)}/distributions/${encodeURIComponent(dist)}/components/${encodeURIComponent(comp)}`,
  );

// --- architectures ---

export const listArchitectures = (repo: string, dist: string) =>
  http.get<Architecture[]>(
    `/repositories/${encodeURIComponent(repo)}/distributions/${encodeURIComponent(dist)}/architectures`,
  );

export const createArchitecture = (
  repo: string,
  dist: string,
  body: CreateNamedRequest,
) =>
  http.post<Architecture>(
    `/repositories/${encodeURIComponent(repo)}/distributions/${encodeURIComponent(dist)}/architectures`,
    body,
  );

export const deleteArchitecture = (repo: string, dist: string, arch: string) =>
  http.del<void>(
    `/repositories/${encodeURIComponent(repo)}/distributions/${encodeURIComponent(dist)}/architectures/${encodeURIComponent(arch)}`,
  );

// --- packages ---

export const listPackages = (repo: string, dist: string, params?: ListParams) =>
  http.get<ListResponse<Package>>(
    `/repositories/${encodeURIComponent(repo)}/distributions/${encodeURIComponent(dist)}/packages`,
    params,
  );

export const pushPackage = (repo: string, dist: string, file: File, component: string) =>
  http.postForm<Package>(
    `/repositories/${encodeURIComponent(repo)}/distributions/${encodeURIComponent(dist)}/packages`,
    { component },
    file,
  );

export const getPackage = (repo: string, id: string) =>
  http.get<Package>(`/repositories/${encodeURIComponent(repo)}/packages/${id}`);

export const getPackageFile = (repo: string, id: string) =>
  http
    .raw(`/repositories/${encodeURIComponent(repo)}/packages/${id}/file`)
    .then((r) => r.blob());

export const deletePackage = (repo: string, id: string) =>
  http.del<void>(`/repositories/${encodeURIComponent(repo)}/packages/${id}`);

// --- users (admin) ---

export const listUsers = (params?: ListParams) =>
  http.get<ListResponse<User>>("/users", params);

export const getUser = (id: string) => http.get<User>(`/users/${id}`);

export const updateUser = (id: string, body: UpdateUserRequest) =>
  http.patch<User>(`/users/${id}`, body);

export const deleteUser = (id: string) => http.del<void>(`/users/${id}`);
