---
name: code-quality-review
description: "双轴代码质量审查:对统计周期内一名成员的逐次提交,按规范轴(Fowler 坏味道基线)与意图轴(提交主题 vs 实际改动)逐条审查,按 critical/warning/info 分级,输出逐提交结论与综合质量评估。方法论融合自 mattpocock/skills 与 coderabbitai/skills 的 code-review 技能。"
---

# 代码质量审查(code-quality-review)

对报告周期内一名成员的逐次提交做双轴审查,产出逐提交结论与综合评估,供汇报叙述与导出引用。

## 输入

`input.subject.commits`:逐次提交证据数组,每项含 sha、day、subject、additions、deletions、files、
debug_hits(调试残留命中)、todo_hits(TODO/FIXME 命中)、diff_sample(新增行内容截断样本,可能缺失)。

## 硬性要求

- **逐条全覆盖**:对每一条提交输出独立结论,一条不许漏、禁止合并点评、禁止只挑代表性的;sha 原样回填。
- **证据绑定**:每条 issue 必须引用 diff_sample 中的具体代码片段或文件路径;没有证据的问题禁止编造。
- **样本缺失降级**:diff_sample 缺失的提交按 subject 与行数给有限判断,并在 issues 标注「无代码样本,按提交元信息判断」。
- **体量约束**:每条提交的 issues 最多 3 条、每条 ≤40 字;verdict/summary 精炼不展开。

## 轴一:规范(Standards)

对照 Fowler《重构》第 3 章坏味道基线逐条比对 diff_sample,每条坏味道是「是什么→怎么改」的判断题,
不是硬违规;跳过工具链已强制的内容:

- **神秘命名**(Mysterious Name):命名不达意 → 改名;起不出诚实的名字说明设计混沌。
- **重复代码**(Duplicated Code):同形逻辑在多个 hunk/文件出现 → 抽公共函数。
- **依恋情结**(Feature Envy):函数大量操作他者数据 → 移动到数据所属处。
- **数据泥团**(Data Clumps):几个字段/参数总是结伴出现 → 收拢成一个类型。
- **基本类型偏执**(Primitive Obsession):原始类型顶替领域概念 → 建领域小类型。
- **重复开关**(Repeated Switches):同一类型的相同分支级联反复出现 → 映射表或多态。
- **霰弹式修改**(Shotgun Surgery):一个逻辑变更散落多文件 → 聚拢到一个模块。
- **发散式变化**(Divergent Change):同一文件因多个不相关原因被改 → 按变化原因拆分。
- **夸夸其谈通用性**(Speculative Generality):为不存在的需求加抽象/参数 → 删掉内联。
- **消息链**(Message Chains):a.b().c().d() 长链导航 → 在首对象上封装一个方法。
- **中间人**(Middle Man):几乎纯转发的层 → 砍掉,直连真实目标。
- **被拒绝的遗赠**(Refused Bequest):继承者拒绝大部分继承 → 改组合。

## 轴二:意图(Spec)

提交 subject 声称要做的事与 diff_sample 实际改动是否一致:

- 声称修复 X 却没有对应改动 → **疑似跑偏**;
- 夹带与主题无关的改动 → **范围蔓延**;
- 实现看起来对但逻辑可疑 → **疑似错误实现**。
- 每条发现必须引用 diff_sample 中的代码片段作为证据。

## 严重度分级与结论映射

- **critical**:安全漏洞 / 数据丢失 / 崩溃风险;
- **warning**:正确性风险 / 性能问题 / 明显坏味道;
- **info**:风格与改进建议。

verdict 映射:含 critical → **差**;warning ≥2 或有重大 warning → **中**;仅 info → **良**;
干净且结构/测试良好 → **优**。debug_hits/todo_hits > 0 必须在 issues 点名;
单次新增 >800 行的巨型变更必须在 issues 提示拆分。

## 输出契约

只输出一个 JSON 对象,不要 Markdown 代码块:

```json
{
  "summary": "一句话总评",
  "changes": [],
  "commitReviews": [
    { "sha": "原样回填", "verdict": "优|良|中|差", "summary": "一句话点评", "issues": ["带证据的问题,每条≤40字"] }
  ],
  "qualityAssessment": {
    "score": 0,
    "conclusion": "一段话综合结论",
    "commonIssues": ["共性问题"],
    "risks": ["风险"],
    "suggestions": ["可执行的改进建议"]
  }
}
```

score 为 0-100 整数,与逐条 verdict 分布一致(多数优/良应高分,出现中/差应显著扣分)。
