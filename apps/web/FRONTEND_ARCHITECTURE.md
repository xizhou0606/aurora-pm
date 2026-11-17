# Frontend Architecture Guide

> 本文档描述了 `apps/web` 的 Feature-Sliced Design (FSD) 架构规范。
> 所有前端开发（包括 AI 生成的代码）须遵循此约束。

## 1. 目录结构

### 顶层

```
apps/web/src/
├── app/            # 应用壳：路由、状态、引导、Provider
├── features/       # 业务功能模块（FSD Slice）
│   ├── ai-pm/
│   ├── project-management/
│   ├── work-items/
│   ├── quality-management/
│   ├── auth/
│   ├── account/
│   ├── workspace/
│   ├── access-control/
│   ├── access-management/
│   ├── bi-screen/
│   ├── inbox/
│   ├── landing/
│   ├── mcp-cli/
│   └── host-navigation/
├── shared/         # 跨功能共享层
│   ├── api/
│   ├── config/
│   ├── copy/
│   ├── hooks/
│   ├── lib/
│   ├── model/
│   ├── navigation/
│   ├── runtime/
│   └── ui/
└── generated/      # 自动生成代码（勿手动编辑）
```

### 路径别名

```json
"@/*": ["./src/*"]
```

所有内部 import 使用 `@/` 前缀，禁止相对路径穿越：

```tsx
// ✅ 正确
import { Button } from '@/shared/ui'
import { ProjectWorkspace } from '@/features/project-management/components'

// ❌ 禁止
import { Button } from '../../shared/ui'
import { ProjectWorkspace } from '../../../features/project-management/components'
```

## 2. Feature 内部规范（FSD Layer）

每个 feature 目录**必须**包含以下子目录：

```
features/<name>/
├── api/          # API 请求封装（useQuery / useMutation 定义）
├── components/   # UI 组件
├── hooks/        # 业务逻辑 Hook
├── model/        # 类型、常量、Selector、状态机
├── routes/       # 路由页面（可被 app/router 动态导入）
├── utils/        # 纯工具函数
└── index.ts      # Public API
```

### 2.1 层级依赖规则

| 层 | 可以引用 |
|---|---|
| `api/` | `shared/` 中的 API 基础设施 |
| `components/` | `api/`、`hooks/`、`model/`、`utils/`、`shared/ui/` |
| `hooks/` | `api/`、`model/`、`utils/` |
| `model/` | 纯类型/常量，不允许引用组件或 Hook |
| `routes/` | 同 feature 或其他 feature 的 `routes/`、`components/` |
| `utils/` | 纯函数，不允许引用组件或 Hook |

### 2.2 Public API（index.ts）

每个 feature 的 `index.ts` 只暴露需要被其他 feature 使用的部分：

```ts
// ✅ 正确
export { ProjectDetailPage, ProjectItemDetailPage } from './routes'
export { projectListQueryOptions } from './model'

// ❌ 禁止：暴露内部实现细节
export { InternalHelperComponent } from './components/InternalHelperComponent'
export { SomeInternalUtility } from './utils/internal-utility'
```

测试文件（`.react-test.tsx` / `.test.ts`）**不**在 `index.ts` 中导出。

## 3. 文件大小限制

| 类型 | 最大行数 | 说明 |
|---|---|---|
| 生产组件 `.tsx` | **300 行** | 超过应拆分子组件 |
| 生产逻辑 `.ts` | **300 行** | 超过应考虑提取 `utils/` 或拆分模块 |
| Hook `.ts` / `.tsx` | **200 行** | 业务逻辑 Hook |
| API 封装 `.ts` | **150 行** | 单一 API 文件 |
| 测试文件 | **不限**（推荐 ≤500 行） | 但大型测试应拆分 concern |
| Model/类型 `.ts` | **300 行** | |
| `routes/` 下的路由页面 | **300 行** | 复杂页面应拆分子组件 |

> AI 生成代码时，必须遵守上述行数约束。如果一个文件超过限制，自动提出拆分方案。

## 4. 命名规范

### 4.1 文件和目录

| 类型 | 命名 | 示例 |
|---|---|---|
| React 组件文件 | `PascalCase.tsx` | `WorkflowInstanceControl.tsx` |
| Hook 文件 | `camelCase.ts` 或 `use*.ts` | `useWorkflowInstance.ts` |
| 纯逻辑/工具 | `camelCase.ts` | `workflow-runtime-track.ts` |
| 类型定义 | `camelCase.ts` | `types-workflow.ts` |
| API 请求 | `camelCase.ts` | `workflow-api.ts` |
| 测试文件 | `*.react-test.tsx`（组件测试） | `WorkflowInstanceControl.react-test.tsx` |
| 测试文件 | `*.test.ts`（逻辑测试） | `workflow-runtime-track.test.ts` |

