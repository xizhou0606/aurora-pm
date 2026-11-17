# PM 后端工程目录与模块边界

## 1. 架构结论

`backend/` 是一个 Go 模块、一个模块化单体和一个独立发布单元。PM 后端不按 API、Worker、导入
命令拆成多个需要独立版本、独立配置或独立发布的后端工程。

- `cmd/pm-api` 是 HTTP 入口；AI、通知等后台执行是同一应用的内部能力。代码可以按
  `internal/worker/<kind>` 分包，但不能演变成新的后端服务边界。
- 迁移、数据修复、数据导入命令是受控运维命令，不是常驻部署单元。
- 现有 `cmd/pm-worker` 和对应运行描述是同一发布单元内的执行角色：共享源码、镜像、版本、配置和
  数据库，不构成第二个工程。它不拥有独立的发布流水线、`go.mod`、镜像或部署目录；后续也不得扩展为
  独立服务。

本规范只定义后端代码和工程目录职责，不评审业务功能，也不改变现有 HTTP 路径、数据库表或接口字段。

## 2. 目录原则

- 目录先按业务领域划分，再在领域内部按适配器划分；不按全局 `service`、`repository`、`utils` 堆放。
- Gin、GORM、环境变量和第三方 SDK 都是边缘实现细节，不能成为业务规则的依赖方向。
- 模块之间通过明确的应用接口协作，不能读取对方的 GORM record、SQL repository 或表结构。
- `cmd/` 只保留入口和退出码处理；装配与生命周期在 `internal/app/`；业务规则不放入口文件。
- 不创建空目录凑结构，也不做全仓“只改路径”的重构。修改一个领域时，再将该领域完整地归位。
- 不新增泛化的 `common`、`base`、`helper`、`util`、`shared` 包。可复用代码必须有明确的业务或技术名称。

## 3. 目标目录

这是当前目录边界。尚未拆细到领域 HTTP adapter 的存量 HTTP 代码继续集中在
`internal/transport/http/`；其持久化 record 直接引用所属领域的 `module/<domain>/gorm` 包。

```text
backend/
├── cmd/                         # 可执行入口，只有参数、启动和退出码
│   ├── pm-api/                  # HTTP 入口
│   ├── pm-import-one2all-users/ # 原 One2All 用户表受控导入命令
│   ├── pm-migrate-legacy/       # 受控维护命令
│   ├── pm-repair-id-sequences/  # 受控维护命令
│   ├── pm-repair-team-scope/    # 受控维护命令
│   ├── pm-migrate-workflow-artifacts/ # 节点附件迁到需求材料与版本报告
│   ├── pm-worker/               # 同一发布单元的 AI 执行角色
│   ├── pm-cover-renderer/       # 同一发布单元的封面渲染进程角色
│   └── pm-cover-worker/         # 同一发布单元的需求封面执行角色
├── internal/
│   ├── app/                     # 组合根与进程生命周期，不放业务规则
│   │   ├── api/                 # API 及其内部后台执行器的统一装配
│   │   ├── worker/              # 存量 worker 执行角色的装配，不是独立工程
│   │   ├── coverrenderer/       # 封面渲染进程角色的装配
│   │   ├── coverworker/         # 需求封面执行角色的装配
│   │   └── maintenance/         # 各维护命令的参数解析和装配
│   ├── transport/
│   │   └── http/                # 无领域语义的 Gin、middleware、错误/响应映射
│   ├── module/                  # 纵向业务模块
│   │   ├── identity/
│   │   ├── project/
│   │   ├── requirement/
│   │   ├── task/
│   │   ├── quality/
│   │   ├── workflow/
│   │   ├── ai/
│   │   ├── notification/
│   │   ├── workbench/            # 工作台、BI 与全局菜单目录
│   │   ├── toolruntime/           # MCP/CLI 共用的协议无关工具契约
│   │   └── audit/
│   ├── integration/             # 一个目录对应一个外部系统/协议
│   │   ├── objectstore/
│   │   ├── feishu/
│   │   └── feishuim/
│   ├── worker/                  # 应用内后台任务的领取、重试与停止
│   │   ├── ai/
│   │   ├── outbox/
│   │   └── requirementcover/
│   ├── platform/                # 无业务语义的配置、数据库、认证、可观测性
│   │   ├── config/
│   │   ├── database/
│   │   └── auth/
│   └── maintenance/             # 仅由维护命令调用的导入、修复和校验逻辑
│       ├── legacyimport/
│       ├── one2alluserimport/
│       └── teamscoperepair/
├── scripts/                     # 后端本地运行、测试、迁移辅助脚本
├── deploy/                      # 后端自身的容器和本地运行描述
├── docs/                        # 运行、迁移和兼容性说明
└── migrations/                  # golang-migrate 可执行 SQL（{version}_{title}.up.sql）
```

一个完整领域只在自己的目录附近保留用例、策略、port 和领域类型；按需增加两个适配器子目录：

