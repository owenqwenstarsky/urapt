# urapt — Implementation Plan

A self-hostable APT repository server with a companion CLI for pushing and
managing Debian/Ubuntu `.deb` packages under your logged-in user.

> Status: Planning. This document is the authoritative blueprint for
> implementation. Decisions captured here are final unless explicitly marked
> "open" or "future".

---

## 1. Goals & scope

### In scope (v1)
- A self-hostable **server** that:
  - Exposes a **REST API** for the CLI to manage repositories, users,
    distributions, components, architectures, and packages.
  - Exposes a **special APT endpoint** that the standard `apt` client speaks to
    (serves `dists/.../{Release,InRelease,Release.gpg}`, `Packages` indices, and
    the `pool/` `.deb` files).
  - Stores **only uploaded `.deb` files on the filesystem** (`store/packages/`).
    Everything else lives in a **SQLite database** (`store/database/sqlite.db`).
  - Generates APT indices **on demand from the database** (indices are never
    persisted to disk).
  - Signs `Release`/`InRelease` with a **server-managed GPG key**.
- A **CLI** (`urapt`) that:
  - Logs you in (username/password → API token stored locally).
  - Pushes `.deb` packages to a repository/distribution/component under your
    identity.
  - Pulls, lists, and deletes packages.
  - Manages repositories, members, distributions, components, architectures.
  - Emits the `sources.list` line + pubkey + `auth.conf` for client setup.
- A **shared utilities component** (`shared/`) used by both server and CLI:
    config, DB, models, GPG, `.deb` parsing, APT index generation, crypto, and
    the typed REST API client + DTOs.

### Out of scope (v1, listed as future)
- Source package (`deb-src`) hosting.
- AppStream / `dep11` metadata.
- `Acquire-By-Hash` indices.
- Web UI.
- External OAuth/OIDC auth.
- Per-repository GPG keys (v1 uses one server-wide key).
- OS keychain credential storage (v1 stores token in a 0600 config file).
- Multi-arch `Contents` indexes.

---

## 2. Confirmed decisions

| Decision | Choice |
|---|---|
| Language / stack | **Go** — single static binary for both server and CLI |
| GPG signing | **Server-managed key**: server generates & stores its own key, signs `Release`/`InRelease` automatically; admin exports the pubkey for clients |
| Auth (CLI ↔ API) | **API tokens**: user logs in with username/password once, gets a token stored locally; CLI sends token in `Authorization: Bearer` |
| Bootstrap admin | **First user to register becomes admin** |
| Repo model | **Full model**: repositories contain suites/distributions (stable, testing…), each suite has components (main, contrib…) and architectures (amd64, arm64, all) |
| Access policy | **Public repo**: APT read is unauthenticated, REST write requires auth. **Private repo**: APT read *and* REST write require auth (APT read via HTTP Basic with token) |
| Permissions | **Repository-scoped**. Creator = owner with full read/write. Owner can grant `read` / `write` / `read-write` / `admin` to other users. Only users with access can push. Server admins can manage everything |

---

## 3. Architecture overview

Three components, one Go module (`urapt`):

```
            +-------------------+        +-------------------+
            |   urapt (CLI)     |        |  urapt-server     |
            |   cmd/urapt       |        |  cmd/urapt-server |
            +---------+---------+        +---------+---------+
                      |                            |
                      |  uses shared/              |  uses shared/
                      v                            v
            +---------------------------------------------+
            |                 shared/                     |
            |  config | db | models | gpg | deb | apt |   |
            |  crypto | api(types) | apiclient(HTTP) |    |
            +---------------------------------------------+
                      |                            |
                      +------> SQLite <-----+      |
                                             |      |
                                      store/database/sqlite.db
                                      store/packages/*.deb (files only)
```

- The **CLI** never touches the DB or filesystem directly; it only talks to the
  server's REST API via `shared/apiclient`.
- The **server** owns the DB and the `store/` directory.
- The **shared** component contains pure libraries and the API contract. No
  component imports upward (no `shared` → `server` or `shared` → `cli`).

---

## 4. Project structure

```
urapt/
  go.mod                         module path: urapt
  go.sum
  README.md
  PLAN.md
  .gitignore                     ignores store/, *.db, built binaries
  Dockerfile                     multi-stage build for urapt-server
  docker-compose.yml             example self-hosted deployment
  Makefile                       build/test/lint targets

  cmd/
    urapt-server/main.go         server entrypoint: load config, wire app, run
    urapt/        main.go        CLI entrypoint: execute cobra root

  shared/                        ---- shared utilities component ----
    config/                      config structs + file/env/flag loading
    db/                          sqlite open (WAL), migrations runner, query helpers
    models/                      domain types (User, Repository, Package, …)
    api/                         REST DTOs + request/response shapes (the contract)
    apiclient/                   typed HTTP client used by the CLI
    crypto/                      password hashing (bcrypt), token gen + hashing
    gpg/                         key generation, clearsign, detached sign, export
    deb/                         .deb (ar) unpack, control.tar parse, control fields
    apt/                         index generation: Packages, Release, InRelease, Release.gpg
    httputil/                    JSON helpers, error rendering, bearer/basic parsing
    log/                         structured logging wrapper
    version/                     build version info

  server/                        ---- server component ----
    app/                         wiring: dependencies, store paths, startup, key init
    restapi/                     REST handlers + routes (chi)
    aptrepo/                     APT endpoint handlers + index cache
    middleware/                  auth (bearer), basic-auth (for private APT), logging, recover
    auth/                        token resolution, permission checks, session/identity

  cli/                           ---- CLI component ----
    commands/                    cobra commands (login, push, repo, …)
    config/                      local config + token store (~/.config/urapt)
    output/                      table/JSON formatting helpers
    interact/                    prompts (password, confirm)

  migrations/                    numbered *.sql files, embedded via go:embed
    0001_init.sql

  store/                         runtime data (gitignored, created at runtime)
    database/sqlite.db
    packages/<sha256>.deb
```

