# PM 文档协作运行时基础

本目录从 ADR-0016 的 P15 纵向验证演进为 P16 Node 运行时基础。它已与 Web provider、Go internal authority、
权威 PostgreSQL persistence 和 app composition 接线，用真实 Hocuspocus WebSocket、Yjs 和最小权限 HTTP bridge
实现鉴权、只读写保护、撤权、schema/引用校验、逐 update actor/operation、持久确认与失败隔离。它仍未通过
P19 多实例/真实代理联合验收，也未部署或执行文档 cutover，因此不能据此宣称 production ready。

## 技术栈与边界

- Bun 1.4 或更高版本（官方 `oven/bun` 运行时承载 Hocuspocus 4.6.0）。
- Hocuspocus server/provider 4.6.0。
- Tiptap 3.29.2 和 Yjs 13.6.32，与现有前端保持一致，不升级编辑器。
- Node 不读取 PM 数据库，不解析用户角色，也不接受浏览器自报的 userId、workspace 或权限。
- 浏览器连接使用 Go 签发的短期、资源限定 `connectionTicket`；Node 以独立服务身份向 Go 兑换最小 actor/access 投影。

## 数据流

```text
browser ticket -> Hocuspocus onAuthenticate -> Go authorize
                                      | 成功后
                                      v
                                Go load snapshot

editor update -> candidate Y.Doc -> schema/size validation
                              -> Go reference/access validation receipt
                              -> in-memory Yjs sync
                              -> Go CAS durable commit
                              -> durable_commit acknowledgement

commit failure -> close every room connection -> quarantine stale Y.Doc
                                      -> unload room
                                      -> next connection reloads Go durable snapshot
```

Hocuspocus 的 `synced` 和 `SyncStatus(true)` 只表示 update 已被内存文档接收。客户端只有收到
`durable_commit`，并确认其中 state vector 覆盖本地文档后，才能显示“已保存”。`src/durability.ts` 提供了这个判定。

## 本地验证

从仓库根目录运行：

```bash
bun install --frozen-lockfile
(cd apps/collaboration && bun --bun run typecheck)
(cd apps/collaboration && bun --bun run test)
(cd apps/collaboration && bun --bun run build)
```

测试只启动环回地址上的临时 HTTP/WebSocket 服务并使用内存数据，不连接业务或生产数据库。

## 启动

先构建，再提供一个实现下列 `/internal/collaboration/v1/*` 候选合同的隔离 authority：

```bash
cd apps/collaboration
PM_COLLAB_AUTHORITY_URL=http://127.0.0.1:18080 \
PM_COLLAB_SERVICE_TOKEN=local-secret-from-untracked-env \
bun run start
```

默认只监听 `127.0.0.1:1234`。运行时不会写本地数据、缓存或日志文件；Yjs 文档仅驻留内存，权威 snapshot、
projection、revision 和审计必须由 Go 持久化。服务身份不得提交到仓库或写入日志。