### 4.2 导出具名（Named Export）

**禁止默认导出**（default export）：

```tsx
// ✅ 正确
export function WorkflowInstanceControl() { ... }

// ❌ 禁止
export default function WorkflowInstanceControl() { ... }
```

### 4.3 CSS

- 使用 Tailwind CSS utility classes，禁止 feature 级独立 CSS 文件
- 仅在极少数第三方复杂场景（如 DayPilot 日历）允许 `.css` 文件，须在白名单中登记
- 样式文件命名：`kebab-case.css`

## 5. 组件规范

### 5.1 组件职责

- **展示组件**：放在 `components/`，纯渲染，通过 props 接收数据和回调
- **页面组件**：放在 `routes/`，负责组装 + 获取路由参数
- **Hook**：封装业务逻辑和状态，不返回 JSX（除非是 Render Props 变体）

### 5.2 Hooks 规则

```ts
// ✅ 正确：API 查询 Hook 返回 query 对象
export function useWorkflowInstances(workspaceId: number, subjectType: string, subjectId: number) {
  return useQuery(workflowInstanceListQueryOptions(workspaceId, subjectType, subjectId))
}

// ❌ 禁止：在组件外部 / 条件中调用 Hook
if (condition) {
  useQuery(...) // 违反 React Rules of Hooks
}
```

## 6. API 层规范

### 6.1 查询封装

- API 函数定义在 `api/` 目录下
- 使用 `@tanstack/react-query` 的 `queryOptions` 模式
- API 请求在 `useQuery` / `useMutation` 中调用，不在组件中直接调用 `fetch`

```ts
// api/workflow-api.ts
export const workflowInstanceQueryOptions = (id: number) => queryOptions({
  queryKey: ['workflow', 'instance', id],
  queryFn: () => requestJson<WorkflowInstance>(`/pm/workflows/instances/${id}`),
})
```

### 6.2 禁止直接操作 Query Cache

禁止在业务组件中直接调用 `invalidateQueries`，`setQueryData` 等 API。查看 `native-feature-architecture.react-test.ts` 中的完整禁止列表。

## 7. 导入规范

### 7.1 导入顺序

1. 外部库（React, react-router, tanstack-query 等）
2. `@/shared/` 共享层
3. `@/features/` 业务模块
4. 同 feature 内的相对路径
5. CSS/样式

### 7.2 禁止的导入

- 禁止跨 feature 的深层导入（`../another-feature/components/...`）
- 禁止从 `packages/shared` / `packages/ui` 导入（已废弃并删除）
- 禁止使用废弃的 `@pm/shared` / `@pm/ui` 别名

## 8. 架构测试

`native-feature-architecture.react-test.ts` 是自动化的架构约束检查，包括：

- 禁止污染 DOM 的原生 HTML 元素（`<input>`、`<select>` 等应用在 feature 组件中）
- 禁止在 feature 层直接使用 `useQuery` / `useMutation`（应转到 `api/` 模块）
- 禁止跨 feature 的深层导入
- 禁止在 query 加载状态上使用布尔 props（loading、error 等）
- CSS 白名单检查

**新增 feature 时必须把它加到 `FEATURE_NAMES` 列表中。**

## 9. 测试规范

### 9.1 文件位置

测试文件与被测文件同目录：

```
components/
├── WorkflowInstanceControl.tsx
├── WorkflowInstanceControl.react-test.tsx   ← 测试文件
├── utils.ts
└── utils.test.ts                            ← 纯逻辑测试
```

### 9.2 命名约定

| 测试类型 | 后缀 | 示例 |
|---|---|---|
| 组件测试 | `.react-test.tsx` | `Button.react-test.tsx` |
| 纯逻辑测试 | `.test.ts` | `format-date.test.ts` |
| Hook 测试 | `.react-test.tsx` | `useWorkflowInstance.react-test.tsx` |

### 9.3 Mock 策略

- Mock API 层使用 `vitest` 模拟，不发起真实 HTTP 请求
- 跨组件 Mock 使用 `vi.mock()` 或虚拟组件（stub）
- 优先使用 `queryOptions` 模式的 query key 匹配，避免 mock 整个请求模块

## 10. 构建约束