### Import rules
- `shared/**` imports only stdlib + 3rd-party libs (never `server/` or `cli/`).
- `server/**` imports `shared/**`.
- `cli/**` imports `shared/**`.
- `cmd/**` are thin `main` packages that wire the relevant component.

---

## 5. Technology choices (libraries)

| Concern | Choice | Why |
|---|---|---|
| SQLite driver | `modernc.org/sqlite` | Pure-Go, no CGO → easy static binaries & cross-compile |
| HTTP router | `go-chi/chi/v5` | Lightweight, middleware-friendly, stdlib-compatible |
| Migrations | `embed` + tiny runner in `shared/db` | No extra tooling; runs on startup |
| Password hashing | `golang.org/x/crypto/bcrypt` | Simple, well-understood |
| Token generation | `crypto/rand` 32B → base64url; store SHA-256 | Standard, revocable |
| GPG / OpenPGP | `github.com/ProtonMail/go-crypto/openpgp` | Maintained OpenPGP in pure Go; keygen, clearsign, detached sig |
| `.deb` ar archive | `pault.ag/go/debian/deb` (+ `blakesmith/ar` fallback) | Debian-aware deb reader |
| Control parsing | `pault.ag/go/debian/control` | RFC822 control field parsing |
| Compression (xz) | `github.com/ulikunitz/xz` | For `control.tar.xz` / `data.tar.xz` |
| Compression (zstd) | `github.com/klauspost/compress/zstd` | For `control.tar.zst` (newer debs) |
| CLI framework | `github.com/spf13/cobra` | Widely known, subcommands, flags |
| Config | `github.com/BurntSushi/toml` + env + flags | TOML file + env overrides + flag overrides |
| Logging | `log/slog` (stdlib) | Structured logging, no dep |
| Validation | `github.com/go-playground/validator/v10` | DTO validation |
| Testing | `testing` + `github.com/stretchr/testify` | Unit + integration |

Go version: **1.22+** (for `log/slog`, enhanced `ServeMux` if needed).

---

## 6. Data model (SQLite schema)

All IDs are text UUIDv4. Timestamps are ISO-8601 UTC text. Booleans are INTEGER
0/1. SQLite in **WAL** mode, `busy_timeout=5000`, `foreign_keys=ON`.

### `users`
| col | type | notes |
|---|---|---|
| id | TEXT PK | uuid |
| username | TEXT UNIQUE NOT NULL | case-insensitive; store lowercased + original |
| password_hash | TEXT NOT NULL | bcrypt |
| is_admin | INTEGER NOT NULL DEFAULT 0 | 1 for admins |
| created_at | TEXT NOT NULL | |
| updated_at | TEXT NOT NULL | |

### `api_tokens`
| col | type | notes |
|---|---|---|
| id | TEXT PK | uuid |
| user_id | TEXT FK→users.id | |
| name | TEXT NOT NULL | user label e.g. "laptop" |
| prefix | TEXT NOT NULL | first 8 chars of token (for identification) |
| token_hash | TEXT UNIQUE NOT NULL | SHA-256 of full token |
| created_at | TEXT NOT NULL | |
| last_used_at | TEXT | nullable |
| revoked_at | TEXT | nullable; if set, invalid |

### `repositories`
| col | type | notes |
|---|---|---|
| id | TEXT PK | |
| name | TEXT UNIQUE NOT NULL | URL-safe `[a-z0-9-]+`, lowercase |
| owner_user_id | TEXT FK→users.id | implicit full access |
| visibility | TEXT NOT NULL | `public` \| `private` |
| description | TEXT | nullable |
| created_at | TEXT NOT NULL | |
| updated_at | TEXT NOT NULL | |

> v1 uses a single server-wide signing key, so no `signing_key_id` column. A
> `gpg_keys` table still holds that one key (see below).

### `repository_members`
| col | type | notes |
|---|---|---|
| repository_id | TEXT FK→repositories.id | |
| user_id | TEXT FK→users.id | |
| access | TEXT NOT NULL | `read` \| `write` \| `read-write` \| `admin` |
| created_at | TEXT NOT NULL | |
| PK | (repository_id, user_id) | |

- `read`: can read/download (private repos) and list.
- `write`: can push packages (no read).
- `read-write`: both.
- `admin`: both + manage members. Owner is implicitly `admin`.

### `distributions` (suites)
| col | type | notes |
|---|---|---|
| id | TEXT PK | |
| repository_id | TEXT FK→repositories.id | |
| name | TEXT NOT NULL | e.g. `stable`, `testing`, `jammy` |
| created_at | TEXT NOT NULL | |
| UNIQUE | (repository_id, name) | |

### `components`
| col | type | notes |
|---|---|---|
| id | TEXT PK | |
| distribution_id | TEXT FK→distributions.id | |
| name | TEXT NOT NULL | e.g. `main`, `contrib` |
| created_at | TEXT NOT NULL | |
| UNIQUE | (distribution_id, name) | |

