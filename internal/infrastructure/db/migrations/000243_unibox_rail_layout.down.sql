DELETE FROM public.user_view_preferences WHERE view = 'unibox_rail';
ALTER TABLE public.user_view_preferences DROP CONSTRAINT IF EXISTS user_view_preferences_layout_check;
ALTER TABLE public.user_view_preferences DROP COLUMN IF EXISTS layout;
