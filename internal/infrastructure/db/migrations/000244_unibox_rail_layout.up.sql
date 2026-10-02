-- A view's layout document, for views that save one instead of columns and a
-- sort: the unibox scope rail's favorites, order and hidden rows. Read back
-- whole and validated by the app on write, never filtered in SQL.
ALTER TABLE public.user_view_preferences
    ADD COLUMN IF NOT EXISTS layout jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE public.user_view_preferences
    DROP CONSTRAINT IF EXISTS user_view_preferences_layout_check;
ALTER TABLE public.user_view_preferences
    ADD CONSTRAINT user_view_preferences_layout_check CHECK (jsonb_typeof(layout) = 'object');
