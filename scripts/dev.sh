#!/bin/sh
# Run a standalone local installation from a fresh clone.
set -eu
cd "$(dirname "$0")/.."
umask 077
mkdir -p .local
secrets=.local/credentials.env
if [ ! -f "$secrets" ]; then
  tmp=$(mktemp .local/credentials.XXXXXX)
  db_password=$(openssl rand -hex 24)
  admin_password=$(openssl rand -hex 18)
  instance=$(openssl rand -hex 6)
  printf 'PM_DEV_DB_PASSWORD=%s\nPM_BOOTSTRAP_ADMIN_PASSWORD=%s\nAURORA_INSTANCE=aurora-pm-%s\n' "$db_password" "$admin_password" "$instance" > "$tmp"
  ln "$tmp" "$secrets" || test -f "$secrets"
  rm -f "$tmp"
fi
set -a
. "./$secrets"
set +a
compose() { docker compose --project-name "${AURORA_INSTANCE:-aurora-pm-local}" --env-file "$secrets" -f compose.yaml "$@"; }
if [ "${1:-}" = down ]; then compose down; exit; fi
bun -e 'for (const port of [18088, 5188]) {
  try { Bun.listen({ hostname: "127.0.0.1", port, socket: { data() {} } }).stop(true) }
  catch { console.error("Local port " + port + " is busy"); process.exit(1) }
}'
bun install --frozen-lockfile
(cd backend && go build -o ../.local/pm-api ./cmd/pm-api)
compose up -d --wait
export GIN_MODE=release PM_ENV=development PM_LOCAL_PREVIEW=true PM_AUTH_MODE=local
export PM_DATABASE_DRIVER=postgres PM_DEV_DB_USER=pm PM_DEV_DB_NAME=pm_dev PM_DEV_DB_PORT=55439
export PM_HTTP_ADDR=127.0.0.1:18088 PM_SCHEMA_MIGRATION_MODE=apply
export PM_PUBLIC_BASE_URL=http://127.0.0.1:5188 VITE_DEV_PROXY_TARGET=http://127.0.0.1:18088
export VITE_DEV_HOST_AUTH_PROXY_TARGET="$VITE_DEV_PROXY_TARGET"
export PM_BOOTSTRAP_ADMIN_USERNAME=admin PM_BOOTSTRAP_ADMIN_DISPLAY_NAME='Local Admin'
export PM_BOOTSTRAP_WORKSPACE_NAME='Aurora Demo' PM_BOOTSTRAP_WORKSPACE_SLUG=aurora-demo PM_BOOTSTRAP_TEAM_NAME='Demo Team'
export PM_RBAC_MODE=enforce PM_RBAC_BOOTSTRAP_USER_ID=1 AI_ENABLED=false PM_AI_EMBEDDED_WORKER=false
export PM_GOOGLE_LOGIN_ENABLED=false PM_TELEGRAM_LOGIN_ENABLED=false PM_FEISHU_ENABLED=false
export PM_SSO_ENABLED=false PM_NOTIFICATION_ENABLED=false PM_OBJECT_STORAGE_ENABLED=false PM_REQUIREMENT_COVER_ENABLED=false
.local/pm-api >.local/api.log 2>&1 &
api=$!
web=''
cleanup() { if [ -n "$web" ]; then kill "$web" 2>/dev/null || true; fi; kill "$api" 2>/dev/null || true; }
trap cleanup EXIT
trap 'exit 130' INT TERM
count=0
while ! curl -fsS http://127.0.0.1:18088/health/ready >/dev/null 2>&1; do
  count=$((count+1))
  if ! kill -0 "$api" 2>/dev/null || [ "$count" -ge 120 ]; then
    echo 'API startup failed. See .local/api.log.' >&2
    exit 1
  fi
  sleep 1
done
# Bootstrap owns this dedicated workspace and user. Repeated starts preserve membership.
compose exec -T postgres psql -Xq -v ON_ERROR_STOP=1 -U pm -d pm_dev <<'SQL'
INSERT INTO workspace_members(workspace_id,user_id,role,enabled,created_at,updated_at)
SELECT w.id,u.id,'admin',true,now(),now() FROM workspaces w,users u
WHERE w.slug='aurora-demo' AND u.external_subject='local:admin'
ON CONFLICT(workspace_id,user_id) DO NOTHING;
SQL
echo 'Open http://127.0.0.1:5188 . Login: admin . Password: PM_BOOTSTRAP_ADMIN_PASSWORD in .local/credentials.env'
(cd apps/web && exec bun node_modules/@rsbuild/core/bin/rsbuild.js dev --config rsbuild.config.ts) &
web=$!
wait "$web"
