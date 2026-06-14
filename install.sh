#!/usr/bin/env bash
# Sempa installer
# Usage (from the project root):
#   bash install.sh
#
# What this does:
#   1. Checks prerequisites (Docker + Compose)
#   2. Asks you a few questions
#   3. Writes .env and .env.local
#   4. Builds the Docker image
#   5. Starts Sempa

set -euo pipefail

# ── Colours ──────────────────────────────────────────────────────────────────
G='\033[0;32m'; Y='\033[1;33m'; R='\033[0;31m'; B='\033[1m'; D='\033[0;90m'; NC='\033[0m'
ok()   { echo -e "${G}  ✓${NC}  $*"; }
warn() { echo -e "${Y}  ⚠${NC}  $*"; }
err()  { echo -e "${R}  ✗${NC}  $*" >&2; exit 1; }
step() { echo -e "\n${B}$*${NC}"; }
dim()  { echo -e "${D}    $*${NC}"; }

ask()         { local -n _v=$2; read -rp "    $1: " _v; }
ask_default() { local -n _v=$2; read -rp "    $1 [${3}]: " _v; [[ -n "${_v}" ]] || _v="$3"; }
ask_secret()  { local -n _v=$2; read -rsp "    $1: " _v; echo; }
ask_yn()      { local -n _v=$2; read -rp "    $1 [y/N]: " _v; _v="${_v,,}"; }

rand_hex() { head -c 24 /dev/urandom | xxd -p | tr -d '\n'; }

# ── Header ───────────────────────────────────────────────────────────────────
echo ""
echo -e "${B}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${B}  Sempa — Personal Task Manager  •  Setup${NC}"
echo -e "${B}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# ── Sanity check ─────────────────────────────────────────────────────────────
if [[ ! -f "docker-compose.yml" ]]; then
  err "Run this script from the Sempa project root (where docker-compose.yml lives)."
fi

# ── Prerequisites ─────────────────────────────────────────────────────────────
step "Checking prerequisites"

if ! command -v docker &>/dev/null; then
  err "Docker is required. Install from https://docs.docker.com/get-docker/"
fi
ok "Docker $(docker --version | awk '{gsub(/,/,"",$3); print $3}')"

if ! docker compose version &>/dev/null 2>&1; then
  err "Docker Compose plugin is required.\n  Ubuntu/Debian: sudo apt-get install docker-compose-plugin\n  Docs: https://docs.docker.com/compose/install/"
fi
ok "Docker Compose $(docker compose version --short 2>/dev/null || docker compose version | grep -oP '[\d.]+' | head -1)"

# ── Existing install? ─────────────────────────────────────────────────────────
if [[ -f ".env" || -f ".env.local" ]]; then
  warn "Existing configuration found."
  ask_yn "Update it? (backs up current files first)" CONFIRM
  if [[ "$CONFIRM" != "y" ]]; then
    echo "  Nothing changed."
    echo "  To start Sempa: docker compose up -d"
    exit 0
  fi
  TS=$(date +%s)
  if [[ -f ".env" ]];       then cp .env       ".env.backup.$TS";       ok "Backed up .env → .env.backup.$TS"; fi
  if [[ -f ".env.local" ]]; then cp .env.local ".env.local.backup.$TS"; ok "Backed up .env.local → .env.local.backup.$TS"; fi
fi

# ── Step 1: URL ───────────────────────────────────────────────────────────────
step "1 / 4  —  App URL"
echo ""
dim "The URL where you (and others) will access Sempa."
dim "Examples:"
dim "  https://sempa.example.com          (custom domain with TLS)"
dim "  https://sempa.your-tailnet.ts.net  (Tailscale — the 'sempa' node, see README)"
dim "  http://192.168.1.10:9001           (local network)"
dim "  http://localhost:9001              (this machine only)"
echo ""
ask_default "App URL" APP_URL "http://localhost:9001"

