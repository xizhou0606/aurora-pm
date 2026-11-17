-- 回退：默认值切回 normal；P 档数据无法还原历史低/普通/高/紧急，统一归 normal。
ALTER TABLE requirements ALTER COLUMN priority SET DEFAULT 'normal';
UPDATE requirements SET priority = 'normal' WHERE priority IN ('p0', 'p1', 'p2', 'p3');