### `architectures`
| col | type | notes |
|---|---|---|
| id | TEXT PK | |
| distribution_id | TEXT FK→distributions.id | |
| name | TEXT NOT NULL | e.g. `amd64`, `arm64` (not `all` — `all` is implicit) |
| created_at | TEXT NOT NULL | |
| UNIQUE | (distribution_id, name) | |

> `all` is **not** stored as an architecture row. Architecture-independent
> packages (arch=`all`) are listed in **every** binary-`<arch>` Packages index.
> The Release `Architectures` field lists the configured architectures.

### `packages` (one row per uploaded .deb version)
| col | type | notes |
|---|---|---|
| id | TEXT PK | |
| repository_id | TEXT FK | |
| distribution_id | TEXT FK | |
| component_id | TEXT FK | |
| name | TEXT NOT NULL | from control `Package` |
| version | TEXT NOT NULL | from control `Version` |
| architecture | TEXT NOT NULL | from control `Architecture` (incl. `all`) |
| source | TEXT | from control `Source` (source pkg name) |
| maintainer | TEXT | |
| priority | TEXT | |
| section | TEXT | |
| origin | TEXT | |
| homepage | TEXT | |
| description | TEXT | full (extended) |
| description_md5 | TEXT | md5 of short description |
| depends | TEXT | |
| pre_depends | TEXT | |
| recommends | TEXT | |
| suggests | TEXT | |
| conflicts | TEXT | |
| breaks | TEXT | |
| provides | TEXT | |
| replaces | TEXT | |
| enhances | TEXT | |
| installed_size | INTEGER | kB |
| essential | TEXT | nullable |
| built_using | TEXT | nullable |
| tag | TEXT | nullable |
| raw_control | TEXT NOT NULL | full control stanza (re-emitted in index) |
| filename | TEXT NOT NULL | real file: `store/packages/<sha256>.deb` |
| pool_path | TEXT NOT NULL | virtual: `pool/<component>/<letter>/<src>/<file>.deb` |
| size | INTEGER NOT NULL | .deb file size in bytes |
| md5sum | TEXT NOT NULL | of .deb |
| sha1 | TEXT NOT NULL | of .deb |
| sha256 | TEXT NOT NULL | of .deb |
| uploaded_by_user_id | TEXT FK→users.id | |
| created_at | TEXT NOT NULL | |
| UNIQUE | (repository_id, distribution_id, component_id, name, version, architecture) | |

### `blobs` (content-addressed .deb files; reference counting + dedup)
| col | type | notes |
|---|---|---|
| sha256 | TEXT PK | |
| filename | TEXT NOT NULL | `store/packages/<sha256>.deb` |
| size | INTEGER NOT NULL | |
| ref_count | INTEGER NOT NULL DEFAULT 0 | |
| created_at | TEXT NOT NULL | |

> On push: compute sha256 → find-or-create blob (ref_count++) → insert package.
> On package delete: ref_count--; when 0, delete the file and the blob row.

### `gpg_keys` (server-managed signing keys)
| col | type | notes |
|---|---|---|
| id | TEXT PK | |
| fingerprint | TEXT UNIQUE NOT NULL | |
| user_id | TEXT NOT NULL | OpenPGP user-id string, e.g. `urapt-server <host>` |
| public_key_armored | TEXT NOT NULL | exported ASCII pubkey (served to clients) |
| private_key_armored | TEXT NOT NULL | armored private key (see security note) |
| is_default | INTEGER NOT NULL DEFAULT 0 | the one default server key |
| created_at | TEXT NOT NULL | |

> **Security note:** the private key is stored in the SQLite DB. v1 stores it
> armored without passphrase (acceptable for a self-hosted single-binary where
> the operator controls the DB file). Future: encrypt at rest with a passphrase
> from config/env (AES-GCM), and/or support per-repo keys. Document the
> tradeoff in README.

### `audit_log` (lightweight, best-effort)
| col | type | notes |
|---|---|---|
| id | TEXT PK | |
| user_id | TEXT | nullable (system events) |
| repository_id | TEXT | nullable |
| action | TEXT NOT NULL | e.g. `package.push`, `repo.create`, `member.add` |
| target | TEXT | human-readable subject |
| details | TEXT | JSON blob |
| created_at | TEXT NOT NULL | |

### `schema_migrations`
| col | type | notes |
|---|---|---|
| version | INTEGER PK | migration number |
| applied_at | TEXT NOT NULL | |

---

## 7. APT repository layout & serving

The APT endpoint base path is **`/apt/<repository>/`**. apt clients use:

```
deb https://<host>/apt/<repo>/ <suite> <component>[ <component>...]
```

### On-disk vs virtual
- **Virtual** paths (served, never on disk): everything under `dists/` and the
  `pool/` tree. Generated from the DB on demand.
- **Real** files on disk: only `store/packages/<sha256>.deb`.

### Routes served by the APT endpoint
| Route | Behavior |
|---|---|
| `GET /apt/:repo/dists/:suite/InRelease` | clearsigned Release (preferred by apt) |
| `GET /apt/:repo/dists/:suite/Release` | unsigned Release |
| `GET /apt/:repo/dists/:suite/Release.gpg` | detached signature of Release |
| `GET /apt/:repo/dists/:suite/:component/binary-:arch/Packages` | package index (text) |
| `GET /apt/:repo/dists/:suite/:component/binary-:arch/Packages.gz` | gzip |
| `GET /apt/:repo/dists/:suite/:component/binary-:arch/Packages.xz` | xz (optional) |
| `GET /apt/:repo/pool/:component/:letter/:src/:filename` | the `.deb` file (streamed, Range support) |