HOST_PORT="9001"
if [[ "$APP_URL" == *"localhost"* || "$APP_URL" == *"127.0.0.1"* ]]; then
  # Extract port from URL if present, else ask
  MAYBE_PORT=""
  if [[ "$APP_URL" =~ :([0-9]+)$ ]]; then MAYBE_PORT="${BASH_REMATCH[1]}"; fi
  if [[ -n "$MAYBE_PORT" ]]; then
    HOST_PORT="$MAYBE_PORT"
  else
    ask_default "Host port" HOST_PORT "9001"
  fi
fi

# ── Step 2: Auth ──────────────────────────────────────────────────────────────
step "2 / 4  —  Authentication"
echo ""
echo "  How should users sign in?"
echo ""
echo "    1)  Google Sign-In  (recommended)"
echo "        Sign in with your Google account — no password to remember."
echo ""
echo "    2)  Username & password"
echo "        Simple, works without any Google account."
echo ""
read -rp "    Choice [1]: " AUTH_CHOICE
AUTH_CHOICE="${AUTH_CHOICE:-1}"

GOOGLE_CLIENT_ID=""; GOOGLE_CLIENT_SECRET=""; ALLOWED_EMAILS=""
SEMPA_USERNAME=""; SEMPA_PASSWORD=""

if [[ "$AUTH_CHOICE" == "1" ]]; then
  echo ""
  dim "You need a Google OAuth 2.0 Client ID."
  dim "Steps (takes ~2 minutes):"
  dim "  1. Go to https://console.cloud.google.com/apis/credentials"
  dim "  2. Click 'Create Credentials' → 'OAuth client ID'"
  dim "  3. Application type: Web application"
  dim "  4. Add Authorised redirect URI:"
  dim "       ${APP_URL}/api/v1/auth/google/callback"
  dim "  5. Copy the Client ID and Secret below."
  echo ""
  ask "Google Client ID" GOOGLE_CLIENT_ID
  ask_secret "Google Client Secret" GOOGLE_CLIENT_SECRET
  echo ""
  dim "Which Google email addresses are allowed to sign in?"
  dim "Comma-separated. Leave blank to allow any Google account."
  ask "Allowed email(s)" ALLOWED_EMAILS
else
  echo ""
  ask_default "Username" SEMPA_USERNAME "admin"
  while true; do
    ask_secret "Password" SEMPA_PASSWORD
    ask_secret "Confirm password" SEMPA_PASSWORD2
    if [[ "$SEMPA_PASSWORD" == "$SEMPA_PASSWORD2" ]]; then break; fi
    warn "Passwords don't match, try again."
  done
fi

# ── Step 3: Optional ─────────────────────────────────────────────────────────
step "3 / 4  —  Optional extras"
echo ""
dim "All optional — press Enter to skip. Email, calendar, and Jira accounts are"
dim "connected later inside the app (Settings), not here."
echo ""

# Local AI (Ollama + qwen) for text processing — opt-in.
dim "Local AI tidies imported email subjects into concise task titles, entirely"
dim "on your own server — nothing is sent to any third party. It runs Ollama with"
dim "the small qwen2.5:1.5b model (~1GB download, CPU-friendly). You can turn it"
dim "on/off later in Settings → Accounts → AI."
ask_yn "Use local AI for text processing?" AI_ENABLE
OLLAMA_BASE_URL=""; OLLAMA_MODEL=""
if [[ "$AI_ENABLE" == "y" ]]; then
  # sempa shares ts-sempa's (host) network namespace, so it reaches the
  # host-networked Ollama container over loopback. These two values prefill the
  # in-app AI fields via the backend's OLLAMA_BASE_URL / OLLAMA_MODEL config.
  OLLAMA_BASE_URL="http://127.0.0.1:11434"
  OLLAMA_MODEL="qwen2.5:1.5b"
  ok "Local AI enabled — Ollama will start and ${OLLAMA_MODEL} will be pulled."
fi
echo ""

