CREATE TABLE public.article_likes (
    id          uuid DEFAULT public.uuid_generate_v7() PRIMARY KEY,
    article_id  uuid NOT NULL REFERENCES public.articles(id) ON DELETE CASCADE,
    user_id     uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    created_at  timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT article_likes_article_id_user_id_key UNIQUE (article_id, user_id)
);

CREATE INDEX idx_article_likes_article_id ON public.article_likes USING btree (article_id);
CREATE INDEX idx_article_likes_user_id ON public.article_likes USING btree (user_id);
CREATE INDEX idx_article_likes_created_at ON public.article_likes USING btree (created_at DESC);