- `:arch` excludes `all`; `all`-arch packages are merged into each real arch's
  index.
- Pool path resolution: `pool/<component>/<letter>/<src>/<filename>` → DB lookup
  by `pool_path` within `:repo` → serve `store/packages/<sha256>.deb` via
  `http.ServeContent` (supports Range, ETag, Last-Modified).
- `:letter`/`:src` follow Debian convention: if source name starts with `lib`,
  prefix = `lib` + first char after `lib` (e.g. `liba` for `libapache2…`);
  else prefix = first char of source/package name. This is cosmetic — only
  internal consistency matters.

### Auth on the APT endpoint
- **Public repo:** no auth for any GET.
- **Private repo:** every GET requires **HTTP Basic auth** with
  `username = <username>`, `password = <api token>`. Server resolves the token,
  checks the user has at least `read`/`write`/`read-write`/`admin` access (or is
  owner/admin). On missing/invalid creds, respond `401` with
  `WWW-Authenticate: Basic realm="urapt <repo>"` so apt's `auth.conf` triggers.

  Client `auth.conf` example:
  ```
  machine <host>
  login <username>
  password <api token>
  ```

---

## 8. Index generation (`shared/apt`)

All indices are built **in memory from the DB** and may be cached (see §11).
Never written to disk.

### Packages index (per component + arch)
For `(repo, suite, component, arch)`:
1. Query all `packages` rows matching `distribution_id`, `component_id`, and
   `(architecture = arch OR architecture = 'all')`.
2. For each row, emit a stanza starting with the control fields (from
   `raw_control`, filtered/normalized) plus the file fields:
   ```
   Package: <name>
   Version: <version>
   Architecture: <arch as stored>
   Filename: <pool_path>            # e.g. pool/main/f/foo/foo_1.0_amd64.deb
   Size: <size>
   MD5sum: <md5sum>
   SHA1: <sha1>
   SHA256: <sha256>
   ...other control fields...
   Description: <description>
   ```
3. Entries separated by a blank line; file ends with a blank line.
4. Serve as `Packages`; also serve gzip (`Packages.gz`) and xz (`Packages.xz`).

### Release file (per suite)
1. Determine components (list of component names for the suite) and
   architectures (configured arch names).
2. For each `(component, arch)` generate the `Packages`, `Packages.gz`,
   `Packages.xz` bytes (reuse from the per-arch generation).
3. Compute checksums + sizes of each, keyed by their **path relative to the
   suite**, e.g. `main/binary-amd64/Packages`.
4. Emit:
   ```
   Origin: urapt <repo>
   Label: urapt <repo>
   Suite: <suite>
   Codename: <suite>
   Date: <RFC2822 now>
   Architectures: amd64 arm64
   Components: main contrib
   Description: <repo description>
   MD5Sum:
    <md5>  <size>  main/binary-amd64/Packages
    ...
   SHA1:
    <sha1>  <size>  main/binary-amd64/Packages
    ...
   SHA256:
    <sha256>  <size>  main/binary-amd64/Packages
    ...
   ```
   - No `Valid-Until` (avoid expiry on quiet self-hosted repos).
   - `Date` is regenerated on cache invalidation.

### InRelease
- `InRelease` = **clearsigned** `Release` (inline OpenPGP signature) using the
  server's default GPG key. Preferred by modern apt.

### Release.gpg
- **Detached** signature of `Release` using the same key. For older apt flows.

### Caching
- A per-`(repo, suite)` cache holds: `{Packages map, Release bytes, InRelease
  bytes, Release.gpg bytes, generation int}`.
- Generation is bumped on any mutation affecting that `(repo, suite)`:
  package push/delete, component/arch add/remove, distribution rename/delete,
  repo visibility change.
- Cache is in-memory only; rebuilt lazily on first request after a bump or after
  server restart. Mutex per key.

---

## 9. GPG signing (`shared/gpg`)

- Uses `github.com/ProtonMail/go-crypto/openpgp`.
- **Key generation** (on first server startup if no default key exists):
  - Key type: **RSA-4096** (broad apt/gpg compatibility; configurable later).
  - User-id: configurable, default `urapt-server <hostname>`.
  - No passphrase (v1; stored armored in DB).
  - Persist armored public + private key to `gpg_keys` with `is_default=1`.
- **Operations:**
  - `Clearsign(data) → InRelease`
  - `DetachedSign(data) → Release.gpg`
  - `ExportPublic() → ascii-armored pubkey` (served at `/api/v1/server/pubkey`
    and `/apt/:repo/...` is signed by it).
- The pubkey is also exposed via the CLI (`urapt repo pubkey`) and the
  `apt-config` helper prints instructions to install it
  (`gpg --dearmor | tee /usr/share/keyrings/urapt.gpg` + signed-by line).

---

## 10. `.deb` parsing (`shared/deb`)

On push, the server must extract control metadata **without executing anything
from the package**.

### Steps
1. Open the `.deb` as an `ar` archive (`pault.ag/go/debian/deb` or raw ar).
2. Read the `debian-binary` member (validate version `2.0`).
3. Locate the `control.tar.*` member; decompress (gzip / xz / zstd by magic
   bytes).
4. From the tar, read the `control` file (and optional `shlibs`, `symbols`,
   `triggers` — ignored for v1).