```text
internal/module/task/
├── service.go                    # 命令、查询、事务边界和跨模块协作
├── policy.go                     # 状态、权限、层级等规则
├── port.go                       # TaskStore、ObjectStore 等消费端接口
├── http/                         # task handler、DTO、路由注册
└── gorm/                         # task record、查询、TaskStore 实现
```

接口只在确实需要替换实现或测试隔离时定义，且放在消费者所在模块；不为目录整齐建立全局接口层。

## 4. 职责与依赖方向

```text
cmd/pm-api -> app/api
                -> transport/http -> module/<domain>/http -> module/<domain>
                -> worker/<kind>  -> module/<domain>
                -> module/<domain>/gorm, integration, platform

cmd/pm-<maintenance> -> app/maintenance -> maintenance -> module, platform
```

### `cmd/`

- 允许：读取启动参数、调用 `internal/app`、处理进程退出码。
- 禁止：SQL、Gin 路由、业务分支和直接业务编排。
- 在线 `cmd` 不新增独立发布的运行类型；后台能力仍属于同一 PM 后端发布单元。

### `internal/app/`

- 负责已校验配置、数据库和客户端的构造、依赖注入、启动及优雅停止。
- 是唯一允许同时依赖 transport、模块 GORM 适配器、integration 与 platform 的位置。
- 后台执行器与 API 的共同生命周期应在 `app` 层明确装配；不负责 HTTP 字段校验、权限决策或业务查询。

### `internal/transport/http/`

- 当前集中承载存量 Gin 初始化、认证 middleware、领域路由、handler、DTO 与响应映射；不得再建立第二个
  根级 HTTP 包。
- 新建或完整迁移的领域路由、handler 与 HTTP DTO 放在 `module/<domain>/http/`，由 transport 组合注册。
- 新建或完整迁移的 handler 只调用模块公开的 command/query；不得直接使用 `*gorm.DB`、GORM record 或外部 SDK。
- `gin.Context` 不得进入新增的 module、gorm、integration 或 worker API；先转换为 `context.Context` 和显式参数。

### `internal/module/<domain>/`

- 放该领域的用例编排、权限/状态规则、事务边界要求、跨模块协作契约和领域事件。
- 不导入 Gin、GORM、环境变量读取、数据库驱动或第三方业务 SDK。
- 持久化与外部系统只通过本模块定义的 port 使用；实现由 `app` 注入。
- 跨模块只依赖对方导出的应用接口，不能访问对方的 `gorm/`、record 或内部常量。

### `internal/module/<domain>/gorm/`

- 放本领域 GORM record、查询条件、repository 实现和索引相关访问。
- GORM record 不是 HTTP response，也不是跨模块契约；其他领域、handler、worker 不直接读取或写入它。
- 事务边界由模块用例决定，repository 接收调用方已建立的事务，不隐式开启跨用例事务。

### `internal/integration/`

- 一个目录封装一个外部系统或协议，只做认证、请求、超时、重试、响应转换和错误归一化。
- 收件人、文案、状态流转等 PM 业务决策属于领域模块，不属于 integration。
- 客户端接口由调用它的模块定义，不能建立全局 `client` 包。

### `internal/worker/`

- 放任务领取、租约、重试、取消、并发限制和优雅停止；实际业务执行交给模块用例。
- 不复用 HTTP handler，不依赖 Gin。
- 它是 PM 后端内部的职责分包，不是独立后端服务或新的发布目录。

### `internal/platform/`

- 只放无业务语义的运行能力：配置解析、数据库连接与事务基础设施、认证原语、日志/指标/追踪。
- `platform/database` 不承载领域查询；bootstrap 数据也不能隐式混入在线 schema migration。

### `internal/maintenance/`

- 放一次性或受控运维逻辑，只能被 `app/maintenance` 和对应维护命令调用。
- HTTP、在线 worker 和领域用例不得依赖 maintenance；维护命令必须保留 dry-run、明确 apply 和可审计报告。

### `scripts/`、`deploy/`、`docs/` 与迁移

- `backend/scripts/` 只放后端开发、测试、迁移和本地运行脚本，不读取前端或相邻工程的私有配置。
- `backend/deploy/` 只放本后端的运行材料；镜像、版本、环境变量和发布流水线只有一份，不由 API/worker
  各维护一套。临时存在的多个进程角色也必须从这同一发布单元启动。
- `backend/docs/` 放架构、运行和迁移说明；数据库文件、报告、上传物和渲染产物不得提交为源码文档。
- Postgres 可执行 schema 来源是 `backend/migrations/` 的 golang-migrate SQL
  （`{version}_{title}.up.sql` / `.down.sql`），由 `internal/platform/database.Migrate`
  在 `PM_SCHEMA_MIGRATION_MODE=apply` 时执行。已 apply 的文件不可改写。生产不新增该环境变量：`pm-api` 默认 `apply`，worker Deployment 已是 `validate`；不要改 `production.env` / `apply.sh`。