# Tailscale sidecar
dim "Tailscale lets you reach Sempa securely from all your devices, with HTTPS"
dim "and no port-forwarding. Key: https://login.tailscale.com/admin/settings/keys"
TS_AUTHKEY=""
read -rp "    Tailscale auth key (optional): " TS_AUTHKEY

# Email-to-task webhook (opt-in)
echo ""
dim "Email → task: forward mail to a Cloudflare Email Routing address to create"
dim "tasks automatically. Enabling generates a webhook token for that worker."
ask_yn "Enable email-to-task?" EMAIL_ENABLE
EMAIL_TOKEN=""; SMTP_SENDERS=""
if [[ "$EMAIL_ENABLE" == "y" ]]; then
  EMAIL_TOKEN=$(rand_hex)
  ok "Generated email-forward token"
  dim "Restrict who can create tasks this way (comma-separated emails or @domain)."
  dim "Leave blank to accept any sender."
  read -rp "    Allowed senders (optional): " SMTP_SENDERS
fi

# Web Push contact (RFC 8292 `sub`). Derive a sensible default so browser/PWA
# notifications work without asking — the VAPID keys themselves auto-generate.
FIRST_EMAIL=$(echo "${ALLOWED_EMAILS}" | cut -d, -f1 | tr -d '[:space:]')
if [[ -n "$FIRST_EMAIL" ]]; then
  VAPID_SUBJECT="mailto:${FIRST_EMAIL}"
else
  HOST_ONLY=$(echo "$APP_URL" | sed -E 's#^https?://##; s#[:/].*$##')
  VAPID_SUBJECT="mailto:admin@${HOST_ONLY:-localhost}"
fi

# ── Step 4: Write config ──────────────────────────────────────────────────────
step "4 / 4  —  Writing config & starting Sempa"
echo ""

# .env  —  Docker Compose variable substitution (not secret)
cat > .env <<EOF
# Generated by install.sh — $(date)
APP_URL=${APP_URL}
HOST_PORT=${HOST_PORT}
EOF
# Local AI: activate the "ai" Compose profile (starts the ollama service) and
# point the backend at the loopback Ollama so the Settings fields prefill.
if [[ "$AI_ENABLE" == "y" ]]; then
  {
    echo "COMPOSE_PROFILES=ai"
    echo "OLLAMA_BASE_URL=${OLLAMA_BASE_URL}"
    echo "OLLAMA_MODEL=${OLLAMA_MODEL}"
  } >> .env
fi
ok "Written .env"

# .env.local  —  secrets loaded into the container
{
  echo "# Generated by install.sh — $(date)"
  [[ -n "$GOOGLE_CLIENT_ID" ]]     && echo "GMAIL_CLIENT_ID=${GOOGLE_CLIENT_ID}"
  [[ -n "$GOOGLE_CLIENT_SECRET" ]] && echo "GMAIL_CLIENT_SECRET=${GOOGLE_CLIENT_SECRET}"
  [[ -n "$ALLOWED_EMAILS" ]]       && echo "SEMPA_ALLOWED_EMAILS=${ALLOWED_EMAILS}"
  [[ -n "$SEMPA_USERNAME" ]]       && echo "SEMPA_USERNAME=${SEMPA_USERNAME}"
  [[ -n "$SEMPA_PASSWORD" ]]       && echo "SEMPA_PASSWORD=${SEMPA_PASSWORD}"
  [[ -n "$TS_AUTHKEY" ]]           && echo "TS_AUTHKEY=${TS_AUTHKEY}"
  [[ -n "$EMAIL_TOKEN" ]]          && echo "EMAIL_FORWARD_TOKEN=${EMAIL_TOKEN}"
  [[ -n "$SMTP_SENDERS" ]]         && echo "SMTP_ALLOWED_SENDERS=${SMTP_SENDERS}"
  [[ -n "$VAPID_SUBJECT" ]]        && echo "VAPID_SUBJECT=${VAPID_SUBJECT}"
} > .env.local
ok "Written .env.local"