5. Parse the control stanza (`pault.ag/go/debian/control`): RFC822-style fields.
6. Extract all fields needed for the `packages` row (see §6) and keep the full
   `raw_control` for re-emission.
7. Compute file hashes (MD5, SHA1, SHA256) and size **of the whole `.deb`**.

### Validation on push
- Must be a valid `ar` archive with `debian-binary` + `control.tar.*`.
- `Package`, `Version`, `Architecture` must be present and non-empty.
- `Architecture` must be one of the distribution's configured arches **or**
  `all`.
- Version must be a valid Debian version string (basic sanity).
- No duplicate of `(repo, distro, component, name, version, arch)` — else 409.
- Reject if `.deb` size > configured max (default 1 GiB).

### Filename / pool path
- Real filename: `store/packages/<sha256>.deb`.
- Pool path: `pool/<component>/<letter>/<src>/<original-filename>` where
  `<original-filename>` is the client-supplied `.deb` basename (sanitized) and
  `<src>` is the `Source` package name (or `Package` name if absent), and
  `<letter>` per the Debian convention above.

---

## 11. Storage layout

```
store/
  database/
    sqlite.db        SQLite (WAL: sqlite.db-wal, sqlite.db-shm)
  packages/
    <sha256>.deb     content-addressed; one file per unique .deb content
```

- Only `store/packages/*.deb` are filesystem user data.
- The server creates `store/`, `store/database/`, `store/packages/` on startup
  if missing.
- Paths are configurable (`store_dir`, `db_path`, `packages_dir`).
- Backups: snapshot `sqlite.db` (with WAL checkpoint) + `packages/` dir.

### Caching & invalidation (server)
- **Index cache**: in-memory, per `(repo, suite)`, generation-tagged (§8).
- **Mutation hooks**: every package/component/arch/distro mutation calls
  `cache.Invalidate(repo, suite)` which bumps generation; next read rebuilds.
- No on-disk cache → restart simply rebuilds on first request.

---

## 12. Authentication & authorization (`server/auth`)

### Identity resolution
- **REST API:** `Authorization: Bearer <token>` → SHA-256 → look up
  `api_tokens` (non-revoked) → load user. Update `last_used_at` (throttled).
  Missing/invalid → `401`.
- **APT endpoint (private repos):** `Authorization: Basic ...` → decode →
  `password` is the token → same resolution.

### Permission helpers
- `CanRead(user, repo)`: repo public → true; else owner / member with
  `read`/`write`/`read-write`/`admin` / server admin.
- `CanWrite(user, repo)`: owner / member with `write`/`read-write`/`admin` /
  server admin.
- `CanManage(user, repo)`: owner / member with `admin` / server admin.
- Server admin (`users.is_admin=1`) bypasses all checks.

### Endpoint-level enforcement
- All `/api/v1/**` except `auth/register`, `auth/login`, `server/info`,
  `server/pubkey` require a valid identity.
- Write operations require `CanWrite`; member management requires `CanManage`;
  user-management requires server admin.
- APT read: `CanRead` (and for public repos, anonymous allowed).

---

## 13. REST API specification

Base: `/api/v1`. JSON in/out unless noted. All list endpoints support
`?page=&per_page=` (default 25, max 100) and return `{items, page, per_page,
total}`.

### Server / setup
| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/server/info` | none | `{version, needsSetup, defaultKeyFingerprint}` |
| GET | `/server/pubkey` | none | ASCII-armored default pubkey (`Content-Type: application/pgp-keys`) |

### Auth
| Method | Path | Auth | Body / Notes |
|---|---|---|---|
| POST | `/auth/register` | none | `{username, password}` → `{user, token}`. First registration → admin. Disabled once `users` non-empty? No — open registration configurable; default open. |
| POST | `/auth/login` | none | `{username, password}` → `{user, token}` (creates a session token) |
| POST | `/auth/logout` | bearer | revokes the current token |
| GET | `/me` | bearer | current user |
| GET | `/me/tokens` | bearer | list the user's tokens |
| POST | `/me/tokens` | bearer | `{name}` → `{id, prefix, token}` (plain token returned once) |
| DELETE | `/me/tokens/:id` | bearer | revoke |

> `register`/`login` both return a plain token once; the CLI stores it.

### Users (admin)
| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/users` | admin | list users |
| GET | `/users/:id` | admin | get user |
| PATCH | `/users/:id` | admin | `{is_admin?}` |
| DELETE | `/users/:id` | admin | delete user (cannot delete self) |

### Repositories
| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/repositories` | bearer | list repos visible to caller (owned + member + public) |
| POST | `/repositories` | bearer | `{name, visibility, description?}` → owner=caller |
| GET | `/repositories/:repo` | bearer* | repo detail + distros summary (*read check for private) |
| PATCH | `/repositories/:repo` | manage | `{visibility?, description?, name?}` |
| DELETE | `/repositories/:repo` | manage | delete repo + cascade |
| GET | `/repositories/:repo/members` | read | list members |
| POST | `/repositories/:repo/members` | manage | `{username, access}` |
| PATCH | `/repositories/:repo/members/:username` | manage | `{access}` |
| DELETE | `/repositories/:repo/members/:username` | manage | remove member |
| GET | `/repositories/:repo/pubkey` | read | ASCII-armored server pubkey (convenience) |

### Distributions / components / architectures
| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/repositories/:repo/distributions` | read | list |
| POST | `/repositories/:repo/distributions` | write | `{name}` |
| DELETE | `/repositories/:repo/distributions/:dist` | write | cascade delete its packages |
| GET | `/repositories/:repo/distributions/:dist/components` | read | |
| POST | `/repositories/:repo/distributions/:dist/components` | write | `{name}` |
| DELETE | `/repositories/:repo/distributions/:dist/components/:comp` | write | cascade |
| GET | `/repositories/:repo/distributions/:dist/architectures` | read | |
| POST | `/repositories/:repo/distributions/:dist/architectures` | write | `{name}` |
| DELETE | `/repositories/:repo/distributions/:dist/architectures/:arch` | write | (reject if packages reference it and arch!=all) |

