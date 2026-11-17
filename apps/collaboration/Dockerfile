ARG BUN_IMAGE=oven/bun:1.4
FROM ${BUN_IMAGE} AS build
WORKDIR /app
COPY package.json bun.lock ./
COPY apps/collaboration/package.json apps/collaboration/package.json
# bun.lock pins the resolution; bun installs from the default npmjs registry.
RUN bun install --frozen-lockfile
COPY apps/collaboration/src apps/collaboration/src
COPY apps/collaboration/tsconfig*.json apps/collaboration/
WORKDIR /app/apps/collaboration
RUN bun --bun run build

FROM ${BUN_IMAGE} AS dependencies
WORKDIR /app
COPY package.json bun.lock ./
COPY apps/collaboration/package.json apps/collaboration/package.json
RUN bun install --production --frozen-lockfile

FROM ${BUN_IMAGE} AS runtime
WORKDIR /app
ENV NODE_ENV=production PM_COLLAB_ADDRESS=0.0.0.0 PM_COLLAB_PORT=1234
# Preserve both Bun’s isolated dependency store and workspace dependency links.
COPY --from=dependencies /app/node_modules ./node_modules
COPY --from=dependencies /app/apps/collaboration/node_modules ./apps/collaboration/node_modules
COPY apps/collaboration/package.json ./apps/collaboration/package.json
COPY --from=build /app/apps/collaboration/dist ./apps/collaboration/dist
USER bun
EXPOSE 1234
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s CMD bun -e "fetch('http://127.0.0.1:1234/healthz').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"
CMD ["bun", "apps/collaboration/dist/index.js"]