| 检查 | 命令 | 必须通过 |
|---|---|---|
| TypeScript 类型 | `npx tsc --noEmit` | ✅ 0 errors |
| 构建 | `npx rsbuild build` | ✅ SUCCESS |
| 测试 | `bun test src` | ✅ 无新增失败 |
| Lint | `npx oxlint src/` | ✅ |

## 11. 技术栈

| 领域 | 技术 |
|---|---|
| 构建工具 | Rsbuild v2 |
| 框架 | React 19 |
| 路由 | @tanstack/react-router |
| 数据获取 | @tanstack/react-query v5 |
| 样式 | Tailwind CSS |
| 测试 | bun test + @testing-library/react |
| 富文本 | Tiptap 3.29.2（版本锁定） |
| 包管理 | bun（bun.lock 锁定，workspace 见根 package.json） |

> ⚠️ `@tiptap/*` 版本已锁定在 `3.29.2`，见根 `package.json` 的 `overrides` 配置（bun install 消费）。升级须验证 `getPreviousBlockSibling` 等所有 API 兼容性。

## 12. Design Tokens & 样式约束

> 所有 UI 代码必须遵循以下设计 token 约束，禁止硬编码值。

### 12.1 字体大小

| 场景 | 必须使用 | 禁止 |
|------|---------|------|
| 正文/导航项 | `text-sm` (14px) | `text-[13px]` |
| 辅助说明/标签/时间戳 | `text-xs` (12px) | `text-[10px]`、`text-[11px]` |
| 标题 H3 | `text-base` (16px) | 硬编码 |
| 标题 H2 | `text-[22px]` | — |
| 标题 H1 | `text-[30px]` | — |

```tsx
// ✅ 正确
<span className="text-xs text-muted-foreground">辅助说明</span>
<dt className="text-xs text-muted-foreground">标签</dt>

// ❌ 禁止
<span className="text-[11px] text-muted-foreground">辅助说明</span>
```

### 12.2 颜色

- **禁止硬编码 hex 颜色**：`bg-[#e2483d]`、`text-[#168a49]` 等一律禁止
- 使用 Tailwind v4 语义 token：`bg-red-500`、`text-emerald-600`、`bg-violet-600`
- 功能色映射：success→`emerald`、warning→`amber`、danger→`red`、info→`blue`、accent→`violet`

```tsx
// ✅ 正确
<div className="bg-red-500 text-white">危险</div>
<div className="border-emerald-200 bg-emerald-50 text-emerald-700">成功</div>

// ❌ 禁止
<div className="bg-[#e2483d]">危险</div>
```

### 12.3 间距

- **禁止任意间距值**：`p-[14px_14px_10px]`、`-top-[3px]`、`gap-[9px]` 一律禁止
- 使用 Tailwind 间距 scale：`p-4`、`-top-0.5`、`gap-2`

```tsx
// ✅ 正确
<div className="p-4 rounded-lg">卡片</div>
<span className="relative -top-0.5">徽标</span>

// ❌ 禁止
<div className="p-[14px_14px_10px]">卡片</div>
```

### 12.4 共享组件

| 场景 | 必须使用 | 禁止 |
|------|---------|------|
| 错误/警告/成功/信息提示 | `AlertBanner` | 内联 `bg-red-50`/`bg-amber-50` div |
| 图标徽章 | `IconBadge` | 内联 `bg-{color}-600 grid size-8` |
| 状态标签 | `StatusPill` | 自定义 pill 组件 |
| 页面标题 | `OperationalPage` / `PageHeader` | 裸 `<h1>` |
| 空状态 | `EmptyState` | 内联空状态 |
| 加载中 | `LoadingSkeleton` | 内联骨架屏 |

```tsx
// ✅ 正确
<AlertBanner tone="error">加载失败</AlertBanner>
<IconBadge icon={Sparkles} tone="violet" />
<StatusPill tone="danger">已拒绝</StatusPill>

// ❌ 禁止
<div className="bg-red-50 border-red-200 rounded-md p-3">加载失败</div>
<span className="grid size-8 bg-violet-600"><Sparkles /></span>
```

### 12.5 动效

- 使用设计 token：`--design-motion-fast: 150ms`、`--design-motion-standard: 220ms`
- 全局 `prefers-reduced-motion` 媒体查询已覆盖，无需额外处理
- 过渡/动画使用 Tailwind `duration-150` / `transition-all` 等标准类

### 12.6 暗色模式

- 所有组件必须支持 `dark:` 变体
- 背景使用 `bg-card`/`bg-muted`/`bg-background` 等语义 token
- 文本使用 `text-foreground`/`text-muted-foreground` 等语义 token
- 边框使用 `border-border` 语义 token

