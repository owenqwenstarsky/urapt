# Changelog

All notable changes to urapt will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-06-28

First public release. urapt is a self-hostable APT repository server with a
companion CLI for pushing and managing Debian `.deb` packages.

### Added
- **Server** (`urapt-server`): REST API + APT endpoint server.
  - SQLite storage (pure-Go `modernc.org/sqlite`, no CGO) with embedded
    migrations. Only `.deb` files are stored on disk, content-addressed by
    SHA-256 and reference-counted for deduplication.
  - Server-managed RSA-4096 OpenPGP signing key (`ProtonMail/go-crypto`),
    generated on first run and stored armored in the database.
  - APT indices (`Release`, `Release.gpg`, `InRelease`, `Packages` with
    `.gz`/`.xz` compression) generated on demand from the database and cached
    in memory; never written to disk.
  - Full APT model: repositories → distributions → components → architectures.
  - Bearer-token auth for the REST API; HTTP Basic auth (password = API token)
    for private-repo APT reads. Public repos allow anonymous APT reads.
  - First registrant becomes admin; repo-scoped permissions
    (`read`/`write`/`read-write`/`admin`) plus owner and server-admin roles.
  - Multipart package push with control-field extraction, list/show/delete,
    pool serving, and blob ref-count cleanup on delete.
  - Audit log for mutating actions.
- **CLI** (`urapt`): companion tool for pushing packages and managing repos.
  - `register`, `login`, `logout`, `whoami`, `token` (create/list/revoke).
  - `repo` (create/list/show/update/delete), `member` (add/update/remove/list).
  - `distro`, `component`, `arch` CRUD.
  - `push`, `pull`, `ls`, `show`, `rm` for packages.
  - `apt-config` helper that prints the exact `apt` setup commands (key
    install, sources.list entry, and auth.conf snippet for private repos).
- **Packaging**: `Dockerfile` (multi-stage distroless static build),
  `docker-compose.yml`, sample `urapt-server.toml.example`.
- **Docs**: `README.md` quick start, architecture overview, configuration
  reference, and security notes; `PLAN.md` full design blueprint.

### Security
- Passwords are bcrypt-hashed (cost 12). API tokens are random 32-byte values
  stored only as SHA-256 hashes with a short display prefix; revocable.
- The OpenPGP private signing key is stored unencrypted in the SQLite
  database. This is acceptable when you control the database file; for
  stronger protection, restrict file permissions and back up the DB securely.
  Per-repo keys and key encryption-at-rest are planned.
- For internet-facing deployments, run behind a TLS-terminating reverse proxy
  (Caddy/nginx). Private-repo credentials must never travel over plain HTTP.

### Known Limitations
- Single server-managed signing key (no per-repo keys yet).
- No web UI; all management is via the CLI.
- No rate limiting or brute-force protection on login endpoints.
- No source packages (`.dsc`/`.orig.tar.*`) or AppStream metadata.

[Unreleased]: https://github.com/owen/urapt/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/owen/urapt/releases/tag/v0.1.0