# ── Build & start ─────────────────────────────────────────────────────────────
echo ""
echo "  Building Docker image (this takes a minute on first run)..."
docker compose build --quiet

echo "  Starting Sempa..."
docker compose up -d

# ── Health check ─────────────────────────────────────────────────────────────
echo "  Waiting for Sempa to be ready..."
READY=false
for _ in $(seq 1 30); do
  if curl -sf "http://localhost:${HOST_PORT}/api/v1/health" &>/dev/null; then
    READY=true; break
  fi
  sleep 1
done

echo ""
if $READY; then
  ok "Sempa is up and healthy."
else
  warn "Container started but health check timed out."
  warn "Check logs: docker compose logs sempa"
fi

# ── Local AI provisioning (Ollama + qwen) ──────────────────────────────────────
if [[ "$AI_ENABLE" == "y" ]]; then
  step "Setting up local AI (Ollama + ${OLLAMA_MODEL})"
  echo ""
  echo "  Waiting for Ollama to be ready..."
  OLLAMA_READY=false
  for _ in $(seq 1 30); do
    if curl -sf "http://127.0.0.1:11434/api/tags" &>/dev/null; then OLLAMA_READY=true; break; fi
    sleep 1
  done
  if $OLLAMA_READY; then
    ok "Ollama is reachable."
    echo "  Pulling ${OLLAMA_MODEL} (first run downloads ~1GB — this can take a few minutes)..."
    if docker compose exec -T ollama ollama pull "${OLLAMA_MODEL}"; then
      ok "Model ${OLLAMA_MODEL} downloaded."
    else
      warn "Couldn't pull ${OLLAMA_MODEL} automatically."
      warn "Pull it later with: docker compose exec ollama ollama pull ${OLLAMA_MODEL}"
    fi
    # Verify end-to-end: the same /api/tags check the in-app "Test connection" runs.
    if curl -sf "http://127.0.0.1:11434/api/tags" | grep -q "${OLLAMA_MODEL%%:*}"; then
      ok "AI connection verified — task-title cleanup is ready."
      dim "Prefilled in Settings → Accounts → AI (model ${OLLAMA_MODEL}, ${OLLAMA_BASE_URL})."
    else
      warn "Ollama is up but ${OLLAMA_MODEL} wasn't detected yet."
      warn "Re-check in Settings → Accounts → AI → Test connection once the pull finishes."
    fi
  else
    warn "Ollama didn't become ready in time. Check: docker compose logs ollama"
  fi
fi

# ── Summary ──────────────────────────────────────────────────────────────────
echo ""
echo -e "${B}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${G}  Sempa is running!${NC}"
echo -e "${B}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Open: ${APP_URL}"
echo ""
if [[ "$AUTH_CHOICE" == "1" && -n "$GOOGLE_CLIENT_ID" ]]; then
  echo -e "  ${Y}⚠  Reminder:${NC} add this Redirect URI in Google Cloud Console"
  echo "     if you haven't already:"
  echo "     ${APP_URL}/api/v1/auth/google/callback"
  echo ""
fi

echo "  Next steps — connect the rest inside the app (Settings):"
dim "  • Gmail / Fastmail        import starred email as tasks"
dim "  • Google / Fastmail cal   see events beside your tasks"
dim "  • Calendar feeds (ICS)    subscribe to any .ics / webcal URL"
dim "  • Jira                    import assigned issues as tasks"
dim "  • Notifications           enable Web Push / Android / webhook + sounds"
echo ""
if [[ "$EMAIL_ENABLE" == "y" ]]; then
  echo "  Email → task is enabled. Point a Cloudflare Email Worker at:"
  dim "  ${APP_URL}/api/v1/tasks/from-email   (Authorization: Bearer <token in .env.local>)"
  echo ""
fi

echo "  Useful commands:"
dim "  docker compose logs -f      stream logs"
dim "  docker compose stop         stop Sempa"
dim "  docker compose up -d        start Sempa"
dim "  bash install.sh             re-run this setup"
echo ""
