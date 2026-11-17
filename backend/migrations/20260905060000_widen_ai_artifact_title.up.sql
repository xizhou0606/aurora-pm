-- 汇报类 AI 产出的 summary 是完整 Markdown 正文，作为 artifact 标题远超
-- varchar(220)；放宽为 text，保留全部内容。
ALTER TABLE ai_artifacts ALTER COLUMN title TYPE text;
