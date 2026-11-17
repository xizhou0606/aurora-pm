-- Cutover marker for golang-migrate.
-- Production and refreshed databases already match GORM schema 0055_external_tool_oauth.
-- This version records that fact. Later schema changes MUST be new numbered SQL files.

SELECT 1;