### 12.7 样式质感建议（Awesome Design 审计结果）

以下为对比线性/其他产品后的可选改进点（不强制）：

| 领域 | 建议 | 优先级 |
|------|------|--------|
| **微交互** | Button 点击 `scale-[0.97]` 反馈；Card hover `shadow-sm`；StatusPill `hover:ring-1` | P1 |
| **阴影系统** | 新增 `--design-shadow-level-1/2/hover` 三级阴影，目前仅一种 | P1 |
| **骨架脉冲** | Skeleton 增加 `animate-pulse` 或自定义 shimmer 动画 | P1 |
| **加载过渡** | OperationalPage scroll 区域添加 `fade-in` 微过渡 | P2 |
| **表单 focus** | Input/Textarea/Select 增加 `ring-offset-1` 更明显 focus 指示 | P2 |
| **空状态** | EmptyState 增加可选 `icon` 装饰 | P3 |
| **页面切换** | 路由页面切换可增加淡入动画 | P3 |

### 12.8 设计系统 Lint（@shadcn/lint）

本节约束由 [shadcn/lint](https://github.com/shadcn-ui/lint) 机械化执行，配置在
`apps/web/.oxlintrc.json`（插件经 `apps/web/shadcn-lint-plugin.mjs` 薄壳加载），
命令：`bun run lint`（即 `oxlint src`）。六条规则均为
**error 级**，`bun run lint` 必须零输出。

- **no-arbitrary-values**：禁止有 scale 等价物却硬写的字号与间距（§12.1/§12.3）。
  结构性任意值（网格模板、`calc/min/max/clamp/var/color-mix`、`--design-shadow-*` 引用、
  transition/animation 列表、视口尺寸等）按配置内清单放行；钦定例外：
  `text-[22px]`/`text-[30px]`（H2/H1）、`text-[9px]`（密集工作流轨道微标签）。
- **no-raw-colors**：禁止硬编码 hex 与未声明主题 token；Tailwind 调色板即本仓库功能色
  （§12.2），按色族放行。
- **no-unknown-classes**：禁止拼错或失效的类名；项目自定义 CSS 类按前缀放行
  （`landing-*`/`pm-*`/`is-*`/`work-items-*`）。
- **require-static-classes**：给 `@/shared/ui` 组件传类名必须可静态分析——
  `cn('静态串', cond && '静态串')` 或双静态分支三元；禁止模板字符串、字符串拼接、
  `[].join(' ')`。
- **no-inline-styles**：静态值一律用 Tailwind 类；运行时几何值（虚拟化/拖拽/测量）
  用 CSS 自定义属性模式：`style={{ '--x': v }}` + `className="w-(--x)"`。
  `features/**` 由架构门禁测试（`native-feature-architecture.react-test.ts`）双重把关：
  style 对象只允许 `--` 开头的键。
- **no-restyle**：本仓库组件为组合式 API，className 组合（语义 token、焦点环、形状、
  动效）是既定用法，全部放行。唯一例外 **Button**：功能调色板色必须走 variant——
  `default`/`outline`/`secondary`/`ghost`/`destructive`/`link` 之外，状态类变体为
  `warning`（实心琥珀）、`danger-soft`/`warning-soft`/`info-soft`（软色描边，追加在
  outline 基底之上）。数据驱动的状态色（风险等级边框、决策卡选中态、交付节点状态）
  用注明原因的行内豁免。
- 主题 token 映射以 `src/shared/styles.css` 的 `@theme inline` 为准（`components.json` 已指向
  该文件）；新增语义色须先声明 `--color-*` 再使用。
- 按文件豁免：`login-provider-icons` / `brand-mark` / `ai-loading` 保留原始品牌色
  （第三方 logo 与品牌插画）；`*.react-test.tsx` 允许非 Tailwind 夹具类。
- 行内样式类型增强：`apps/web/src/react-css-properties.d.ts` 为 `React.CSSProperties`
  开启 `--` 自定义属性键。

## 13. 架构变更流程

1. 从 `dev` 拉分支，命名遵循 `feature/<desc>` 或 `fix/<desc>`
2. 按模块分步提交代码（不混合同一 commit 中的无关变更）
3. 提交前运行完整检查：`tsc --noEmit` + `rsbuild build` + vitest
4. 确保无新增测试失败（允许已有的 dev 基线失败）
5. 更新本文件（如果架构约束变更）
6. 提 PR 到 `dev`