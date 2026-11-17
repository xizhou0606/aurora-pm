# AI Runtime Skills 职责登记

本目录是 ai-runtime 运行时技能的唯一存放点，本 README 是技能职责边界的唯一登记处。
**本项目 ai-runtime 只有且只需要三个一级技能**，运行时只加载以下三个技能，各自职责互不重叠：

| 技能 | 目录 | 能力 | 职责边界 |
| --- | --- | --- | --- |
| `qa-test-case-design` | `skills/qa-test-case-design/` | `qa.test_cases` | QA 用例设计：结构化测试用例、覆盖度补丁、飞书导出模板。只负责「怎么测」，不做代码评审。 |
| `code-quality-review` | `skills/code-quality-review/` | `code.commit_quality_review` | 逐提交代码质量审查（双轴：规范轴 + 意图轴）：对成员统计周期内逐次提交独立出结论与综合评估，供汇报叙述与导出引用。不做用例设计，不做整体绩效评价。 |
| `changelog-draft` | `skills/changelog-draft/` | `changelog.draft` | 版本公告草稿：把发布窗口内的 conventional commits（含截断 diff 样本）归并转写为面向用户的 CHANGELOG.md 区块草稿。服务两条链路：构建期自动生成（`release-notes:auto`，部署 build/deploy 钩子，无人值守，失败降级为确定性分组）与人工 CLI（`bun changelog:draft`）。只面向本仓产品公告，不做 PM 业务数据分析，不进业务 API。 |

## 方法论来源

- 每个技能的来源仓库、路径与 commit 记录在各自同名的 `.lock.json`
  （`qa-test-case-design.lock.json` / `code-quality-review.lock.json` / `changelog-draft.lock.json`），
  lock 同时记录文件内容哈希；运行时（`src/qa-test-case-skill.ts`、`src/code-quality-review.ts`、
  `src/changelog-draft.ts`）加载时校验哈希，保证线上跑的方法论可回溯到具体版本。
- `code-quality-review` 的方法论融合自 `mattpocock/skills` 与 `coderabbitai/skills` 的
  code-review。
- `changelog-draft` 为本仓自研方法论（依据根目录 CHANGELOG.md 的维护约定），lock
  的 `source.repositories` 指向本仓，不参与 `sync:skills` 上游同步。
- 刷新/同步技能用 `(cd apps/ai-runtime && bun run sync:skills)`（可用 `PM_SKILLS=` 选子集、
  `PM_SKILL_COMMIT_<NAME>` 钉住 commit），运行时只读本目录，不访问 GitHub。
  同步清单只覆盖上游技能；`changelog-draft` 修改 SKILL.md 后必须同步更新其
  lock 中的文件哈希与 `packageSha256`。

## 变更门禁

- **新增技能必须先在本 README 登记职责（目录、能力、边界），禁止与既有技能职责重叠**；
  未登记的目录不属于运行时技能契约，运行时代码不得引用。
- 删除或重命名技能时，必须同步更新本 README、对应 `.lock.json` 与 `src/` 内的加载代码。

本发行版不分发第三方 QA 技能文件。获得上游授权后可按 qa-test-case-design.lock.json 的 commit 准备本地技能。其他第三方技能许可见 ../third-party-licenses/。
