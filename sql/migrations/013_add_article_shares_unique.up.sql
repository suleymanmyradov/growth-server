-- Add unique constraint to prevent duplicate shares
-- This ensures a user can only share an article once per platform
-- If you need to track multiple shares, consider incrementing a share_count instead

ALTER TABLE article_shares 
    ADD CONSTRAINT unique_article_share_per_platform 
    UNIQUE (article_id, user_id, platform);