### Packages
| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/repositories/:repo/distributions/:dist/packages` | read | filters: `?component=&arch=&name=&q=` |
| GET | `/repositories/:repo/packages/:id` | read | package metadata |
| POST | `/repositories/:repo/distributions/:dist/packages` | write | **multipart/form-data**: `file` = .deb, `component` = name. Parses, validates, stores. → `{package}` |
| GET | `/repositories/:repo/packages/:id/file` | read | stream the `.deb` (CLI `pull`) |
| DELETE | `/repositories/:repo/packages/:id` | write | delete version (decrements blob ref) |

### Errors
Uniform `{error: {code, message, details?}}` with appropriate HTTP status
(`400` validation, `401` unauth, `403` forbidden, `404` not found, `409`
conflict/duplicate, `500` server).

---

## 14. APT endpoint specification

Routes (see §7). Behavior summary:
- All under `/apt/:repo/...`.
- Public repo: anonymous GETs.
- Private repo: HTTP Basic required (token as password); `401` + challenge
  otherwise.
- Indices generated via `shared/apt` with the in-memory cache.
- Pool files served via `http.ServeContent` (Range, ETag by sha256).
- `404` for unknown repo/suite/component/arch.
- The APT endpoint is mounted on the **same HTTP server** as the REST API, just
  a different path prefix and a different auth mode (Basic vs Bearer).

---

## 15. CLI design (`cmd/urapt`)

### Local config & token store
- File: `~/.config/urapt/config.toml` (perm `0600`).
- Contents:
  ```toml
  [default]
  server = "https://apt.example.com"
  user = "alice"
  token = "urapt_..."     # stored; future: OS keychain
  ```
- `--server`, `--user`, `--token` flags override; env `URAPT_SERVER`,
  `URAPT_TOKEN` also supported.
- Future: integrate `go-keyring` for the token.

### Commands (cobra)
```
urapt version
urapt login [<server>] [--username]         # prompts password; stores token
urapt logout
urapt whoami
urapt register [<server>] [--username]      # create account (first → admin)

urapt token create [--name]
urapt token list
urapt token revoke <id|prefix>

urapt repo create <name> [--public|--private] [--description]
urapt repo list
urapt repo info <name>
urapt repo set-visibility <name> --public|--private
urapt repo delete <name>
urapt repo pubkey <name> [-o file]

urapt repo members list <name>
urapt repo members add <name> <username> --access=read|write|read-write|admin
urapt repo members update <name> <username> --access=...
urapt repo members remove <name> <username>

urapt distro create <repo> <distro>
urapt distro list <repo>
urapt distro delete <repo> <distro>

urapt component create <repo> <distro> <component>
urapt component list <repo> <distro>
urapt component delete <repo> <distro> <component>

urapt arch add <repo> <distro> <arch>
urapt arch list <repo> <distro>
urapt arch remove <repo> <distro> <arch>

urapt push <repo> <distro> <component> <file.deb> [--arch=<override>]
urapt pull <repo> <name>[@<version>][:<arch>] [-o file]   # or by id via --id
urapt ls <repo> <distro> [--component] [--arch] [--name] [-q]
urapt show <repo> <id|name[@version][:arch]>
urapt rm <repo> <id|name[@version][:arch]>

urapt apt-config <repo> <distro> [--component=main] [--signed-by=/usr/share/keyrings/urapt.gpg]
        # prints: sources.list line, pubkey install steps, and (if private) auth.conf
```

### Output
- Human-readable tables by default; `--json` for scripting.
- `urapt apt-config` is the key UX helper: it fetches the pubkey, prints the
  signed-by sources line, and (for private repos) the `auth.conf` snippet.

### Push flow (detailed)
1. Resolve token from config.
2. Verify the `.deb` locally (`shared/deb`) to give early, clear errors.
3. `POST /repositories/:repo/distributions/:dist/packages` (multipart) with
   `component` and the file.
4. Server parses/validates/stores, returns the package record.
5. CLI prints the new package's `name_version_arch` and pool path.

---

## 16. Server configuration

Sources, precedence **low → high**: defaults → TOML file (`--config`, default
`./urapt-server.toml`) → env (`URAPT_*`) → CLI flags.

| Key | Env | Default | Notes |
|---|---|---|---|
| `bind` | `URAPT_BIND` | `0.0.0.0:8080` | listen address |
| `base_url` | `URAPT_BASE_URL` | `http://localhost:8080` | external URL for generated apt config / pubkey links |
| `store_dir` | `URAPT_STORE_DIR` | `./store` | |
| `db_path` | `URAPT_DB_PATH` | `<store_dir>/database/sqlite.db` | |
| `packages_dir` | `URAPT_PACKAGES_DIR` | `<store_dir>/packages` | |
| `log_level` | `URAPT_LOG_LEVEL` | `info` | debug/info/warn/error |
| `signing_key_type` | `URAPT_SIGNING_KEY_TYPE` | `rsa` | |
| `signing_key_bits` | `URAPT_SIGNING_KEY_BITS` | `4096` | |
| `signing_key_user_id` | `URAPT_SIGNING_KEY_USER_ID` | `urapt-server <hostname>` | |
| `max_package_size` | `URAPT_MAX_PACKAGE_SIZE` | `1073741824` (1 GiB) | |
| `open_registration` | `URAPT_OPEN_REGISTRATION` | `true` | allow new account registration |
| `tls_enabled` | `URAPT_TLS_ENABLED` | `false` | optional built-in TLS |
| `tls_cert` / `tls_key` | … | — | paths if TLS enabled |

