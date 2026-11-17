# 版本化 SQL Migration 规范

历史 `000001` 切换标记之后，PostgreSQL 的 schema 变更统一使用 UTC 时间戳作为
migration 版本号：

```text
YYYYMMDDHHMMSS_short_description.up.sql
YYYYMMDDHHMMSS_short_description.down.sql
```

规则：

1. 时间戳必须取创建文件时的 UTC 当前时间，禁止预留或猜测递增序号。
2. 每个版本必须同时存在描述一致的 `up` 与 `down` 文件，且各自只能有一个。
3. migration 一旦进入共享环境，其版本号、文件名和 SQL 内容均不可修改。
4. 如果其他分支已经使用同一秒的时间戳，后合并的分支必须重新生成时间戳；禁止让不同 SQL
   复用同一个版本号。
5. migration 必须与 `golang-migrate` 兼容；完成切换后禁止通过 GORM `AutoMigrate`
   推进 PostgreSQL schema。
6. 日常 API 进程使用 `validate` 模式；只有指定的迁移进程可以使用 `apply`。
7. 数据转换必须提供可审查的回滚策略；存在精度损失或不可逆行为时，必须在 `down` SQL
   或迁移记录中明确说明。

数据库 migration 测试会强制检查时间戳格式、版本唯一性及 up/down 配对关系，使冲突在合并前失败。