必要配置：

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PM_COLLAB_AUTHORITY_URL` | 无 | Go authority 基础地址，必填 |
| `PM_COLLAB_SERVICE_TOKEN` | 无 | 最小权限服务身份，必填且敏感 |
| `PM_COLLAB_ADDRESS` | `127.0.0.1` | 监听地址；真实部署只能在受控网络和代理后调整 |
| `PM_COLLAB_PORT` | `1234` | HTTP/WebSocket 端口 |
| `PM_COLLAB_AUTHORITY_TIMEOUT_MS` | `2000` | 单次 authority 请求超时 |
| `PM_COLLAB_MAX_CONNECTIONS` | `200` | 单实例连接上限 |
| `PM_COLLAB_MAX_DOCUMENTS` | `100` | 单实例活跃文档上限 |
| `PM_COLLAB_MAX_PAYLOAD_BYTES` | `262144` | 单 WebSocket payload 上限 |
| `PM_COLLAB_REVALIDATE_CEILING_MS` | `5000` | 空闲连接最大复验间隔 |

`GET /healthz` 只报告进程存活；`GET /readyz` 会调用 authority health，依赖不可用时返回 503。

## 候选 authority 合同

所有请求都必须携带服务 Bearer token 和 `X-PM-Collaboration-Contract: 1`，响应及错误均禁止包含正文、
connection ticket 或凭据。候选端点为：

- `authorize`：短期 ticket + canonical room -> grant/actor/level/policy revision/expiry。
- `revalidate`：在每次写入和空闲定时器上重查 grant；撤权或过期 fail closed。
- `load`：只在鉴权成功后返回已提交 Yjs snapshot 及 snapshot/projection hash。
- `validate-mutation`：对完整候选 projection、schema 与资产/表格引用签发短期、snapshot-bound receipt。
- `commit`：同时校验 grant、receipt、policy revision、资源引用和 durable revision CAS 后原子持久化。

Go 侧已挂载与这些路径匹配的窄 internal handler/DTO，并组合受控 service-token verifier、真实 ticket/grant、
validation receipt、resourceaccess、document validator 与 PostgreSQL repository。这些路径只属于内部服务合同，
不得注册为业务公开 API。

## 反向代理与生命周期

真实接入必须由同源 TLS 代理暴露专用 WebSocket 路径，启用 upgrade、空闲超时、连接/payload 限额和
`no-store`；代理不得记录 Hocuspocus auth frame 或服务凭据。Node 到 Go 走受控内部地址，服务身份只允许上述
五个 bridge 操作，不能获得用户超管或任意数据库权限。

收到 `SIGINT`/`SIGTERM` 后进程停止接入、关闭 WebSocket，并由 Hocuspocus flush 待处理 store。异常退出时只保证
恢复 Go 已确认的 durable revision；内存已同步但未确认的更新不会被描述为已保存。备份和恢复对象属于 Go 的
Yjs snapshot/update log 与投影，不是 Node 内存。

## 已知门禁

## 镜像与 GitLab CI

仓库根执行 `docker build -f apps/collaboration/Dockerfile -t pm-collaboration:local .`。
镜像仅包含生产依赖与构建输出，使用非 root 用户；不包含 `.env`、数据库凭据或测试数据。
`collaboration-test` 执行类型检查、25 个运行时测试、构建及部署合同检查；
`build-pm-collaboration` 在 main/default 或 dev 分支通过该检查后发布 Harbor 镜像。
所需 Harbor/共享模板变量沿用根 `.gitlab-ci.yml`；构建镜像使用官方 `oven/bun:1.4`。

Kubernetes 使用既有 `deploy/k8s/production/render.sh` / `apply.sh`，默认不开启。
在私有 `production.env` 配置 `PM_COLLAB_ENABLED=true` 与 CI 发布的
`PM_COLLAB_IMAGE=仓库地址@sha256:完整摘要`，在预置 `pm-secrets` 中设置
`PM_COLLAB_SERVICE_TOKEN`（至少 32 字符）及 `PM_COLLAB_TOKEN_KEY`（恰好 32 字节）。
Node 只读取 service token；Go 读取两个密钥。渲染不创建 Secret。
两个密钥的职责区别（勿混淆）：
- `PM_COLLAB_SERVICE_TOKEN` 是 Node 调用 Go authority HTTP 接口的服务间认证令牌；
- `PM_COLLAB_TOKEN_KEY` 是 Go 后端 `securetoken`（AES-256-GCM）的密钥，仅 Go 持有，
  只用于签发/验证 TTL ≤ 5 分钟的短期票据——`connection-ticket`（浏览器持票连 Node，
  Node 回调 Go 验票）、`connection-grant`（Node 提交 mutation 时携带的授权证明）、
  `validation-receipt`（校验收据）。它**不加密文档内容**：文档照常经 Go 落库，
  轮换该 key 不会使已保存文档不可读，只是让在途票据失效（协同用户重连即可），
  改后重启 pm-api、Node 无需改动。密钥值不写进仓库/工单/日志。
公开地址由 PM HTTPS 地址生成 `/collaboration`，前端从 Go ticket 响应获取 WebSocket 地址，
不需要新增前端构建变量。HTTP NodePort 模式拒绝开启生产共编。

当前固定单副本 Recreate，包含独立存活/就绪探测与 60 秒退出宽限，不配置 HPA。
镜像由新增 GitLab 作业发布；现有本地 `build-and-push.sh` 仍只构建原三个镜像，
协作镜像摘要通过上述独立变量提供，不写进旧三镜像 lock 文件。
关闭开关会关闭 Go authority；已有 Node Deployment 不被自动删除，需按停写/排空流程另行下线。
这些配置不会执行旧文档 cutover，也不代表多实例生产验收完成。

## 待完成验收

- 当前 WebSocket 测试仍使用 HTTP 内存 authority double；Go 生产组合与 app mount 已完成，但真实 PostgreSQL、
  双 Node/API 实例、反向代理故障和 CAS 恢复运行尚未完成。
- 运行时固定 `debounce: 0`，用 document 级 mutation lock 串行化 validate/apply，并按接收顺序保存精确 raw update、
  candidate snapshot/state-vector、receipt 和 actor grant；`onStoreDocument.lastContext` 不参与归因。未来不得直接启用
  debounce，否则 callback 合并会重新破坏逐 update 审计。
- commit 失败会断开整间房、清空待提交队列、隔离并卸载 stale Y.Doc；下一连接只从 Go durable snapshot 重载。
  单实例已验证失败 update 不会混入后续提交；P19 仍须验证多实例同时广播、响应丢失但 Go 已提交、代理断连与重试。
- Go durable ack 仍是唯一“已保存”信号；内存同步和失败前短暂可见不得展示为保存成功。
- Node 投影已使用真实 `image`/`bitable` JSON 节点并只抽取 `assetId` 与稳定表格引用；Go document owner 仍会
  独立执行 canonical schema/引用复验。完整编辑器 extension、评论锚点与 TableKit 兼容仍须由联合验收覆盖，不能把
  Node 的最小投影 schema 当成第二套 document registry。

详细选型结论见 [P15_REPORT.md](P15_REPORT.md)，本轮高优先级门禁见
[P16_NODE_GATE_REPORT.md](P16_NODE_GATE_REPORT.md)。

## CLI / MCP 正文替换

协作运行时提供受服务令牌保护的内部正文替换能力，Go 通过 `PM_COLLAB_INTERNAL_URL` 连接。有人连接或房间加载中时拒绝执行，替换期间禁止新连接进入。API 与本运行时都需要更新并重启，继续保持单副本 Recreate。契约、权限、存储和验证步骤见 [正文 Tool 说明](../../docs/tools/document-content-replacement.md)。
