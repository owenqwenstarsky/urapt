# urapt-web

The web UI for [urapt](https://github.com/owenqwenstarsky/urapt), a self-hostable
APT repository server. A React single-page app that talks to a urapt-server's
REST API (`/api/v1`) to manage repositories, members, distributions, packages,
tokens, and users — full management parity with the `urapt` CLI.

## Stack

- **Vite** + **React 18** + **TypeScript**
- **Tailwind CSS** + **shadcn/ui** (New York, neutral)
- **React Router v7**, **TanStack Query v5**, **Zustand**, **Vitest**

## Requirements

- Node.js 20+ (see `.nvmrc`) and pnpm 10+

## Development

Start a local urapt-server (from the repo root):

```bash
make run-server    # listens on 0.0.0.0:8080
```

Then in `urapt-web/`:

```bash
pnpm install
pnpm dev           # http://localhost:5173
```

Vite proxies `/api` and `/apt` to `http://localhost:8080` (override with the
`URAPT_SERVER` env var), so the UI is same-origin with the API in dev — no CORS
needed. If `needs_setup` is true (no users yet), you'll be sent to the first-run
setup screen to create the admin account.

## Scripts

| Script           | Description                          |
| ---------------- | ------------------------------------ |
| `pnpm dev`       | Start the Vite dev server            |
| `pnpm build`     | Type-check and build to `dist/`      |
| `pnpm preview`   | Preview the production build locally |
| `pnpm typecheck` | Run `tsc` with no emit               |
| `pnpm lint`      | Run ESLint                           |
| `pnpm format`    | Run Prettier (write)                 |
| `pnpm test`      | Run Vitest once                      |

## Configuration

| Env var             | Default                 | Notes                                                                            |
| ------------------- | ----------------------- | -------------------------------------------------------------------------------- |
| `VITE_API_BASE_URL` | `/api/v1`               | REST API base. Override only for cross-origin dev (requires CORS on the server). |
| `URAPT_SERVER`      | `http://localhost:8080` | Dev proxy target only.                                                           |

## Deployment (same-origin)

The UI is designed to be served from the **same origin** as a urapt-server,
behind a reverse proxy that serves the SPA from `dist/` and forwards `/api/v1`
and `/apt` to `urapt-server`. No CORS, no server changes — the API base is just
`/api/v1`.

```bash
pnpm build    # outputs dist/
```

### Railway

`urapt-web/` includes a Dockerfile for deploying the UI as a separate Railway
service. The container builds the Vite app and serves it with Caddy, proxying
`/api/*` and `/apt/*` to an existing `urapt-server` service.

In Railway, create a new service from this repo with:

- Root Directory: `urapt-web`
- Environment variable: `URAPT_SERVER_URL=https://your-urapt-server.up.railway.app`

A trailing slash is okay; the container normalizes it before starting Caddy.

Do not set `VITE_API_BASE_URL` for this deployment; the UI should keep using the
default same-origin `/api/v1` path.


### Caddy

```caddy
apt.example.com {
    # SPA
    root * /srv/urapt-web/dist
    try_files {path} /index.html
    file_server

    # REST API + APT endpoint
    reverse_proxy /api/* 127.0.0.1:8080
    reverse_proxy /apt/*  127.0.0.1:8080
}
```

### nginx

```nginx
server {
    listen 80;
    server_name apt.example.com;
    root /srv/urapt-web/dist;

    location / { try_files $uri /index.html; }

    location /api/ { proxy_pass http://127.0.0.1:8080; }
    location /apt/  { proxy_pass http://127.0.0.1:8080; }
}
```

## Architecture

```
src/
  lib/api/       typed client mirroring urapt's shared/api + shared/models
                 (client, errors, endpoints, types)
  stores/        Zustand auth store (persisted token)
  hooks/         shared server/user queries, logout
  components/    app shell, common UI (copy button, confirm, states), shadcn ui
  features/      auth, setup, repos (tabs), tokens, users, settings
  routes.tsx     route tree with lazy-loaded pages + protected/admin guards
```

The API client (`src/lib/api/`) is hand-written and typed against urapt's Go
DTOs in `shared/api/api.go` and `shared/models/models.go`. A `401` anywhere
clears the local session and redirects to `/login`.

## License

GPL-3.0-or-later, matching the urapt project.
