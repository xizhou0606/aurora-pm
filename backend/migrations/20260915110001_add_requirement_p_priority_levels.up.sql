-- 需求优先级切换 P0–P3 档位（产品确认口径）：历史数据统一回填 P1。
-- 字段沿用 requirements.priority，仅换枚举值与默认值；应用层校验同步收紧。
UPDATE requirements SET priority = 'p1' WHERE priority IN ('low', 'normal', 'high', 'urgent');
ALTER TABLE requirements ALTER COLUMN priority SET DEFAULT 'p1';
