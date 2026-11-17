-- Run pm-cleanup-user-identities --retire-external-id --apply first.
-- All writers must remain stopped between preparation and deployment.
-- The production migrator splits at semicolons, so do not use a DO block.
-- Reading the optional key as JSON also supports fresh/already-retired schemas.
-- A nonnumeric diagnostic deliberately fails the cast before DROP can run.
SELECT CAST(CASE WHEN EXISTS (
    SELECT 1 FROM public.users AS u
    WHERE COALESCE((to_jsonb(u)->>'external_user_id')::bigint, 0) <> 0
) THEN 'external_user_id requires archived offline preparation before upgrade'
ELSE '1' END AS integer);

ALTER TABLE public.users DROP COLUMN IF EXISTS external_user_id;
