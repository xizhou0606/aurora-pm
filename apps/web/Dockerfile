ARG BUN_IMAGE=oven/bun:1.4
ARG NGINX_IMAGE=nginx:1.27-alpine
FROM ${BUN_IMAGE} AS build

WORKDIR /app

# Copy root workspace config (CHANGELOG.md feeds the in-app release notes)
COPY package.json bun.lock tsconfig.base.json CHANGELOG.md ./

# Copy app package files
COPY apps/web/package.json apps/web/

# bun.lock pins the resolution; bun installs from the default npmjs registry.
RUN bun install --frozen-lockfile

# Copy app source
COPY apps/web/ apps/web/

WORKDIR /app/apps/web
RUN bun --bun run build

FROM ${NGINX_IMAGE}

COPY apps/web/nginx.conf /etc/nginx/templates/default.conf
COPY apps/web/docker-entrypoint.sh /usr/local/bin/pm-web-entrypoint
COPY --from=build /app/apps/web/dist /usr/share/nginx/html
# The entrypoint renders the nginx template (substituting the CSP
# frame-ancestors value) into PM_NGINX_CONFIG_PATH. Ship no static server
# block in conf.d — not even the base image's welcome default.conf: a second
# `listen 80; server_name _` server collides and nginx silently keeps the
# first file alphabetically, which would disable the rendered config.
RUN rm -f /etc/nginx/conf.d/default.conf && chmod +x /usr/local/bin/pm-web-entrypoint

ENV PM_NGINX_TEMPLATE_PATH=/etc/nginx/templates/default.conf \
    PM_NGINX_CONFIG_PATH=/etc/nginx/conf.d/default.conf

EXPOSE 80
ENTRYPOINT ["/usr/local/bin/pm-web-entrypoint"]
