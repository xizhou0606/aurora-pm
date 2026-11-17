#!/bin/sh
set -eu

test_dir="$(mktemp -d)"
trap 'rm -rf "$test_dir"' EXIT INT TERM

mock_bin="$test_dir/bin"
runtime_file="$test_dir/runtime-config.js"
nginx_template="$test_dir/nginx.template.conf"
nginx_file="$test_dir/nginx.conf"
mkdir -p "$mock_bin"

printf '#!/bin/sh\nexit 0\n' >"$mock_bin/nginx"
chmod +x "$mock_bin/nginx"
printf 'add_header Content-Security-Policy "frame-ancestors __PM_FRAME_ANCESTORS__" always;\n' >"$nginx_template"

PATH="$mock_bin:$PATH" \
PM_RUNTIME_CONFIG_PATH="$runtime_file" \
PM_NGINX_TEMPLATE_PATH="$nginx_template" \
PM_NGINX_CONFIG_PATH="$nginx_file" \
PM_FRAME_ANCESTORS="'self' https://one2all.example.com" \
PM_API_BASE_URL="https://pm-api.example.com" \
PM_HOST_API_BASE_URL="https://one2all-api.example.com" \
PM_FEISHU_LOGIN_ENABLED="true" \
PM_FEISHU_LOGIN_APP_ID="feishu-app-id-must-not-reach-browser" \
PM_FEISHU_LOGIN_APP_SECRET="feishu-app-secret-must-not-reach-browser" \
PM_NATIVE_WORK_ITEMS_MODE="iframe" \
PM_NATIVE_BI_SCREEN_MODE="native" \
PM_NATIVE_QUALITY_MODE="iframe" \
PM_PRODUCT_NAME='Acme "Delivery"' \
PM_APP_VERSION='1.2.3-rc.1' \
PM_VERSION_SUMMARY='["新增设备授权","修复登录问题"]' \
PM_RELEASE_DATE='2026-09-01' \
PM_PRIMARY_COLOR="#0e7490" \
PM_LOGIN_SUBTITLE='项目<交付>' \
/bin/sh "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/docker-entrypoint.sh"

grep -q '"PM_API_BASE_URL":"https://pm-api.example.com"' "$runtime_file"
grep -q '"PM_HOST_API_BASE_URL":"https://one2all-api.example.com"' "$runtime_file"
if grep -q 'PM_FEISHU_LOGIN_ENABLED' "$runtime_file"; then
  echo "retired Feishu login flag leaked into runtime config" >&2
  exit 1
fi
if grep -Eq 'PM_NATIVE_(INBOX|WORK_ITEMS|BI_SCREEN|QUALITY)_MODE|PM_EXTERNAL_APP_URL' "$runtime_file"; then
  echo "retired native module flag leaked into runtime config" >&2
  exit 1
fi
if grep -Eq 'PM_FEISHU_LOGIN_APP_(ID|SECRET)|feishu-app-(id|secret)-must-not-reach-browser' "$runtime_file"; then
  echo "Feishu provider credential leaked into runtime config" >&2
  exit 1
fi
grep -q '"PM_PRODUCT_NAME":"Acme \\"Delivery\\""' "$runtime_file"
grep -q '"PM_APP_VERSION":"1.2.3-rc.1"' "$runtime_file"
grep -q '"PM_VERSION_SUMMARY":"\[\\"新增设备授权\\",\\"修复登录问题\\"\]"' "$runtime_file"
grep -q '"PM_RELEASE_DATE":"2026-09-01"' "$runtime_file"
grep -q '"PM_PRIMARY_COLOR":"#0e7490"' "$runtime_file"
grep -q '"PM_LOGIN_SUBTITLE":"项目\\u003c交付\\u003e"' "$runtime_file"
grep -q "frame-ancestors 'self' https://one2all.example.com" "$nginx_file"

default_runtime_file="$test_dir/runtime-config-defaults.js"
PATH="$mock_bin:$PATH" \
PM_RUNTIME_CONFIG_PATH="$default_runtime_file" \
PM_NGINX_TEMPLATE_PATH="$nginx_template" \
PM_NGINX_CONFIG_PATH="$nginx_file" \
/bin/sh "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/docker-entrypoint.sh"

if grep -Eq 'PM_NATIVE_|PM_EXTERNAL_APP_URL' "$default_runtime_file"; then
  echo "retired runtime config leaked into defaults" >&2
  exit 1
fi

repo_nginx="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/nginx.conf"
if ! grep -q 'location /.well-known/' "$repo_nginx"; then
  echo "pm-web nginx must proxy /.well-known/ to pm-api for CLI/MCP OAuth discovery" >&2
  exit 1
fi
