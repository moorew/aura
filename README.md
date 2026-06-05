# Sempa

A self-hosted personal task manager for everyone.

Plan your day, track focused work, and end each day with intention — with your email and calendar pulled in automatically.

---

## Features

- **Daily Kanban** — drag tasks across a week view, plan each day
- **Email → Tasks** — import starred Gmail or Fastmail emails as tasks
- **Schedule panel** — see calendar events alongside your tasks
- **Pomodoro + timeboxing** — schedule focused blocks, track sessions per task
- **Weekly review** — set objectives, review what shipped, plan ahead
- **Shutdown ritual** — guided end-of-day reflection
- **Jira sync** — bi-directional: import assigned issues, mark done in Sempa to close the ticket
- **Recurring tasks** — daily, weekly, and monthly templates
- **Keyboard shortcuts** — `n` new task, `t` today, `j/k` prev/next week, `?` help

### Apps

| Platform | How to get it |
|----------|--------------|
| **Web** | Self-host with Docker (see below) |
| **Android** | APK from [GitHub Releases](../../releases) or build from source |
| **Windows** | `.msi` installer from [GitHub Releases](../../releases) (x64 + ARM64) |
| **PWA** | Install from your browser when visiting your Sempa instance |

All apps connect to your self-hosted server — your data stays on your machine.

---

## Quick start

**Prerequisites:** Docker and Docker Compose (v2).

```bash
git clone https://github.com/moorew/sempa.git
cd sempa
bash install.sh
```

The installer asks a few questions (URL, auth method, optional API keys), writes your config, builds the image, and starts the container. The whole process takes about 2 minutes.

Open the URL it prints and follow the in-app setup wizard to connect your email and calendar.

---

## Self-hosting with Tailscale (recommended)

Tailscale is the easiest way to access Sempa securely from all your devices without exposing it to the public internet.

### Why Tailscale?

- **No port forwarding** — access your server from anywhere on your tailnet
- **Automatic HTTPS** — Tailscale provides TLS certificates via MagicDNS
- **Zero-trust networking** — only your devices can reach the server
- **Works on all platforms** — desktop, mobile, and headless servers

### Setup