> For internet-facing deployments, recommend a TLS-terminating reverse proxy
> (Caddy/nginx) over built-in TLS.

### Startup sequence (`server/app`)
1. Load config.
2. Ensure `store_dir`, `db_path` parent, `packages_dir` exist.
3. Open SQLite (WAL, foreign_keys, busy_timeout); run migrations.
4. Ensure a default GPG key exists (generate if missing).
5. Build index cache; wire router (REST + APT); start HTTP server.
6. Log `base_url`, `needsSetup` (no users yet), and pubkey fingerprint.

---

## 17. Security considerations

- Passwords: bcrypt (cost 12).
- Tokens: 32-byte `crypto/rand`, base64url, prefixed `urapt_`; store only
  SHA-256; `prefix` column for display; revocable; `last_used_at` tracking.
- Private repo APT reads: HTTP Basic over **TLS only** (warn if plain HTTP +
  private repo).
- GPG private key in DB: documented tradeoff; future encryption-at-rest.
- File uploads: stream to a temp file, hash, then move to
  `store/packages/<sha256>.deb` (no execution, no path traversal — pool paths
  are generated server-side, not from client input).
- Input validation: repo/distro/component/arch names restricted to safe
  charset (`[a-z0-9][a-z0-9-+.]*`, lowercase); usernames `[a-z0-9_-]{3,32}`.
- Rate limiting on `auth/login` and `auth/register` (future; note in README).
- No CORS by default (API-only; future web UI would add it).
- `ServeContent` for pool files prevents directory traversal (DB lookup by
  exact `pool_path`).

---

## 18. Testing strategy

### Unit (`shared/**`)
- `shared/crypto`: hash/verify, token round-trip.
- `shared/deb`: parse a fixture `.deb` (gzip/xz/zstd control.tar), assert
  fields, hashes, sizes.
- `shared/apt`: given in-memory package rows, assert `Packages`, `Release`,
  `InRelease`, `Release.gpg` byte content and checksum correctness.
- `shared/gpg`: generate key, clearsign + verify, detached sign + verify
  round-trip using the same library.

### Integration (`server/**`, `cli/**`)
- `httptest.Server` + temp SQLite + temp `store/`.
- REST: register → login → create repo → add distro/component/arch → push
  fixture `.deb` → list → pull → delete; permission matrix tests.
- APT: push a fixture, then `GET` `InRelease`/`Release`/`Packages`/pool and
  assert content + signature verification with the server pubkey.
- Private repo: assert `401` without Basic, `200` with token Basic.
- CLI: thin tests using a fake `apiclient` (interface-based) for command
  output; plus an end-to-end test spinning the server against a temp dir.

### Full apt integration (CI, optional)
- A Debian-based Docker container test: install a built `urapt-server`, push a
  real `.deb`, configure apt against it, run `apt-get update && apt-get install
  <pkg>`, assert the package installs. Runs in CI matrix (not required for
  `go test`).

### Lint / format
- `go vet ./...`, `gofmt -l`, `golangci-lint run` (Makefile targets).

---

## 19. Deployment

### Docker
- Multi-stage `Dockerfile`: `golang:1.22` build → `gcr.io/distroless/static`
  (or `alpine`) runtime. Expose `8080`. Volume `/data` mapped to `store_dir`.
- `docker-compose.yml`: one service, persistent volume, env for `base_url`,
  reverse-proxy example.

### Releases
- `Makefile` builds `urapt-server` and `urapt` for
  `linux/amd64,linux/arm64,darwin/amd64,darwin/arm64` (CGO disabled via
  `modernc.org/sqlite`).
- GitHub Releases with archive + checksums (future CI).

### First-run UX (documented in README)
1. Run `urapt-server` (or docker compose up).
2. `urapt register` → you become admin.
3. `urapt repo create myrepo --public`.
4. `urapt distro create myrepo stable`.
5. `urapt component create myrepo stable main`.
6. `urapt arch add myrepo stable amd64`.
7. `urapt push myrepo stable main ./foo_1.0_amd64.deb`.
8. `urapt apt-config myrepo stable` → paste into client `/etc/apt/sources.list.d/myrepo.list`, install pubkey.
9. `sudo apt-get update && sudo apt-get install foo`.

---

## 20. Implementation phases & tasks

Each phase ends with `go build ./...` + `go test ./...` green.

### Phase 0 — Scaffold
- [ ] `go mod init urapt`; Go 1.22.
- [ ] Directory tree (§4); empty packages with doc comments.
- [ ] `Makefile` (`build`, `test`, `vet`, `fmt`, `lint`, `run-server`, `run-cli`).
- [ ] `.gitignore` (`store/`, `*.db*`, `urapt`, `urapt-server`).
- [ ] `shared/version`, `cmd/urapt-server/main.go` + `cmd/urapt/main.go` stubs.

