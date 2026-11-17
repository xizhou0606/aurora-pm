FROM oven/bun:1.4 AS build
WORKDIR /app

# Copy root workspace config
COPY package.json bun.lock tsconfig.base.json ./

# Copy app package files
COPY apps/ai-runtime/package.json apps/ai-runtime/
COPY apps/ai-runtime/tsconfig.json apps/ai-runtime/

# bun.lock pins the resolution; bun installs from the default npmjs registry.
RUN bun install --frozen-lockfile

# Copy app source
COPY apps/ai-runtime/src apps/ai-runtime/src
COPY apps/ai-runtime/scripts apps/ai-runtime/scripts
COPY apps/ai-runtime/skills apps/ai-runtime/skills

ARG SYNC_QA_SKILL=true
ARG QA_SKILL_REF=main
ARG QA_SKILL_COMMIT=
ARG GITHUB_TOKEN=
ENV QA_SKILL_REF=${QA_SKILL_REF}
ENV QA_SKILL_COMMIT=${QA_SKILL_COMMIT}
ENV GITHUB_TOKEN=${GITHUB_TOKEN}

WORKDIR /app/apps/ai-runtime
RUN if [ "$SYNC_QA_SKILL" = "true" ]; then bun scripts/sync-qa-test-case-skill.mjs; fi
RUN bun --bun run build

FROM oven/bun:1.4
WORKDIR /app
ENV NODE_ENV=production
# git: 启动器要在挂载的发布仓库(/repo)上收集提交窗口。
# deb.debian.org 直连在 pm-prod 实测 ~116 kB/s（apt 阶段 4-6 分钟），
# 换腾讯云内网镜像源后为秒级；trixie 的 deb822 源在 debian.sources。
RUN sed -i 's|deb.debian.org|mirrors.cloud.tencent.com|g; s|security.debian.org|mirrors.cloud.tencent.com|g' /etc/apt/sources.list.d/debian.sources \
    && apt-get update \
    && apt-get install -y --no-install-recommends git \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /app/package.json /app/bun.lock ./
COPY --from=build /app/apps/ai-runtime/package.json apps/ai-runtime/

RUN bun install --production --frozen-lockfile

COPY --from=build /app/apps/ai-runtime/dist apps/ai-runtime/dist
COPY --from=build /app/apps/ai-runtime/skills apps/ai-runtime/skills

USER bun
EXPOSE 8091
# 与 collaboration 同款官方模式：bun -e 内联 fetch，Jenkins wait_container
# 据此给出真实健康判定（无此声明时只确认 running）。
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s CMD bun -e "fetch('http://127.0.0.1:8091/health/live').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"
# 公告生成以最佳努力后台运行,失败只打日志,绝不阻塞服务启动。
CMD ["sh", "-c", "bun apps/ai-runtime/dist/release-notes-on-start.js & exec bun apps/ai-runtime/dist/server.js"]