1. **Install Tailscale** on your server and all devices you want to access Sempa from: [tailscale.com/download](https://tailscale.com/download)

2. **Run the installer:**
   ```bash
   bash install.sh
   ```
   When asked for the URL, use your Tailscale machine name:
   ```
   https://your-machine.tail1234.ts.net
   ```

3. **Generate a Tailscale auth key** at [Tailscale Admin → Keys](https://login.tailscale.com/admin/settings/keys) and paste it when the installer asks for `TS_AUTHKEY`. This lets the Docker sidecar join your tailnet automatically.

4. **Enable HTTPS** (optional but recommended):
   ```bash
   tailscale cert your-machine.tail1234.ts.net
   ```
   The bundled `ts-sempa` Docker container handles this automatically.

5. **Connect your phone/desktop app**: Open the app, enter your Tailscale URL (e.g. `https://sempa.tail1234.ts.net`) in the server field, and sign in.

### Alternative: any reverse proxy

Sempa works behind any reverse proxy (Caddy, nginx, Traefik). Set `APP_URL` to your public URL and configure the proxy to forward to port 9001. If you go this route, **make sure you have authentication enabled** (Google OAuth or username/password).

---

## Manual setup

If you prefer to configure things by hand:

**1. Clone the repo**

```bash
git clone https://github.com/moorew/sempa.git
cd sempa
```

**2. Create `.env`** (Docker Compose variable substitution)

```bash
cp .env.example .env
# Edit .env and set APP_URL to wherever Sempa will live
```

**3. Create `.env.local`** (secrets — never committed)

```bash
cp .env.local.example .env.local
# Fill in your credentials (see Configuration below)
```

**4. Build and start**

```bash
docker compose build
docker compose up -d
```

**5. Open the app**

Navigate to `APP_URL` in your browser. The first-run wizard will guide you through connecting integrations.

---

## Configuration

All configuration is in two files that you create locally:

| File | Purpose |
|------|---------|
| `.env` | Infrastructure (URL, port) — Docker Compose reads this for variable substitution |
| `.env.local` | Secrets (API keys, credentials) — loaded into the container |

### `.env`

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_URL` | `http://localhost:9001` | The URL where Sempa is accessible (no trailing slash) |
| `HOST_PORT` | `9001` | The port to expose on the host |

### `.env.local`

#### Authentication

Sempa supports two auth methods. You can enable one or both.

**Google Sign-In (recommended)**

Uses OAuth — you sign in with your Google account, no password to manage.

```dotenv
GMAIL_CLIENT_ID=your-client-id.apps.googleusercontent.com
GMAIL_CLIENT_SECRET=your-secret
# Comma-separated list of allowed Google emails.
# Leave unset to allow any Google account.
SEMPA_ALLOWED_EMAILS=you@gmail.com
```

Setup steps:
1. Go to [Google Cloud Console → Credentials](https://console.cloud.google.com/apis/credentials)
2. Create an OAuth 2.0 Client ID (Web application)
3. Add an Authorised redirect URI: `{APP_URL}/api/v1/auth/google/callback`
4. Copy the Client ID and Secret into `.env.local`

> The same credentials are used for Gmail integration — you only need one OAuth client for everything.

**Username & password**

```dotenv
SEMPA_USERNAME=admin
SEMPA_PASSWORD=your-strong-password
```

If `SEMPA_PASSWORD` is not set, auth is disabled entirely (fine for local-only installs on a trusted network like Tailscale).

#### Tailscale (optional)

If you use the bundled Tailscale sidecar (`ts-sempa` service in `docker-compose.yml`), add your auth key:

```dotenv
TS_AUTHKEY=tskey-auth-...
```

Generate one at [Tailscale Admin → Keys](https://login.tailscale.com/admin/settings/keys). The key is read by the `ts-sempa` container to join your tailnet.

#### Optional

| Variable | Description |
|----------|-------------|
| `TS_AUTHKEY` | Auth key for the Tailscale sidecar container |
| `ANTHROPIC_API_KEY` | Enables AI-powered task title cleanup when importing emails |
| `EMAIL_FORWARD_TOKEN` | Secret token for the Cloudflare email-to-task webhook |
| `SMTP_PORT` | Port for the built-in inbound SMTP server (default: `2525`) |
| `INBOX_POLL_INTERVAL` | How often to poll the email inbox (default: `1m`) |

---

## Integrations

All integrations are optional and configured through the Settings UI after first login.

| Integration | What it does |
|-------------|-------------|
| **Gmail** | Imports starred emails as tasks. Uses the same OAuth app as sign-in. |
| **Google Calendar** | Shows today's events in the Schedule panel. Enabled via the Gmail settings page. |
| **Fastmail** | Imports starred emails as tasks via IMAP. App password required. |
| **Fastmail Calendar** | Syncs JMAP calendar events into the Schedule panel. |
| **Jira** | Imports assigned issues as tasks. Marking a Jira-sourced task done closes the ticket. |
| **Calendar feeds (ICS)** | Subscribe to any `.ics` / webcal URL for read-only events. |
| **Email inbox** | Forward any email to a Fastmail address to auto-create a task. |

---

## Connecting mobile & desktop apps

The Android app and Windows desktop app connect to your self-hosted server:

1. **Install the app** from [GitHub Releases](../../releases)
2. **Open the app** — you'll see a "Server URL" field
3. **Enter your server address** (e.g. `https://sempa.tail1234.ts.net`)
4. **Sign in** with your Google account or username/password

Both your phone and server must be on the same Tailscale network (or the server must be reachable from your phone's network).

> **Tip:** Install Tailscale on your phone to access your server from anywhere, even on mobile data.

---

## Upgrading

```bash
git pull
docker compose build
docker compose up -d
```

Database migrations run automatically on startup. Your data is in a Docker volume (`sempa_data`) and is preserved across rebuilds.

---

## Development

**Requirements:** Go 1.21+, Node.js 20+

```bash
# Backend (runs on :9001)
cd backend
go run ./cmd/server/

# Frontend (runs on :5173, proxies API to :9001)
cd frontend
npm install
npm run dev
```

The frontend dev server sets `VITE_API_URL=http://localhost:9001` automatically via `.env.development`. You can set `SEMPA_PASSWORD=dev` in your shell to enable auth locally.

### Building native apps

```bash
# Android (requires Android SDK)
cd frontend
npx cap sync android
npx cap open android   # opens in Android Studio

# Windows (requires Rust toolchain)
cd frontend
npm run tauri build
```

### Project structure

```
backend/
  cmd/server/        Entry point
  internal/
    api/             HTTP handlers
    config/          Environment-based config
    db/              SQLite stores + migrations
    integrations/    External service clients (Gmail, Fastmail, Jira, iCal)
frontend/
  src/
    routes/          SvelteKit pages
    lib/
      components/    Reusable UI components
      stores/        Svelte runes-based state
      api.ts         Typed API client
  src-tauri/         Tauri (Windows/macOS/Linux) desktop app
  android/           Capacitor Android wrapper
deploy/
  update.sh          Pull + rebuild script
```

---

## Philosophy

- **Single-user per instance.** Each person runs their own copy — like Gitea or Vaultwarden. Your data stays on your server.
- **No cloud dependency.** Runs fully offline once configured. External services (Gmail, Jira) are optional integrations.
- **Small footprint.** ~10 MB Docker image, ~20 MB RAM. SQLite database — no separate database server.
- **API-first.** Everything the frontend does goes through the REST API.

---

## Roadmap

- [x] Android app (Capacitor)
- [x] Windows desktop app (Tauri)
- [ ] Slack integration
- [ ] CalDAV write-back (create Sempa tasks as calendar events)
- [ ] Public Docker image on GitHub Container Registry

---

## License

MIT
