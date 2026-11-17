# PM 权限目录

跨 Tool、Action、HTTP 接口与前端模块的强制基数、变更顺序、兼容性和验收规则见
[`../../.codex/specs/tool-action-frontend-governance.md`](../../.codex/specs/tool-action-frontend-governance.md)。本目录是该规范的唯一机器可读事实源。

`catalog.json` 是 PM 权限 code 的唯一事实源。授权展示从前端交互反推，按
`页面（page）→ 前端视图（view，可递归）→ 用户动作（action）` 建模；无 UI 的接口或 Tool 动作放入
明确的 capability 视图。只有 action code 能授权后端操作；页面和视图 code 只用于权限树展示和批量选择，
不能直接授权业务行为。`pages[].modules` 暂作 action 存储分类与旧页面兼容结构，不得用它反向生成已纳入治理的前端权限树。

## 强制门禁

在仓库根目录运行：

```sh
python3 scripts/validate-access-catalog.py
```

每次修改权限目录后必须重新生成 Go、TypeScript 和业务动作映射文档：

```sh
python3 scripts/generate-access-catalog.py
python3 scripts/generate-access-catalog.py --check
```

门禁会比对权限目录、React router、shell catalog，以及所有已登记的生产 Gin 路由注册函数。新增受保护
route 必须唯一映射到一个 action；公开路由或仅需登录态的系统路由必须有经过评审的豁免原因。新增路由注册
函数必须同步加入 `route_inventory.sources`，否则未跟踪路由扫描会直接失败。

## 系统固定基础能力

登录会话、自助账号、工作区选择、最小用户/团队目录和范围选择器属于固定基础能力，不进入角色或用户组的
可编辑 Action 集合。每条基础能力仍必须声明 `capability`、`access_stage`、前端调用入口和最小返回投影；
管理接口继续由对应业务 Action 保护，不能因存在基础目录接口而放宽。

基础能力若有语义相关的 MCP/CLI Tool，使用 `related_tools` 记录审计关系。该字段不是授权继承：Tool 仍由
独立的 audience-bound token、精确 Tool + scope Grant、ChangeSet 和 UAT 规则授权；不得把基础能力或页面
Action 直接换算为外部 Tool 执行权限。

全部协议无关 Tool 必须且只能属于一种治理分类：通过 `tool_bindings` 关联一个业务 Action、通过
`related_tools` 关联一个固定基础能力，或通过 `tool_no_action` 登记经过评审的无 Action 原因。门禁会从
Tool catalog 反向校验全集，未分类和重复分类都会失败。

## 新增业务动作闭环

新增按钮、行操作、批量操作、mutation、后台操作或 Tool 时，必须在同一变更内完成：

1. 从页面、可递归前端视图和按钮/交互反推 action，登记稳定 code，并绑定唯一后端 route 或权威的 handler 内策略。
2. 纳入 `operation_governance`，声明前端 owner、组件入口、capability owner，以及
   `页面 > 区域 > 按钮/交互` 格式的实际入口位置；一个 action 有多个入口时必须全部列出。
3. 显式登记 Tool 名称，或给出 `not_applicable` 原因。
4. 重新生成并提交 [业务动作映射表](../../docs/access/business-action-matrix.md)。
5. 让组件显隐/禁用与 mutation 提交复用同一 capability，后端仍独立 fail closed。

`api_only` 动作的按钮位置必须为空，生成的映射表会显示“无按钮（仅接口）”；`ui` 动作至少要有一个
按钮/交互位置。缺失、状态冲突或引用不存在的组件文件都会导致门禁失败。
单一生产入口使用 `component`；同一 action 被多个生产组件消费时必须改用 `components` 数组，逐项登记
真正承载按钮、菜单或交互的组件。生成器会把这些组件全部写入映射表，校验器会逐个检查文件存在。

没有独立后端 route 的 action 必须填写明确的 `frontend_only_reason`。在获得专用 route 或权威 handler 内
策略前，该 action 不可独立授权。运行时常量、数据库种子和前端类型必须从本目录生成；业务代码不得重复
硬编码权限字符串。

## 生成物

生成的 Go 包提供保留存储分类的 `Tree`、面向授权界面的递归 `PermissionTree`、`ActionCodes`、`ActionForRoute`、`BootstrapActionRoutes`、
`OperationBindings`、`ActionForTool` 和 `ToolNamesForAction`。每条 route bootstrap 记录包含稳定唯一的 `InterfaceCode`：取
`METHOD + " " + canonical route` 的 SHA-256 前 24 位并加上 `route.` 前缀，确保不超过 service interface
的 96 字符限制，同时保留 action 的按钮级业务含义。查询只接受 canonical route template；`/core` 和
`/platform` 兼容别名不会生成独立权限 key。

生成的 TypeScript 提供 `PM_ACTION`、`PMActionCode`、`PM_ACTION_CODES`、`PMPageCode`、
`PM_PAGE_BY_ROUTE`、`PM_OPERATION_ACTIONS_BY_DOMAIN` 和 `PM_TOOL_NAMES_BY_ACTION`。显示名称和权限树正文
仍由后端 canonical response 提供，不在前端复制第二套文案或树结构。
