-- 0001_init.sql: initial urapt schema.

CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    username      TEXT UNIQUE NOT NULL,
    username_lc   TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    is_admin      INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE TABLE api_tokens (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    prefix       TEXT NOT NULL,
    token_hash   TEXT UNIQUE NOT NULL,
    created_at   TEXT NOT NULL,
    last_used_at TEXT,
    revoked_at   TEXT
);

CREATE INDEX idx_api_tokens_user ON api_tokens(user_id);
CREATE INDEX idx_api_tokens_hash ON api_tokens(token_hash);

CREATE TABLE repositories (
    id            TEXT PRIMARY KEY,
    name          TEXT UNIQUE NOT NULL,
    owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    visibility    TEXT NOT NULL CHECK (visibility IN ('public','private')),
    description   TEXT,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE TABLE repository_members (
    repository_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    access        TEXT NOT NULL CHECK (access IN ('read','write','read-write','admin')),
    created_at    TEXT NOT NULL,
    PRIMARY KEY (repository_id, user_id)
);

CREATE TABLE distributions (
    id            TEXT PRIMARY KEY,
    repository_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    UNIQUE (repository_id, name)
);

CREATE TABLE components (
    id              TEXT PRIMARY KEY,
    distribution_id TEXT NOT NULL REFERENCES distributions(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    created_at      TEXT NOT NULL,
    UNIQUE (distribution_id, name)
);

CREATE TABLE architectures (
    id              TEXT PRIMARY KEY,
    distribution_id TEXT NOT NULL REFERENCES distributions(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    created_at      TEXT NOT NULL,
    UNIQUE (distribution_id, name)
);

CREATE TABLE packages (
    id                  TEXT PRIMARY KEY,
    repository_id       TEXT NOT NULL REFERENCES repositories(id) ON DELETE RESTRICT,
    distribution_id     TEXT NOT NULL REFERENCES distributions(id) ON DELETE CASCADE,
    component_id        TEXT NOT NULL REFERENCES components(id) ON DELETE RESTRICT,
    name                TEXT NOT NULL,
    version             TEXT NOT NULL,
    architecture        TEXT NOT NULL,
    source              TEXT,
    maintainer          TEXT,
    priority            TEXT,
    section             TEXT,
    origin              TEXT,
    homepage            TEXT,
    description         TEXT,
    description_md5     TEXT,
    depends             TEXT,
    pre_depends         TEXT,
    recommends          TEXT,
    suggests            TEXT,
    conflicts           TEXT,
    breaks              TEXT,
    provides            TEXT,
    replaces            TEXT,
    enhances            TEXT,
    installed_size      INTEGER,
    essential           TEXT,
    built_using         TEXT,
    tag                 TEXT,
    raw_control         TEXT NOT NULL,
    filename            TEXT NOT NULL,
    pool_path           TEXT NOT NULL,
    size                INTEGER NOT NULL,
    md5sum              TEXT NOT NULL,
    sha1                TEXT NOT NULL,
    sha256              TEXT NOT NULL,
    uploaded_by_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at          TEXT NOT NULL,
    UNIQUE (repository_id, distribution_id, component_id, name, version, architecture)
);

CREATE INDEX idx_packages_lookup ON packages(repository_id, distribution_id, component_id, name);
CREATE INDEX idx_packages_distro ON packages(repository_id, distribution_id);
CREATE INDEX idx_packages_arch ON packages(distribution_id, architecture);
CREATE INDEX idx_packages_sha256 ON packages(sha256);

CREATE TABLE blobs (
    sha256     TEXT PRIMARY KEY,
    filename   TEXT NOT NULL,
    size       INTEGER NOT NULL,
    ref_count  INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE TABLE gpg_keys (
    id                   TEXT PRIMARY KEY,
    fingerprint          TEXT UNIQUE NOT NULL,
    user_id              TEXT NOT NULL,
    public_key_armored   TEXT NOT NULL,
    private_key_armored  TEXT NOT NULL,
    is_default           INTEGER NOT NULL DEFAULT 0,
    created_at           TEXT NOT NULL
);

CREATE TABLE audit_log (
    id            TEXT PRIMARY KEY,
    user_id       TEXT REFERENCES users(id) ON DELETE SET NULL,
    repository_id TEXT REFERENCES repositories(id) ON DELETE SET NULL,
    action        TEXT NOT NULL,
    target        TEXT,
    details       TEXT,
    created_at    TEXT NOT NULL
);

CREATE INDEX idx_audit_repo ON audit_log(repository_id, created_at);
CREATE INDEX idx_audit_user ON audit_log(user_id, created_at);