- `backend/docs/migrations/` 统一存放历史说明、来源清单、兼容性决策与迁移报告；它们不作为可执行迁移来源。
- SQLite 测试库和尚未 cutover 的空 Postgres 仍可走 GORM AutoMigrate；已有
  `schema_migrations` 的 Postgres 不再 AutoMigrate。系统默认行仍由启动 Bootstrap 幂等写入。

## 5. 当前代码归位状态

| 当前位置 | 目标归位 | 状态与新增规则 |
| --- | --- | --- |
| `internal/transport/http/` | 后端唯一的存量 HTTP adapter 汇集点 | 已完成物理迁移。新领域可渐进拆到 `module/<domain>/http`，但不得恢复根级 `internal/httpapi/`。 |
| `internal/compat/model/` | 删除，不再设立全局模型门面 | 已完成 record 物理迁移；所有调用方直接依赖所属 `module/<domain>/gorm` 包。 |
| `internal/platform/{config,database,auth}` | 运行时基础设施 | 已完成物理迁移；database schema 装配直接导入 record 所属领域包。 |
| `internal/integration/{objectstore,feishu,feishuim}` | 外部系统客户端 | 已完成物理迁移；PM 业务决策不得进入 integration。 |
| `internal/module/notification` | 通知意图、可复用链接、事件类型和事务 outbox 写入 | 已完成；只负责持久化事件，不投递外部消息。 |
| `internal/worker/outbox` | 通知 outbox 领取、重试与投递循环 | 已完成；通过 identity 查询 open_id，经 feishuim 投递。 |
| `internal/worker/ai` | AI 后台执行 | 已完成物理迁移。 |
| `internal/module/workflow/testcase` | 工作流测试用例契约和物化 | 已完成物理迁移。 |
| `internal/module/task/plan` | 任务计划领域计算 | 已完成物理迁移。 |
| `internal/module/workbench/` | `internal/module/workbench/` | 工作台、BI 与全局菜单注册/查询的 owning module；不得新增根级 navigation 包。 |
| `internal/module/toolruntime/` | 外部自动化工具目录、协议无关执行契约与 ChangeSet 策略 | MCP 与 CLI adapter 必须消费同一 canonical catalog；本模块不拥有协议帧、Gin、GORM 或 PM 业务 RBAC。 |
| `internal/module/requirement/cover` + `internal/worker/requirementcover` | 封面队列写入与后台领取 | 已完成物理迁移。 |
| `internal/app/{api,worker,coverrenderer,coverworker}` | 各进程角色的装配 | 已完成；它们共享同一代码、镜像、配置和发布单元。 |
| `internal/taskkey/` | `internal/module/task/gorm/key_sequence.go` | 已完成，旧包不得恢复。 |
| `internal/legacyimport/`、`internal/teamscoperepair/` | `internal/maintenance/...` | 已完成，在线代码不得导入。 |
| 根目录 `scripts/start-local-*`、`run-local-*` | `backend/scripts/` | 已完成，脚本只依赖后端自身配置。 |
| `deploy/docker-compose.backend.yml`、`docker-compose.shared-dev-db.yml` | `backend/deploy/` | 已完成，与后端镜像及 `.env.example` 一起维护。 |
| `backend/docs/migrations/` | 历史说明、来源清单、兼容性决策与迁移报告 | 不得放可执行 SQL；后续 schema 只新增 `backend/migrations/` 下的 golang-migrate 文件。 |

`internal/transport/http` 仍是存量 HTTP adapter 汇集点；所有 record 已直接归属各领域的 `gorm` 包，
全局菜单注册与当前用户菜单查询已归属 `module/workbench`；MCP/CLI 工具契约归属
`module/toolruntime`，不再保留跨领域模型门面或根级 navigation 包。
后续拆领域时必须把某一领域的 handler、用例和持久化访问一起归位，不能复制后长期双写或保留两套规则。

## 6. 渐进迁移和检查

后续拆分按“修改哪个领域，就完整归位哪个领域”执行。优先将 `transport/http` 内的领域 handler、DTO、
用例和 GORM 查询一起移入所属模块。每次迁移必须保持既有 HTTP 和数据库兼容。

新增或迁移代码时至少满足：

- `cmd/` 仅依赖 `internal/app/...`；
- 新业务规则、事务与权限判断进入 `module/<domain>`；
- 新 HTTP 代码进入 `module/<domain>/http`，新 GORM 查询进入 `module/<domain>/gorm`；
- 新第三方调用进入 `integration/<system>`；
- 新后台执行进入 `worker/<kind>`，并在 `app` 层明确其生命周期；
- 维护代码只进入 `maintenance/`；
- 测试与职责同目录：模块、HTTP、GORM、worker、maintenance 分别放置。

[`scripts/check-boundaries.sh`](scripts/check-boundaries.sh) 检查 `cmd` 的依赖、maintenance 的在线隔离，
以及本次已经移除的根级 internal 包不被重新引入，并禁止恢复 `internal/compat/model`、`internal/navigation`
等全局门面或根级业务目录。