### Phase 1 — Shared core
- [ ] `shared/config`: struct + TOML + env + flag loading; defaults.
- [ ] `shared/db`: open sqlite (WAL/FK/busy), `embed` migrations, runner,
  query helpers (`*sql.DB` wrapper).
- [ ] `migrations/0001_init.sql` (all tables in §6).
- [ ] `shared/models`: Go structs for every table.
- [ ] `shared/crypto`: bcrypt password, token gen + SHA-256, prefix.
- [ ] `shared/log`: slog wrapper.
- [ ] `shared/httputil`: JSON read/write, error rendering, bearer/basic parse.

### Phase 2 — Shared domain utilities
- [ ] `shared/gpg`: keygen, export pubkey, clearsign, detached sign (round-trip
  tests).
- [ ] `shared/deb`: ar open, control.tar extract (gz/xz/zstd), control parse,
  full-field extraction + hashes/size (fixture tests).
- [ ] `shared/apt`: `Packages`, `Release`, `InRelease`, `Release.gpg`
  generation + cache struct (unit tests with fake rows).

### Phase 3 — Server: auth & users
- [ ] `server/app`: wiring, startup sequence, key init.
- [ ] `server/middleware`: recover, logging, bearer auth, basic auth.
- [ ] `server/auth`: identity resolution, `CanRead/Write/Manage`.
- [ ] `server/restapi`: `/server/info`, `/server/pubkey`, `/auth/register`,
  `/auth/login`, `/auth/logout`, `/me`, `/me/tokens`, `/users*`.
- [ ] First-registration-is-admin logic.

### Phase 4 — Server: repos & structure
- [ ] Repositories CRUD + members CRUD + visibility + pubkey.
- [ ] Distributions / components / architectures CRUD.
- [ ] Permission enforcement on all of the above.
- [ ] Cache invalidation hooks wired.

### Phase 5 — Server: packages
- [ ] `POST .../packages`: multipart receive → `shared/deb` parse → validate →
  store blob → insert row → invalidate cache.
- [ ] `GET .../packages` (list) + `GET .../packages/:id` + `GET .../file`
  (stream) + `DELETE .../packages/:id` (blob refcount).
- [ ] Blob dedup + cleanup on refcount 0.

### Phase 6 — Server: APT endpoint
- [ ] `server/aptrepo`: routes (§7), cache-backed handlers, pool serving via
  `ServeContent`.
- [ ] Private-repo Basic auth + `401` challenge.
- [ ] Signature of Release/InRelease with default key.
- [ ] Integration tests: push → fetch indices → verify sigs → fetch pool.

### Phase 7 — CLI: config & auth
- [ ] `cli/config`: load/save TOML, token store, flag/env overrides.
- [ ] `cli/commands`: `login`, `logout`, `whoami`, `register`, `version`,
  `token *`.
- [ ] `shared/apiclient`: typed client for all endpoints used so far.

### Phase 8 — CLI: repos & structure
- [ ] `repo *`, `repo members *`, `repo pubkey`, `distro *`, `component *`,
  `arch *` commands.
- [ ] Extend `shared/apiclient`.

### Phase 9 — CLI: packages & apt-config
- [ ] `push`, `pull`, `ls`, `show`, `rm`.
- [ ] `apt-config` helper (fetch pubkey, print sources + signed-by + auth.conf).
- [ ] End-to-end CLI↔server test (temp dir).

### Phase 10 — Hardening, docs, packaging
- [ ] README quickstart + architecture summary.
- [ ] `Dockerfile` (multi-stage) + `docker-compose.yml`.
- [ ] Cross-compile Makefile targets.
- [ ] golangci-lint config; fix findings.
- [ ] Optional: Debian-container apt integration test.
- [ ] Security review pass (input validation, path traversal, authz matrix).

---

## 21. Open / future

- **Per-repo GPG keys** (each repo its own trust root) — schema already
  extensible; add `repositories.signing_key_id`.
- **GPG private key encryption at rest** (passphrase from config/env).
- **OS keychain** for CLI token (`go-keyring`).
- **Source packages** (`deb-src`, `source/` indices).
- **AppStream / `dep11`** metadata; **`Contents`** indexes.
- **`Acquire-By-Hash`** for atomic updates.
- **Web UI** (small SPA) + CORS.
- **Rate limiting** on auth endpoints.
- **Built-in TLS** (Let's Encrypt autocert option).
- **Token scopes** (per-repo, read-only tokens).
- **Webhook on package push** (for downstream CI).
- **Open registration toggle / invite-only mode** (config exists; add invites).
- **Retention policies** (keep last N versions per package).

---

## 22. Glossary

- **Suite / Distribution** — e.g. `stable`, `testing`, `jammy`. The apt
  `deb ... <suite> <component>` line.
- **Component** — e.g. `main`, `contrib`, `non-free`. A section within a suite.
- **Architecture** — e.g. `amd64`, `arm64`. `all` = architecture-independent.
- **Packages index** — `dists/<suite>/<component>/binary-<arch>/Packages`:
  lists every `.deb` in that slice with metadata + download path.
- **Release / InRelease / Release.gpg** — suite-level metadata + checksums of
  the indices, signed so apt can trust them.
- **Pool** — flat directory tree where the actual `.deb` files live
  (`pool/<component>/<letter>/<src>/<file>.deb`).
- **Blob** — a content-addressed `.deb` file in `store/packages/<sha256>.deb`,
  reference-counted so identical uploads deduplicate.
