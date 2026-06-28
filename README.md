# urapt

A self-hostable APT repository server with a companion CLI for pushing and
managing Debian/Ubuntu `.deb` packages under your logged-in user.

urapt gives you your own `apt` server: run the server, log in with the CLI,
create repositories, and push `.deb` files. Clients configure `apt` against it
and install packages normally. Packages are stored as content-addressed files
on disk; everything else lives in a SQLite database. APT indices
(`Release`, `InRelease`, `Packages`) are generated on demand from the database
and signed with a server-managed OpenPGP key.

## Components

- **`urapt-server`** (`cmd/urapt-server`) — the REST API + APT endpoint server.
- **`urapt`** (`cmd/urapt`) — the CLI for pushing packages and managing repos.
- **`shared/`** — shared utilities (config, db, models, gpg, deb parsing, apt
  index generation, the typed API client) used by both server and CLI.

## Quick start

### Run the server

```bash
make build
./urapt-server --bind 0.0.0.0:8080 --base-url https://apt.example.com
```

Or with Docker:

```bash
docker compose up -d   # see docker-compose.yml
```

The server creates `store/database/sqlite.db` and `store/packages/` on first
run and generates an RSA-4096 signing key stored in the database.

### Set up the CLI

```bash
./urapt register https://apt.example.com          # first account becomes admin
./urapt repo create myrepo --public
./urapt distro create myrepo stable
./urapt component create myrepo stable main
./urapt arch add myrepo stable amd64
```

### Push a package

```bash
./urapt push myrepo stable main ./hello_1.0.0_amd64.deb
```

### Configure apt clients

```bash
./urapt apt-config myrepo stable
```

This prints the exact commands to install the signing key and add the
repository, for example:

```bash
curl -fsSL https://apt.example.com/api/v1/server/pubkey \
  | sudo gpg --dearmor -o /usr/share/keyrings/urapt-myrepo.gpg
echo 'deb [arch=amd64 signed-by=/usr/share/keyrings/urapt-myrepo.gpg] https://apt.example.com/apt/myrepo/ stable main' \
  | sudo tee /etc/apt/sources.list.d/myrepo.list
sudo apt update
sudo apt install hello
```

For **private** repositories, `apt-config` also prints an
`/etc/apt/auth.conf.d/...` snippet using your API token as the password.

## Architecture

```
            +-------------------+        +-------------------+
            |   urapt (CLI)     |        |  urapt-server     |
            +---------+---------+        +---------+---------+
                      |  shared/                  |  shared/
                      v                           v
            +---------------------------------------------+
            |                 shared/                     |
            | config | db | models | gpg | deb | apt |    |
            | crypto | api(DTOs) | apiclient | httputil   |
            +---------------------------------------------+
                      |                            |
                      +--> SQLite <---+      store/database/sqlite.db
                                     store/packages/<sha256>.deb  (files only)
```

- The CLI never touches the DB or filesystem; it only talks to the REST API.
- Only uploaded `.deb` files are stored on disk (`store/packages/`), content-
  addressed by SHA-256 and reference-counted for deduplication.
- APT indices are generated in memory from the DB and cached (invalidated on
  any mutation); they are never written to disk.

### Endpoints

- **REST API** at `/api/v1/**` — auth, users, repositories, members,
  distributions/components/architectures, packages. Bearer-token auth.
- **APT endpoint** at `/apt/:repo/**` — serves `dists/.../{Release,InRelease,
  Release.gpg}`, `Packages[.gz|.xz]`, and `pool/.../*.deb`. Public repos allow
  anonymous reads; private repos require HTTP Basic auth (password = API token).

### Permissions

Each repository has an owner with full access. The owner can grant `read`,
`write`, `read-write`, or `admin` to other users. Only users with access can
push. Server admins can manage everything.

## Configuration

Server config is loaded from defaults → TOML file (`--config`, default
`./urapt-server.toml`) → environment (`URAPT_*`) → flags. Key options:

| Key | Default | Notes |
|---|---|---|
| `bind` | `0.0.0.0:8080` | listen address |
| `base_url` | `http://localhost:8080` | external URL for apt-config output |
| `store_dir` | `./store` | data directory |
| `signing_key_bits` | `4096` | RSA signing key size |
| `max_package_size` | `1073741824` | 1 GiB upload limit |
| `open_registration` | `true` | allow new account registration |

## Building

Requires Go 1.22+. CGO is not required (SQLite is the pure-Go `modernc`
driver), so binaries are static and cross-compilable.

```bash
make build         # builds urapt-server and urapt
make test          # go test ./...
make vet           # go vet ./...
```

## Security notes

- Passwords are bcrypt-hashed; API tokens are random 32-byte values stored only
  as SHA-256 hashes (revocable, with a display prefix).
- The OpenPGP **private signing key is stored unencrypted in the SQLite
  database**. This is acceptable when you control the database file; for
  stronger protection, restrict file permissions and back up the DB securely.
  Per-repo keys and key encryption-at-rest are planned.
- For internet-facing deployments, run behind a TLS-terminating reverse proxy
  (Caddy/nginx). Private-repo credentials must never travel over plain HTTP.

## Status

See `CHANGELOG.md` for release history and `PLAN.md` for the full design and
roadmap. Future work includes per-repo signing keys, key encryption-at-rest,
OS keychain token storage, source packages, AppStream metadata, and a web UI.

## License

urapt is free software released under the terms of the
[GNU General Public License v3.0 or later](LICENSE). See `LICENSE` for the
full text.
