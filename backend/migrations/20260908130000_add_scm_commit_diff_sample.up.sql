-- 逐提交 AI 质量分析的数据底座：
-- diff_sample 保留新增行内容（截断），供报告把每次提交交给 AI 点评；
-- debug_hits/todo_hits 是采集时的廉价命中计数，构成可审计的质量扣分。
ALTER TABLE scm_commits
    ADD COLUMN IF NOT EXISTS diff_sample TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS debug_hits INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS todo_hits INT NOT NULL DEFAULT 0;
