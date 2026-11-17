#!/bin/sh
set -eu

output="${PM_RUNTIME_CONFIG_PATH:-/usr/share/nginx/html/runtime-config.js}"
nginx_template="${PM_NGINX_TEMPLATE_PATH:-/etc/nginx/conf.d/default.conf}"
nginx_output="${PM_NGINX_CONFIG_PATH:-/etc/nginx/conf.d/default.conf}"
frame_ancestors="${PM_FRAME_ANCESTORS:-'self'}"

if ! printf '%s' "$frame_ancestors" | grep -Eq "^[A-Za-z0-9:/._*' -]+$"; then
  echo "PM_FRAME_ANCESTORS contains unsupported characters" >&2
  exit 1
fi

if grep -q '__PM_FRAME_ANCESTORS__' "$nginx_template"; then
  rendered_nginx="${nginx_output}.tmp"
  sed "s|__PM_FRAME_ANCESTORS__|$frame_ancestors|g" "$nginx_template" >"$rendered_nginx"
  mv "$rendered_nginx" "$nginx_output"
fi

json_escape() {
  printf '%s' "$1" |
    tr -d '\r\n' |
    sed 's/\\/\\\\/g; s/"/\\"/g; s/</\\u003c/g; s/>/\\u003e/g'
}

printf 'window.__RUNTIME_CONFIG__ = {' >"$output"
printf '"PM_API_BASE_URL":"%s",' "$(json_escape "${PM_API_BASE_URL:-}")" >>"$output"
printf '"PM_HOST_API_BASE_URL":"%s",' "$(json_escape "${PM_HOST_API_BASE_URL:-}")" >>"$output"
printf '"PM_PRODUCT_NAME":"%s",' "$(json_escape "${PM_PRODUCT_NAME:-PM 项目管理}")" >>"$output"
printf '"PM_APP_VERSION":"%s",' "$(json_escape "${PM_APP_VERSION:-}")" >>"$output"
printf '"PM_VERSION_SUMMARY":"%s",' "$(json_escape "${PM_VERSION_SUMMARY:-}")" >>"$output"
printf '"PM_RELEASE_DATE":"%s",' "$(json_escape "${PM_RELEASE_DATE:-}")" >>"$output"
printf '"PM_PRODUCT_SHORT_NAME":"%s",' "$(json_escape "${PM_PRODUCT_SHORT_NAME:-PM}")" >>"$output"
printf '"PM_LOGO_URL":"%s",' "$(json_escape "${PM_LOGO_URL:-}")" >>"$output"
printf '"PM_PRIMARY_COLOR":"%s",' "$(json_escape "${PM_PRIMARY_COLOR:-#2563eb}")" >>"$output"
printf '"PM_LOGIN_TITLE":"%s",' "$(json_escape "${PM_LOGIN_TITLE:-}")" >>"$output"
printf '"PM_LOGIN_SUBTITLE":"%s",' "$(json_escape "${PM_LOGIN_SUBTITLE:-}")" >>"$output"
printf '"PM_LOGIN_BACKGROUND_COLOR":"%s",' "$(json_escape "${PM_LOGIN_BACKGROUND_COLOR:-#eef2f7}")" >>"$output"
printf '"PM_LOGIN_BACKGROUND_IMAGE":"%s",' "$(json_escape "${PM_LOGIN_BACKGROUND_IMAGE:-}")" >>"$output"
printf '"PM_TIMEZONE":"%s"' "$(json_escape "${PM_TIMEZONE:-Asia/Shanghai}")" >>"$output"
printf '};\n' >>"$output"

exec nginx -g 'daemon off;'
