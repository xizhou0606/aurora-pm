ALTER TABLE ai_artifacts ALTER COLUMN title TYPE varchar(220) USING left(title, 220);
