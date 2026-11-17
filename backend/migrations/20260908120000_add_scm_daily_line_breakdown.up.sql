-- 代码活动构成拆分（A）与评审覆盖信号（B）：
-- 行数按工作类型（源码/测试/文档/配置）落列，合入 PR 记录"带评审合入"数。
-- 历史行保持 0（仅总数），新窗口同步后按日重写补齐。
ALTER TABLE scm_daily_contributions
    ADD COLUMN IF NOT EXISTS source_additions INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS test_additions INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS doc_additions INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS config_additions INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS prs_reviewed INT NOT NULL DEFAULT 0;
